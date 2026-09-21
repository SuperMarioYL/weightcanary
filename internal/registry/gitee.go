package registry

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SuperMarioYL/weightcanary/internal/state"
)

// lfsPointerMaxSize bounds which tree entries get a blob fetch to test for a
// Git LFS pointer. Real pointers are ~134 bytes, so anything above 200 bytes
// is an ordinary small file and never fetched.
const lfsPointerMaxSize = 200

// giteeRequestSpacing is the minimum spacing between Gitee API calls. The
// anonymous API rate-limits aggressively (a burst of pointer decodes for a
// sharded model answers HTTP 403 mid-run), so the poller spaces its requests
// out; set GITEE_TOKEN for authenticated, higher-limit runs.
const (
	giteeRequestSpacing = 150 * time.Millisecond
	giteeMaxSpacing     = 3 * time.Second
)

// Gitee snapshots the mirror repos under the "Hugging Face 模型镜像" org
// (hf-models). LFS weights are listed as ~134-byte pointer blobs, so the real
// size and sha256 are decoded from the pointer content: every file within
// pointer size range costs one extra (spaced) blob request. Set GITEE_TOKEN
// for private repos or higher rate limits.
type Gitee struct {
	BaseURL string
	Client  *http.Client
	Token   string

	mu       sync.Mutex
	lastCall time.Time
	spacing  time.Duration
}

func NewGitee() *Gitee {
	return &Gitee{
		BaseURL: "https://gitee.com/api/v5",
		Client:  newHTTPClient(),
		Token:   os.Getenv("GITEE_TOKEN"),
		spacing: giteeRequestSpacing,
	}
}

// throttle spaces out API calls across all models polled by this poller.
func (g *Gitee) throttle() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if wait := g.spacing - time.Since(g.lastCall); wait > 0 {
		time.Sleep(wait)
	}
	g.lastCall = time.Now()
}

// backOff doubles the spacing after a 403 and returns true when the response
// is worth retrying later rather than trusting.
func (g *Gitee) backOff(code int) {
	if code != http.StatusForbidden {
		return
	}
	g.mu.Lock()
	if g.spacing < giteeMaxSpacing {
		g.spacing *= 2
		if g.spacing > giteeMaxSpacing {
			g.spacing = giteeMaxSpacing
		}
	}
	g.mu.Unlock()
}

func (g *Gitee) Name() string { return "gitee" }

func (g *Gitee) Snapshot(ctx context.Context, repoID string) (state.RegistryState, error) {
	st := state.RegistryState{RepoID: repoID, CheckedAt: time.Now().UTC()}

	var repo struct {
		DefaultBranch string `json:"default_branch"`
	}
	g.throttle()
	code, err := getJSON(ctx, g.Client, g.authed(g.BaseURL+"/repos/"+repoID), "", &repo)
	g.backOff(code)
	if err != nil {
		return st, fmt.Errorf("gitee: %w", err)
	}
	if code == http.StatusNotFound || code == http.StatusUnauthorized {
		return st, nil
	}
	if code == http.StatusForbidden {
		return st, fmt.Errorf("gitee: HTTP 403 fetching %s — rate limited; set GITEE_TOKEN for higher limits", repoID)
	}
	if code != http.StatusOK {
		return st, fmt.Errorf("gitee: unexpected HTTP %d fetching %s", code, repoID)
	}
	branch := repo.DefaultBranch
	if branch == "" {
		branch = "master"
	}

	var head struct {
		Commit struct {
			Sha string `json:"sha"`
		} `json:"commit"`
	}
	g.throttle()
	code, err = getJSON(ctx, g.Client, g.authed(g.BaseURL+"/repos/"+repoID+"/branches/"+branch), "", &head)
	g.backOff(code)
	if err != nil {
		return st, fmt.Errorf("gitee: %w", err)
	}
	if code == http.StatusForbidden {
		return st, fmt.Errorf("gitee: HTTP 403 fetching the head of %s — rate limited; set GITEE_TOKEN for higher limits", branch)
	}
	if code != http.StatusOK {
		return st, fmt.Errorf("gitee: unexpected HTTP %d fetching the head of %s", code, branch)
	}

	var tree struct {
		Tree      []giteeTreeEntry `json:"tree"`
		Truncated bool             `json:"truncated"`
	}
	treeURL := fmt.Sprintf("%s/repos/%s/git/trees/%s?recursive=1", g.BaseURL, repoID, branch)
	g.throttle()
	code, err = getJSON(ctx, g.Client, g.authed(treeURL), "", &tree)
	g.backOff(code)
	if err != nil {
		return st, fmt.Errorf("gitee: %w", err)
	}
	if code == http.StatusForbidden {
		return st, fmt.Errorf("gitee: HTTP 403 fetching the tree of %s — rate limited; set GITEE_TOKEN for higher limits", repoID)
	}
	if code != http.StatusOK {
		return st, fmt.Errorf("gitee: unexpected HTTP %d fetching the tree of %s", code, repoID)
	}
	if tree.Truncated {
		return st, fmt.Errorf("gitee: the tree listing of %s is truncated; the repo is too large for one pass", repoID)
	}

	st.Present = true
	st.CommitSha = head.Commit.Sha
	st.Files = make(map[string]state.FileState, len(tree.Tree))
	undecoded := 0
	for _, e := range tree.Tree {
		if e.Type != "blob" {
			continue
		}
		fs := state.FileState{Size: e.Size, Present: true}
		if e.Size > 0 && e.Size < lfsPointerMaxSize {
			// A failed pointer decode leaves the tree metadata (pointer
			// size, no sha); the listing is then marked Partial instead of
			// silently reading as divergence in the report.
			if oid, size, ok := g.lfsPointer(ctx, repoID, e.Sha); ok {
				fs.LfsSha256 = oid
				fs.Size = size
			} else {
				undecoded++
			}
		}
		st.Files[e.Path] = fs
	}
	if undecoded > 0 {
		st.Partial = true
	}
	return st, nil
}

// lfsPointer fetches one blob and decodes it if it is a Git LFS pointer.
func (g *Gitee) lfsPointer(ctx context.Context, repoID, sha string) (oid string, size int64, ok bool) {
	var blob struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	g.throttle()
	g.throttle()
	code, err := getJSON(ctx, g.Client, g.authed(fmt.Sprintf("%s/repos/%s/git/blobs/%s", g.BaseURL, repoID, sha)), "", &blob)
	g.backOff(code)
	if err != nil || code != http.StatusOK {
		return "", 0, false
	}
	var raw []byte
	if blob.Encoding == "base64" {
		raw, err = base64.StdEncoding.DecodeString(blob.Content)
		if err != nil {
			return "", 0, false
		}
	} else {
		raw = []byte(blob.Content)
	}
	return parseLFSPointer(string(raw))
}

// authed appends the access_token query parameter Gitee's API expects.
func (g *Gitee) authed(u string) string {
	if g.Token == "" {
		return u
	}
	sep := "?"
	if strings.Contains(u, "?") {
		sep = "&"
	}
	return u + sep + "access_token=" + url.QueryEscape(g.Token)
}

type giteeTreeEntry struct {
	Type string `json:"type"` // "blob" (file), "tree" (dir), "commit" (submodule)
	Path string `json:"path"`
	Sha  string `json:"sha"`
	Size int64  `json:"size"`
}

// parseLFSPointer decodes a Git LFS pointer
// ("version https://git-lfs...\noid sha256:<hex>\nsize <bytes>") into the
// real sha256 and size of the weight file it stands for.
func parseLFSPointer(content string) (oid string, size int64, ok bool) {
	if !strings.HasPrefix(content, "version https://git-lfs") {
		return "", 0, false
	}
	for _, line := range strings.Split(content, "\n") {
		if v, found := strings.CutPrefix(line, "oid sha256:"); found {
			oid = strings.TrimSpace(v)
		} else if v, found := strings.CutPrefix(line, "size "); found {
			size, _ = strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		}
	}
	if oid == "" || size <= 0 {
		return "", 0, false
	}
	return oid, size, true
}

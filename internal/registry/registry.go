// Package registry snapshots model repos from the registries WeightCanary
// watches. All three are public REST endpoints read with plain HTTP — no
// SDKs. The poller reality verified against the live APIs (v0.1):
//
//   - Hugging Face: /api/models/{id} yields the repo commit sha, then
//     /api/models/{id}/tree/{sha}?recursive=true yields per-file size and
//     lfs.oid (sha256). No per-file mtime is exposed, so freshness is
//     size+sha256 comparison against the last snapshot. A missing or gated
//     repo answers HTTP 401, not 404 — both mean "absent" here.
//   - ModelScope: /api/v1/models/{id}/repo/files?Recursive=true returns the
//     whole listing in one response (the Page/PageSize params are accepted
//     but ignored) with per-file Size and, for LFS weights, a Sha256 that
//     matches the Hugging Face lfs.oid. A missing repo answers HTTP 200 with
//     Success=false, not an HTTP error.
//   - Gitee: /git/trees/{ref}?recursive=1 lists blobs, but LFS weights show
//     up as ~134-byte pointer blobs, so the real size and sha256 are decoded
//     from the pointer content fetched per small file.
package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/SuperMarioYL/weightcanary/internal/state"
)

// Poller reads one registry. Implementations are safe for concurrent use.
type Poller interface {
	// Name is the registry key used in models.yaml mirrors and report rows.
	Name() string
	// Snapshot lists one repo. A missing or private repo is not an error: it
	// returns a state with Present=false. Only transport or parse failures
	// return an error.
	Snapshot(ctx context.Context, repoID string) (state.RegistryState, error)
}

func newHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

// getJSON fetches url and decodes the JSON body into out. It returns the HTTP
// status code so callers can classify "absent" responses (401/404 on some
// registries) separately from transport errors. When the response advertises a
// continuation (a Link rel="next" header — the Hugging Face tree API paginates
// this way), the absolute URL of the next page is returned so the caller can
// read the whole listing.
func getJSON(ctx context.Context, client *http.Client, url string, token string, out any) (next string, code int, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("User-Agent", "weightcanary")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK && out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return "", resp.StatusCode, fmt.Errorf("decode %s: %w", url, err)
		}
		return nextPageLink(resp.Header.Get("Link")), resp.StatusCode, nil
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return "", resp.StatusCode, nil
}

// nextPageLink extracts the URL of the next page from an RFC 5988 Link header
// ("<url>; rel=\"next\"", possibly among several targets), or "" when the
// listing ends here.
func nextPageLink(header string) string {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) {
			continue
		}
		start, end := strings.Index(part, "<"), strings.Index(part, ">")
		if start != -1 && end > start {
			return part[start+1 : end]
		}
	}
	return ""
}

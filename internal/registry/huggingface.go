package registry

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/SuperMarioYL/weightcanary/internal/state"
)

// HuggingFace snapshots repos on huggingface.co. A missing or gated repo
// answers HTTP 401; both it and 404 mean "absent". Set HF_TOKEN to read gated
// repos you have access to.
type HuggingFace struct {
	BaseURL string
	Client  *http.Client
	Token   string
}

func NewHuggingFace() *HuggingFace {
	return &HuggingFace{
		BaseURL: "https://huggingface.co",
		Client:  newHTTPClient(),
		Token:   os.Getenv("HF_TOKEN"),
	}
}

func (h *HuggingFace) Name() string { return "huggingface" }

func (h *HuggingFace) Snapshot(ctx context.Context, repoID string) (state.RegistryState, error) {
	st := state.RegistryState{RepoID: repoID, CheckedAt: time.Now().UTC()}

	var meta struct {
		Sha string `json:"sha"`
	}
	code, err := getJSON(ctx, h.Client, h.BaseURL+"/api/models/"+repoID, h.Token, &meta)
	if err != nil {
		return st, fmt.Errorf("huggingface: %w", err)
	}
	if code == http.StatusUnauthorized || code == http.StatusNotFound {
		// HF answers 401 (not 404) for a missing or gated repo: absent.
		return st, nil
	}
	if code != http.StatusOK {
		return st, fmt.Errorf("huggingface: unexpected HTTP %d fetching %s", code, repoID)
	}

	var tree []hfTreeEntry
	code, err = getJSON(ctx, h.Client, h.BaseURL+"/api/models/"+repoID+"/tree/"+meta.Sha+"?recursive=true", h.Token, &tree)
	if err != nil {
		return st, fmt.Errorf("huggingface: %w", err)
	}
	if code != http.StatusOK {
		return st, fmt.Errorf("huggingface: unexpected HTTP %d fetching the file tree of %s", code, repoID)
	}

	st.Present = true
	st.CommitSha = meta.Sha
	st.Files = make(map[string]state.FileState, len(tree))
	for _, e := range tree {
		if e.Type != "file" {
			continue
		}
		fs := state.FileState{Size: e.Size, Present: true}
		if e.LFS != nil {
			fs.LfsSha256 = e.LFS.Oid
		}
		st.Files[e.Path] = fs
	}
	return st, nil
}

type hfTreeEntry struct {
	Type string `json:"type"`
	Path string `json:"path"`
	Size int64  `json:"size"`
	LFS  *struct {
		Oid  string `json:"oid"`
		Size int64  `json:"size"`
	} `json:"lfs"`
}

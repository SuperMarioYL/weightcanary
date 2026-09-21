package registry

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/SuperMarioYL/weightcanary/internal/state"
)

// ModelScope snapshots repos on modelscope.cn. The files endpoint returns the
// whole recursive listing in one response; for LFS weights its per-file
// Sha256 matches the Hugging Face lfs.oid. A missing repo answers HTTP 200
// with Success=false rather than an HTTP error status.
type ModelScope struct {
	BaseURL string
	Client  *http.Client
}

func NewModelScope() *ModelScope {
	return &ModelScope{BaseURL: "https://modelscope.cn", Client: newHTTPClient()}
}

func (m *ModelScope) Name() string { return "modelscope" }

func (m *ModelScope) Snapshot(ctx context.Context, repoID string) (state.RegistryState, error) {
	st := state.RegistryState{RepoID: repoID, CheckedAt: time.Now().UTC()}

	var page struct {
		Code    int  `json:"Code"`
		Success bool `json:"Success"`
		Data    struct {
			Files []struct {
				Type   string `json:"Type"` // "blob" (file) or "tree" (dir)
				Path   string `json:"Path"`
				Size   int64  `json:"Size"`
				IsLFS  bool   `json:"IsLFS"`
				Sha256 string `json:"Sha256"`
			} `json:"Files"`
		} `json:"Data"`
	}
	url := m.BaseURL + "/api/v1/models/" + repoID + "/repo/files?Revision=master&Root=&Recursive=true"
	code, err := getJSON(ctx, m.Client, url, "", &page)
	if err != nil {
		return st, fmt.Errorf("modelscope: %w", err)
	}
	if code != http.StatusOK {
		return st, fmt.Errorf("modelscope: unexpected HTTP %d fetching %s", code, repoID)
	}
	if !page.Success || page.Code != 200 {
		// Absent repo: HTTP 200 carrying an application-level error code.
		return st, nil
	}

	st.Present = true
	st.Files = make(map[string]state.FileState, len(page.Data.Files))
	for _, e := range page.Data.Files {
		if e.Type != "blob" {
			continue
		}
		fs := state.FileState{Size: e.Size, Present: true}
		// Only LFS weights carry a sha256 comparable across registries; for
		// plain files the field is a git blob hash convention, so size is the
		// honest comparison signal there.
		if e.IsLFS {
			fs.LfsSha256 = e.Sha256
		}
		st.Files[e.Path] = fs
	}
	return st, nil
}

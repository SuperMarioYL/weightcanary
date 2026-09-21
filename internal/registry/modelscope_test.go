package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestModelScopeSnapshot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/models/deepseek-ai/DeepSeek-V3.1/repo/files" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.URL.Query().Get("Recursive") != "true" || r.URL.Query().Get("Revision") != "master" {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		w.Write([]byte(`{
			"Code": 200, "Success": true,
			"Data": {"Files": [
				{"Type":"tree","Path":"assets","Size":0,"IsLFS":false,"Sha256":""},
				{"Type":"blob","Path":"config.json","Size":1007,"IsLFS":false,"Sha256":"c8a792d9"},
				{"Type":"blob","Path":"model-00001-of-000163.safetensors","Size":5234139343,"IsLFS":true,
				 "Sha256":"ba38046c0ea567aa2cd032b31b26a58bdba73fb5687da60ffa25b8d11cfdb4e4"}
			]}
		}`))
	}))
	defer srv.Close()

	ms := &ModelScope{BaseURL: srv.URL, Client: srv.Client()}
	st, err := ms.Snapshot(context.Background(), "deepseek-ai/DeepSeek-V3.1")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Present {
		t.Fatalf("state = %+v", st)
	}
	// Tree entries are skipped; the two blobs remain.
	if len(st.Files) != 2 {
		t.Fatalf("want 2 blobs, got %d", len(st.Files))
	}
	lfs := st.Files["model-00001-of-000163.safetensors"]
	if lfs.LfsSha256 != "ba38046c0ea567aa2cd032b31b26a58bdba73fb5687da60ffa25b8d11cfdb4e4" {
		t.Fatalf("lfs sha = %q", lfs.LfsSha256)
	}
	// Plain-file hashes are a git blob convention, not the LFS sha: unset.
	if got := st.Files["config.json"]; got.LfsSha256 != "" || got.Size != 1007 {
		t.Fatalf("plain file state = %+v", got)
	}
}

func TestModelScopeMissingRepoIsAbsent(t *testing.T) {
	// ModelScope answers HTTP 200 with Success=false for a missing repo.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Code":10010205001,"Message":"record not found","Success":false}`))
	}))
	defer srv.Close()

	ms := &ModelScope{BaseURL: srv.URL, Client: srv.Client()}
	st, err := ms.Snapshot(context.Background(), "gone/nope")
	if err != nil {
		t.Fatal(err)
	}
	if st.Present || st.Error != "" {
		t.Fatalf("expected absent without error, got %+v", st)
	}
}

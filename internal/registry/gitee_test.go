package registry

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testPointer = "version https://git-lfs.github.com/spec/v1\noid sha256:c3ec0ed85ee95cae5c9b76a7af7272a23169b81705c177c0212f65b24d4b8b41\nsize 3753953568\n"

func TestGiteeSnapshotDecodesLFSPointers(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)

	mux.HandleFunc("/repos/hf-models/GLM-4.5", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"full_name":"hf-models/GLM-4.5","default_branch":"main"}`))
	})
	mux.HandleFunc("/repos/hf-models/GLM-4.5/branches/main", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"name":"main","commit":{"sha":"9cfe10c892f5772a937adb8176ce0f7f6900a0dd"}}`))
	})
	mux.HandleFunc("/repos/hf-models/GLM-4.5/git/trees/main", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("recursive") != "1" {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		w.Write([]byte(fmt.Sprintf(`{
			"truncated": false,
			"tree": [
				{"type":"tree","path":"assets","sha":"abc"},
				{"type":"blob","path":"config.json","sha":"sha-config","size":1007},
				{"type":"blob","path":"model-00001-of-00093.safetensors","sha":"sha-pointer","size":135}
			]
		}`)))
	})
	mux.HandleFunc("/repos/hf-models/GLM-4.5/git/blobs/sha-pointer", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"content":"` + base64.StdEncoding.EncodeToString([]byte(testPointer)) + `","encoding":"base64"}`))
	})

	g := &Gitee{BaseURL: srv.URL, Client: srv.Client()}
	st, err := g.Snapshot(context.Background(), "hf-models/GLM-4.5")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Present || st.CommitSha != "9cfe10c892f5772a937adb8176ce0f7f6900a0dd" {
		t.Fatalf("state = %+v", st)
	}
	if len(st.Files) != 2 {
		t.Fatalf("want 2 blobs, got %d", len(st.Files))
	}
	lfs := st.Files["model-00001-of-00093.safetensors"]
	if lfs.LfsSha256 != "c3ec0ed85ee95cae5c9b76a7af7272a23169b81705c177c0212f65b24d4b8b41" || lfs.Size != 3753953568 {
		t.Fatalf("pointer decode failed: %+v", lfs)
	}
	// The small non-pointer file keeps its tree size and gains no sha.
	if got := st.Files["config.json"]; got.Size != 1007 || got.LfsSha256 != "" {
		t.Fatalf("plain file state = %+v", got)
	}
}

func TestGiteeMissingRepoIsAbsent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	g := &Gitee{BaseURL: srv.URL, Client: srv.Client()}
	st, err := g.Snapshot(context.Background(), "gone/nope")
	if err != nil {
		t.Fatal(err)
	}
	if st.Present || st.Error != "" {
		t.Fatalf("expected absent without error, got %+v", st)
	}
}

func TestGiteeTruncatedTreeIsAnError(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	mux.HandleFunc("/repos/a/b", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"default_branch":"main"}`))
	})
	mux.HandleFunc("/repos/a/b/branches/main", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"commit":{"sha":"abc"}}`))
	})
	mux.HandleFunc("/repos/a/b/git/trees/main", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"truncated": true, "tree": []}`))
	})

	g := &Gitee{BaseURL: srv.URL, Client: srv.Client()}
	if _, err := g.Snapshot(context.Background(), "a/b"); err == nil || !strings.Contains(err.Error(), "truncated") {
		t.Fatalf("want truncation error, got %v", err)
	}
}

func TestParseLFSPointer(t *testing.T) {
	oid, size, ok := parseLFSPointer(testPointer)
	if !ok || oid != "c3ec0ed85ee95cae5c9b76a7af7272a23169b81705c177c0212f65b24d4b8b41" || size != 3753953568 {
		t.Fatalf("parse = %q %d %v", oid, size, ok)
	}
	if _, _, ok := parseLFSPointer("not a pointer"); ok {
		t.Fatal("plain content must not parse as a pointer")
	}
}

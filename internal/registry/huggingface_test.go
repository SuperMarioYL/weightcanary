package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHuggingFaceSnapshot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/models/deepseek-ai/DeepSeek-V3.1":
			// The tree request must carry the commit sha from the meta call.
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"sha":"c0781d03","siblings":[{"rfilename":"README.md"}]}`))
		case r.URL.Path == "/api/models/deepseek-ai/DeepSeek-V3.1/tree/c0781d03":
			if r.URL.Query().Get("recursive") != "true" {
				t.Errorf("tree request must be recursive, got %q", r.URL.RawQuery)
			}
			w.Write([]byte(`[
				{"type":"file","oid":"8ea6b5b5","size":1686,"path":"config.json"},
				{"type":"directory","size":0,"path":"assets"},
				{"type":"file","oid":"7e4877d7","size":5234139343,"path":"model-00001-of-000163.safetensors",
				 "lfs":{"oid":"ba38046c0ea567aa2cd032b31b26a58bdba73fb5687da60ffa25b8d11cfdb4e4","size":5234139343,"pointerSize":135}}
			]`))
		default:
			t.Errorf("unexpected request path %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	hf := &HuggingFace{BaseURL: srv.URL, Client: srv.Client()}
	st, err := hf.Snapshot(context.Background(), "deepseek-ai/DeepSeek-V3.1")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Present || st.CommitSha != "c0781d03" {
		t.Fatalf("state = %+v", st)
	}
	if len(st.Files) != 2 {
		t.Fatalf("want 2 files (directories skipped), got %d", len(st.Files))
	}
	lfs := st.Files["model-00001-of-000163.safetensors"]
	if lfs.LfsSha256 != "ba38046c0ea567aa2cd032b31b26a58bdba73fb5687da60ffa25b8d11cfdb4e4" || lfs.Size != 5234139343 {
		t.Fatalf("lfs state = %+v", lfs)
	}
	if got := st.Files["config.json"]; got.Size != 1686 || got.LfsSha256 != "" {
		t.Fatalf("plain file state = %+v", got)
	}
}

func TestHuggingFaceTreePaginationFollowed(t *testing.T) {
	// The tree API paginates at 1000 entries per page and hands back the next
	// page as a Link rel="next" header. The poller must follow it, or repos
	// with more files than one page silently record a truncated ledger.
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/models/a/b/tree/s1" {
			w.Write([]byte(`{"sha":"s1"}`))
			return
		}
		switch r.URL.Query().Get("cursor") {
		case "":
			w.Header().Set("Link", `<`+srv.URL+`/api/models/a/b/tree/s1?cursor=p2>; rel="next"`)
			w.Write([]byte(`[
				{"type":"file","oid":"a1","size":10,"path":"page1-file-a"},
				{"type":"file","oid":"a2","size":20,"path":"page1-file-b"}
			]`))
		case "p2":
			w.Write([]byte(`[
				{"type":"file","oid":"b1","size":30,"path":"page2-file-c"}
			]`))
		default:
			t.Errorf("unexpected cursor %q", r.URL.Query().Get("cursor"))
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	hf := &HuggingFace{BaseURL: srv.URL, Client: srv.Client()}
	st, err := hf.Snapshot(context.Background(), "a/b")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Present {
		t.Fatalf("state = %+v", st)
	}
	for _, path := range []string{"page1-file-a", "page1-file-b", "page2-file-c"} {
		f, ok := st.Files[path]
		if !ok {
			t.Fatalf("file %q missing from the ledger — a page was dropped", path)
		}
		if !f.Present {
			t.Fatalf("file %q = %+v", path, f)
		}
	}
}

func TestHuggingFaceMissingRepoIsAbsent(t *testing.T) {
	// HF answers 401, not 404, for a missing or gated repo; both mean absent.
	for _, code := range []int{http.StatusUnauthorized, http.StatusNotFound} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		}))
		hf := &HuggingFace{BaseURL: srv.URL, Client: srv.Client()}
		st, err := hf.Snapshot(context.Background(), "gone/nope")
		if err != nil {
			t.Fatalf("HTTP %d: %v", code, err)
		}
		if st.Present || st.Error != "" {
			t.Fatalf("HTTP %d: expected absent without error, got %+v", code, st)
		}
		srv.Close()
	}
}

func TestHuggingFaceTokenHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/models/a/b/tree") {
			w.Write([]byte(`[]`))
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer hf_test" {
			t.Errorf("Authorization = %q", got)
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	hf := &HuggingFace{BaseURL: srv.URL, Client: srv.Client(), Token: "hf_test"}
	if _, err := hf.Snapshot(context.Background(), "a/b"); err != nil {
		t.Fatal(err)
	}
}

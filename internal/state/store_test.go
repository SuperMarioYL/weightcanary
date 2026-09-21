package state

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLoadRoundtrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	snap := &Snapshot{
		Version:     SchemaVersion,
		GeneratedAt: now,
		Models: []AvailabilityRecord{{
			ModelID:   "deepseek-ai/DeepSeek-V3.1",
			CheckedAt: now,
			HF: RegistryState{
				RepoID:    "deepseek-ai/DeepSeek-V3.1",
				Present:   true,
				CommitSha: "c0781d039fb7a1ba2abc4add0bdc293e92d2b8db",
				Files: map[string]FileState{
					"config.json": {Size: 1686, Present: true},
					"model.safetensors": {
						Size:      5234139343,
						LfsSha256: "ba38046c0ea567aa2cd032b31b26a58bdba73fb5687da60ffa25b8d11cfdb4e4",
						Present:   true,
					},
				},
				CheckedAt: now,
			},
			Mirrors: map[string]RegistryState{
				"modelscope": {
					RepoID:  "deepseek-ai/DeepSeek-V3.1",
					Present: true,
					Files: map[string]FileState{
						"config.json": {Size: 1686, Present: true},
					},
					CheckedAt: now,
				},
			},
		}},
	}

	path := filepath.Join(t.TempDir(), "state.json")
	if err := Save(path, snap); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != SchemaVersion || len(got.Models) != 1 {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
	rec := got.Models[0]
	if rec.HF.CommitSha != snap.Models[0].HF.CommitSha {
		t.Fatalf("commit sha = %q", rec.HF.CommitSha)
	}
	lfs := rec.HF.Files["model.safetensors"]
	if lfs.LfsSha256 != snap.Models[0].HF.Files["model.safetensors"].LfsSha256 || lfs.Size != 5234139343 {
		t.Fatalf("lfs file state = %+v", lfs)
	}
	if !rec.CheckedAt.Equal(now) || !got.GeneratedAt.Equal(now) {
		t.Fatal("timestamps did not roundtrip")
	}
	if rec.Mirrors["modelscope"].RepoID != "deepseek-ai/DeepSeek-V3.1" {
		t.Fatalf("mirror state = %+v", rec.Mirrors["modelscope"])
	}
}

func TestLoadRejectsWrongSchemaVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := Save(path, &Snapshot{Version: SchemaVersion + 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("wrong schema version should fail")
	}
}

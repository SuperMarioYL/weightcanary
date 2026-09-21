package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/SuperMarioYL/weightcanary/internal/state"
)

func fixture(t *testing.T) *state.Snapshot {
	t.Helper()
	now := time.Now().UTC()
	files := func(pairs map[string]state.FileState) map[string]state.FileState { return pairs }
	return &state.Snapshot{
		Version:     state.SchemaVersion,
		GeneratedAt: now,
		Models: []state.AvailabilityRecord{{
			ModelID:   "zai-org/GLM-4.5",
			CheckedAt: now,
			HF: state.RegistryState{
				RepoID: "zai-org/GLM-4.5", Present: true, CommitSha: "abc1234def5678",
				Files: files(map[string]state.FileState{
					"config.json":             {Size: 1007, Present: true},
					"model-00001.safetensors": {Size: 3753953568, LfsSha256: "c3ec0ed8", Present: true},
				}),
				CheckedAt: now,
			},
			Mirrors: map[string]state.RegistryState{
				"modelscope": {
					RepoID: "ZhipuAI/GLM-4.5", Present: true,
					Files: files(map[string]state.FileState{
						"config.json":             {Size: 1007, Present: true},
						"model-00001.safetensors": {Size: 3753953568, LfsSha256: "c3ec0ed8", Present: true},
					}),
					CheckedAt: now,
				},
				"gitee": {
					RepoID: "hf-models/GLM-4.5", Present: true,
					Files: files(map[string]state.FileState{
						"config.json": {Size: 1007, Present: true},
					}),
					CheckedAt: now,
				},
			},
		}},
	}
}

// flatten collapses whitespace so assertions survive tabwriter column padding.
func flatten(out string) string {
	return strings.Join(strings.Fields(out), " ")
}

func TestRenderInSyncAndDivergent(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, fixture(t), false); err != nil {
		t.Fatal(err)
	}
	out := flatten(buf.String())
	for _, want := range []string{
		"zai-org/GLM-4.5",
		"huggingface ok 2 files 3.5 GiB commit abc1234",
		"modelscope ok 2 files 3.5 GiB in sync",
		"gitee ok 1 files 1007 B",
		"divergent: 1 missing (1 weights)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("report missing %q:\n%s", want, buf.String())
		}
	}
}

func TestRenderErrorAndAbsent(t *testing.T) {
	snap := fixture(t)
	snap.Models[0].Mirrors["modelscope"] = state.RegistryState{
		RepoID: "ZhipuAI/GLM-4.5", Error: "modelscope: context deadline exceeded",
	}
	snap.Models[0].Mirrors["gitee"] = state.RegistryState{RepoID: "hf-models/GLM-4.5"}
	snap.Models[0].HF = state.RegistryState{RepoID: "zai-org/GLM-4.5"}

	var buf bytes.Buffer
	if err := Render(&buf, snap, false); err != nil {
		t.Fatal(err)
	}
	out := flatten(buf.String())
	for _, want := range []string{
		"huggingface absent 0 files 0 B repo missing or private",
		"modelscope error 0 files 0 B modelscope: context deadline exceeded",
		"gitee absent 0 files 0 B repo missing or private",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("report missing %q:\n%s", want, buf.String())
		}
	}
}

func TestRenderFilesFlag(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, fixture(t), true); err != nil {
		t.Fatal(err)
	}
	out := flatten(buf.String())
	for _, want := range []string{
		"huggingface: 2 files",
		"model-00001.safetensors 3.5 GiB c3ec0ed8",
		"config.json 1007 B",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("report missing %q:\n%s", want, buf.String())
		}
	}
}

func TestCompareMirror(t *testing.T) {
	hf := state.RegistryState{Present: true, Files: map[string]state.FileState{
		"config.json":   {Size: 100, Present: true},
		"a.safetensors": {Size: 10, LfsSha256: "aaa", Present: true},
		"b.safetensors": {Size: 20, LfsSha256: "bbb", Present: true},
	}}
	synced := state.RegistryState{Present: true, Files: map[string]state.FileState{
		"config.json":    {Size: 100, Present: true},
		"a.safetensors":  {Size: 10, LfsSha256: "aaa", Present: true},
		"b.safetensors":  {Size: 20, LfsSha256: "bbb", Present: true},
		"mirror-only.md": {Size: 5, Present: true},
	}}
	s := CompareMirror(hf, synced)
	if !s.InSync() || s.Extra != 1 {
		t.Fatalf("in-sync mirror misjudged: %+v", s)
	}

	diverged := state.RegistryState{Present: true, Files: map[string]state.FileState{
		"config.json":   {Size: 999, Present: true},                  // changed
		"a.safetensors": {Size: 10, LfsSha256: "aaa", Present: true}, // same
	}}
	s = CompareMirror(hf, diverged)
	if s.InSync() || s.Missing != 1 || s.MissingWeights != 1 || s.Changed != 1 {
		t.Fatalf("divergence misjudged: %+v", s)
	}

	// Storage-plumbing differences are not divergence: a mirror may hold a
	// big plain file (tokenizer.json) as Git LFS while the source keeps it
	// as a git blob. Same size, sha only on one side → same content.
	plumbed := state.RegistryState{Present: true, Files: map[string]state.FileState{
		"config.json":    {Size: 100, Present: true},
		"a.safetensors":  {Size: 10, LfsSha256: "aaa", Present: true},
		"b.safetensors":  {Size: 20, LfsSha256: "bbb", Present: true},
		"tokenizer.json": {Size: 7847578, LfsSha256: "stored-as-lfs", Present: true},
	}}
	src := state.RegistryState{Present: true, Files: map[string]state.FileState{
		"config.json":    {Size: 100, Present: true},
		"a.safetensors":  {Size: 10, LfsSha256: "aaa", Present: true},
		"b.safetensors":  {Size: 20, LfsSha256: "bbb", Present: true},
		"tokenizer.json": {Size: 7847578, Present: true},
	}}
	if s := CompareMirror(src, plumbed); !s.InSync() {
		t.Fatalf("storage plumbing misjudged as divergence: %+v", s)
	}

	// Two different LFS hashes with equal size are a real divergence.
	swapped := state.RegistryState{Present: true, Files: map[string]state.FileState{
		"config.json":   {Size: 100, Present: true},
		"a.safetensors": {Size: 10, LfsSha256: "aaa", Present: true},
		"b.safetensors": {Size: 20, LfsSha256: "different-hash", Present: true},
	}}
	if s := CompareMirror(hf, swapped); s.InSync() || s.Changed != 1 {
		t.Fatalf("hash mismatch misjudged: %+v", s)
	}
}

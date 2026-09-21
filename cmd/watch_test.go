package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/SuperMarioYL/weightcanary/internal/config"
	"github.com/SuperMarioYL/weightcanary/internal/registry"
	"github.com/SuperMarioYL/weightcanary/internal/report"
	"github.com/SuperMarioYL/weightcanary/internal/state"
)

// recordingPoller is a fake registry that records every repo id it is asked
// to poll, so tests can assert no request is issued for an unconfigured site.
type recordingPoller struct {
	name  string
	calls []string
}

func (p *recordingPoller) Name() string { return p.name }

func (p *recordingPoller) Snapshot(ctx context.Context, repoID string) (state.RegistryState, error) {
	p.calls = append(p.calls, repoID)
	return state.RegistryState{RepoID: repoID, Present: true, CheckedAt: time.Now().UTC()}, nil
}

// Regression: a mirror omitted from models.yaml is simply not watched (see
// README, "缺一个就不盯那个站"). pollModel must not poll it with an empty repo
// id and the report must render no row for it.
func TestPollModelSkipsUnconfiguredMirrors(t *testing.T) {
	hf := &recordingPoller{name: "huggingface"}
	modelscope := &recordingPoller{name: "modelscope"}
	gitee := &recordingPoller{name: "gitee"}
	pollers := map[string]registry.Poller{
		"modelscope": modelscope,
		"gitee":      gitee,
	}
	m := config.ModelConfig{
		ID: "deepseek-ai/DeepSeek-V3.1",
		Mirrors: map[string]string{
			"modelscope": "deepseek-ai/DeepSeek-V3.1",
			// gitee deliberately omitted
		},
	}

	rec := pollModel(context.Background(), m, hf, pollers)

	if got := gitee.calls; len(got) != 0 {
		t.Fatalf("gitee polled with repo ids %v; want no calls for an unconfigured mirror", got)
	}
	if _, ok := rec.Mirrors["gitee"]; ok {
		t.Fatal("gitee state recorded for an unconfigured mirror; want the record left unset")
	}
	if got, want := hf.calls, []string{"deepseek-ai/DeepSeek-V3.1"}; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("huggingface polled %v, want %v", got, want)
	}
	if got, want := modelscope.calls, []string{"deepseek-ai/DeepSeek-V3.1"}; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("modelscope polled %v, want %v", got, want)
	}

	var out bytes.Buffer
	snap := &state.Snapshot{Version: state.SchemaVersion, GeneratedAt: time.Now().UTC(), Models: []state.AvailabilityRecord{rec}}
	if err := report.Render(&out, snap, false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "gitee") {
		t.Fatalf("report mentions gitee for a model that does not watch it:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "modelscope") {
		t.Fatalf("report lost the configured modelscope row:\n%s", out.String())
	}
}

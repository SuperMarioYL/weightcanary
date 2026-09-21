package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "models.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadValid(t *testing.T) {
	cfg, err := Load(writeTemp(t, `
models:
  - id: deepseek-ai/DeepSeek-V3.1
    mirrors:
      modelscope: deepseek-ai/DeepSeek-V3.1
      gitee: hf-models/DeepSeek-V3.1
  - id: zai-org/GLM-4.5
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Models) != 2 {
		t.Fatalf("got %d models, want 2", len(cfg.Models))
	}
	if got := cfg.Models[0].Mirrors["modelscope"]; got != "deepseek-ai/DeepSeek-V3.1" {
		t.Fatalf("modelscope mirror = %q", got)
	}
	if cfg.Models[1].Mirrors != nil {
		t.Fatalf("mirrors without entry should stay nil, got %v", cfg.Models[1].Mirrors)
	}
}

func TestLoadUnknownMirrorKey(t *testing.T) {
	_, err := Load(writeTemp(t, `
models:
  - id: a/b
    mirrors:
      github: a/b
`))
	if err == nil || !strings.Contains(err.Error(), "unknown mirror registry") {
		t.Fatalf("want unknown mirror error, got %v", err)
	}
}

func TestLoadEmptyAndDuplicate(t *testing.T) {
	if _, err := Load(writeTemp(t, "models: []\n")); err == nil {
		t.Fatal("empty watchlist should fail")
	}
	if _, err := Load(writeTemp(t, "models:\n  - id: a/b\n  - id: a/b\n")); err == nil {
		t.Fatal("duplicate id should fail")
	}
}

func TestDefaultYAMLParsesAndPrefills(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte(DefaultYAML()), &cfg); err != nil {
		t.Fatal(err)
	}
	if err := cfg.validate(); err != nil {
		t.Fatalf("default watchlist invalid: %v", err)
	}
	ids := make([]string, 0, len(cfg.Models))
	for _, m := range cfg.Models {
		ids = append(ids, m.ID)
	}
	want := "deepseek-ai/DeepSeek-V3.1 Qwen/Qwen3-235B-A22B zai-org/GLM-4.5"
	if got := strings.Join(ids, " "); got != want {
		t.Fatalf("default models = %q, want %q", got, want)
	}
	if got := cfg.Models[2].Mirrors["modelscope"]; got != "ZhipuAI/GLM-4.5" {
		t.Fatalf("GLM-4.5 modelscope id = %q, want ZhipuAI/GLM-4.5", got)
	}
}

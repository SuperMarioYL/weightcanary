// Package config loads the WeightCanary watchlist (models.yaml): which models
// to watch and which per-registry repo ids map to them.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// MirrorKeys lists the mirror registries a model can configure, in report
// order. registry.Poller and this list must cover the same set.
var MirrorKeys = []string{"modelscope", "gitee"}

// ModelConfig is one watched model. The Hugging Face id is the primary key;
// mirrors map registry names to the repo id on that registry, because ids
// differ per registry (ModelScope hosts GLM-4.5 as ZhipuAI/GLM-4.5 while the
// Gitee mirrors sit under the hf-models org).
type ModelConfig struct {
	ID      string            `yaml:"id"`
	Mirrors map[string]string `yaml:"mirrors,omitempty"`
}

// Config is the parsed watchlist.
type Config struct {
	Models []ModelConfig `yaml:"models"`
}

// Load reads and validates a models.yaml watchlist.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if len(c.Models) == 0 {
		return fmt.Errorf("no models listed — add at least one model id")
	}
	seen := map[string]bool{}
	for i, m := range c.Models {
		if m.ID == "" {
			return fmt.Errorf("model #%d: id is required", i+1)
		}
		if seen[m.ID] {
			return fmt.Errorf("model %s: listed twice", m.ID)
		}
		seen[m.ID] = true
		for key := range m.Mirrors {
			if !knownMirror(key) {
				return fmt.Errorf("model %s: unknown mirror registry %q (known: %v)", m.ID, key, MirrorKeys)
			}
			if m.Mirrors[key] == "" {
				return fmt.Errorf("model %s: mirror %q has an empty repo id", m.ID, key)
			}
		}
	}
	return nil
}

func knownMirror(key string) bool {
	for _, k := range MirrorKeys {
		if k == key {
			return true
		}
	}
	return false
}

// DefaultYAML is the pre-filled watchlist written by `weightcanary init` and
// shipped as models.example.yaml. The default targets are the open-weight
// models CN teams most often pin in production.
func DefaultYAML() string {
	return `# WeightCanary watchlist.
# Each entry is keyed by the Hugging Face model id; "mirrors" maps a registry
# name to the repo id on that registry. Ids differ per registry — ModelScope
# hosts GLM-4.5 as ZhipuAI/GLM-4.5, and the Gitee mirrors sit under the
# "Hugging Face 模型镜像" org (hf-models).
models:
  - id: deepseek-ai/DeepSeek-V3.1
    mirrors:
      modelscope: deepseek-ai/DeepSeek-V3.1
      gitee: hf-models/DeepSeek-V3.1

  - id: Qwen/Qwen3-235B-A22B
    mirrors:
      modelscope: Qwen/Qwen3-235B-A22B
      gitee: hf-models/Qwen3-235B-A22B

  - id: zai-org/GLM-4.5
    mirrors:
      modelscope: ZhipuAI/GLM-4.5
      gitee: hf-models/GLM-4.5
`
}

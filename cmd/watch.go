package cmd

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/SuperMarioYL/weightcanary/internal/config"
	"github.com/SuperMarioYL/weightcanary/internal/registry"
	"github.com/SuperMarioYL/weightcanary/internal/report"
	"github.com/SuperMarioYL/weightcanary/internal/state"
)

// NewWatchCmd snapshots the watchlist, saves the ledger and prints the report.
func NewWatchCmd() *cobra.Command {
	var (
		configPath string
		statePath  string
		showFiles  bool
	)
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Snapshot the watchlist and print the availability report",
		Long: `Snapshot every model in the watchlist across Hugging Face and its
configured mirrors, save the result to the state.json ledger and print a
per-model availability report (file presence, mirror sync state, commit sha).

Run it again later — from a terminal, cron or a GitHub Action — to keep the
ledger current; committing state.json turns git history into the audit trail
of what was available and when.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("%s not found — run `weightcanary init` first", configPath)
				}
				return err
			}
			snap, err := runWatch(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			if err := state.Save(statePath, snap); err != nil {
				return err
			}
			return report.Render(cmd.OutOrStdout(), snap, showFiles)
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "models.yaml", "watchlist file")
	cmd.Flags().StringVarP(&statePath, "state", "s", "state.json", "availability ledger file")
	cmd.Flags().BoolVar(&showFiles, "files", false, "also list every observed file per registry")
	return cmd
}

// runWatch polls every configured model. Models run concurrently (bounded);
// records land back in watchlist order so the report is deterministic.
func runWatch(ctx context.Context, cfg *config.Config) (*state.Snapshot, error) {
	hf := registry.NewHuggingFace()
	pollers := map[string]registry.Poller{
		"modelscope": registry.NewModelScope(),
		"gitee":      registry.NewGitee(),
	}

	records := make([]state.AvailabilityRecord, len(cfg.Models))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, m := range cfg.Models {
		wg.Add(1)
		go func(i int, m config.ModelConfig) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			records[i] = pollModel(ctx, m, hf, pollers)
		}(i, m)
	}
	wg.Wait()
	return &state.Snapshot{
		Version:     state.SchemaVersion,
		GeneratedAt: time.Now().UTC(),
		Models:      records,
	}, nil
}

func pollModel(ctx context.Context, m config.ModelConfig, hf registry.Poller, pollers map[string]registry.Poller) state.AvailabilityRecord {
	rec := state.AvailabilityRecord{
		ModelID:   m.ID,
		Mirrors:   map[string]state.RegistryState{},
		CheckedAt: time.Now().UTC(),
	}
	rec.HF = snapshotWith(ctx, hf, m.ID)
	for _, key := range config.MirrorKeys {
		// pollers covers every key in config.MirrorKeys by construction.
		rec.Mirrors[key] = snapshotWith(ctx, pollers[key], m.Mirrors[key])
	}
	return rec
}

// snapshotWith runs one poller and converts a transport failure into a recorded
// error state, so one unreachable registry never aborts the whole run.
func snapshotWith(ctx context.Context, p registry.Poller, repoID string) state.RegistryState {
	st, err := p.Snapshot(ctx, repoID)
	if err != nil {
		return state.RegistryState{RepoID: repoID, Error: err.Error(), CheckedAt: time.Now().UTC()}
	}
	return st
}

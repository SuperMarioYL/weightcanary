// Package cmd holds the weightcanary subcommands: init, watch and report.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/SuperMarioYL/weightcanary/internal/config"
)

// NewInitCmd writes the pre-filled watchlist.
func NewInitCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write a models.yaml watchlist pre-filled with the default models",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			const path = "models.yaml"
			if _, err := os.Stat(path); err == nil && !force {
				return fmt.Errorf("%s already exists (use --force to overwrite)", path)
			}
			if err := os.WriteFile(path, []byte(config.DefaultYAML()), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), `wrote %s with the default watchlist:

  deepseek-ai/DeepSeek-V3.1
  Qwen/Qwen3-235B-A22B
  zai-org/GLM-4.5            (ModelScope id: ZhipuAI/GLM-4.5)

Edit it to add or remove models, then run:

  weightcanary watch
`, path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing models.yaml")
	return cmd
}

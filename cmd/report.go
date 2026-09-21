package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/SuperMarioYL/weightcanary/internal/report"
	"github.com/SuperMarioYL/weightcanary/internal/state"
)

// NewReportCmd re-renders the availability report from the last snapshot.
func NewReportCmd() *cobra.Command {
	var (
		statePath string
		markdown  bool
		showFiles bool
	)
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Re-render the availability report from the last snapshot",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if markdown {
				// m3: markdown ledger with pledge day counters for the README.
				return fmt.Errorf("--markdown is not implemented yet (planned for the pledge-ledger milestone)")
			}
			snap, err := state.Load(statePath)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("%s not found — run `weightcanary watch` first", statePath)
				}
				return err
			}
			return report.Render(cmd.OutOrStdout(), snap, showFiles)
		},
	}
	cmd.Flags().StringVarP(&statePath, "state", "s", "state.json", "availability ledger file")
	cmd.Flags().BoolVar(&markdown, "markdown", false, "render as markdown (not implemented yet)")
	cmd.Flags().BoolVar(&showFiles, "files", false, "also list every observed file per registry")
	return cmd
}

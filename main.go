// Command weightcanary watches whether the open-weight models a product
// depends on are still there: file presence on Hugging Face, mirror sync on
// ModelScope/Gitee, and what exactly changed — recorded in a commit-friendly
// state.json ledger.
package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/SuperMarioYL/weightcanary/cmd"
)

const version = "0.1.0"

func main() {
	root := &cobra.Command{
		Use:     "weightcanary",
		Version: version,
		Short:   "Availability-risk monitor for the open-weight models you depend on",
		Long: `weightcanary watches the open-weight models a product depends on:
whether the weights are still listed on Hugging Face, whether ModelScope/Gitee
mirrors have diverged from the source, and what exactly changed — recorded in
a commit-friendly state.json ledger.`,
		SilenceUsage: true,
	}
	root.AddCommand(
		cmd.NewInitCmd(),
		cmd.NewWatchCmd(),
		cmd.NewReportCmd(),
	)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

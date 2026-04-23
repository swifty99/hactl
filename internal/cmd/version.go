package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// Set via -ldflags at build time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print hactl version",
	Run: func(cmd *cobra.Command, args []string) {
		printVersion(cmd.OutOrStdout())
	},
}

func printVersion(w io.Writer) {
	_, _ = fmt.Fprintf(w, "hactl %s (commit %s, built %s)\n", version, commit, date)
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

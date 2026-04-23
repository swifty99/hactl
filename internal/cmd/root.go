package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	flagDir   string
	flagSince string
	flagTop   int
	flagFull  bool
	flagJSON  bool
	flagColor bool
	flagStats bool
)

var rootCmd = &cobra.Command{
	Use:   "hactl",
	Short: "CLI for Home Assistant analysis & development",
	Long:  "hactl – LLM-friendly CLI for Home Assistant analysis, debugging, and controlled automation management.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagDir, "dir", "", "instance directory (overrides HACTL_DIR and auto-discovery)")
	rootCmd.PersistentFlags().StringVar(&flagSince, "since", "24h", "time range for queries (e.g. 24h, 7d)")
	rootCmd.PersistentFlags().IntVar(&flagTop, "top", 10, "max items to display")
	rootCmd.PersistentFlags().BoolVar(&flagFull, "full", false, "show full/raw output")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "output as JSON")
	rootCmd.PersistentFlags().BoolVar(&flagColor, "color", false, "enable colored output")
	rootCmd.PersistentFlags().BoolVar(&flagStats, "stats", false, "show response size and estimated token count")
}

// statsWriter wraps an io.Writer and counts bytes written.
type statsWriter struct {
	inner io.Writer
	bytes int64
}

func (sw *statsWriter) Write(p []byte) (int, error) {
	n, err := sw.inner.Write(p)
	sw.bytes += int64(n)
	return n, err
}

// estimateTokens estimates token count from byte count.
// Approximation: ~4 characters per token for English text.
func estimateTokens(bytes int64) int64 {
	return (bytes + 3) / 4
}

// writeStats writes the stats footer to the given writer.
func writeStats(w io.Writer, byteCount int64) {
	tokens := estimateTokens(byteCount)
	_, _ = fmt.Fprintf(w, "---\nstats: %d bytes, ~%d tokens\n", byteCount, tokens)
}

// Execute runs the root command.
func Execute() error {
	sw := &statsWriter{inner: os.Stdout}
	rootCmd.SetOut(sw)
	defer rootCmd.SetOut(nil)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}

	if flagStats {
		writeStats(os.Stderr, sw.bytes)
	}
	return nil
}

// RunWithOutput executes the command with the given args and captures output to w.
// Used by integration tests to run hactl commands programmatically.
func RunWithOutput(args []string, w io.Writer) error {
	sw := &statsWriter{inner: w}
	rootCmd.SetOut(sw)
	rootCmd.SetArgs(args[1:]) // skip "hactl" binary name
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetArgs(nil)
		// Reset flags to defaults for next invocation
		flagDir = ""
		flagSince = "24h"
		flagTop = 10
		flagFull = false
		flagJSON = false
		flagColor = false
		flagStats = false
		resetSubcommandFlags()
	}()

	err := rootCmd.Execute()

	if flagStats {
		writeStats(w, sw.bytes)
	}

	return err
}

// resetSubcommandFlags resets all subcommand-specific flags to their defaults.
// This prevents flag value leakage between consecutive RunWithOutput calls in tests.
func resetSubcommandFlags() {
	flagAutoFailing = false
	flagAutoPattern = ""
	flagAutoTag = ""
	flagAutoFile = ""
	flagAutoConfirm = false
	flagTplFile = ""
	flagEntPattern = ""
	flagEntDomain = ""
	flagEntResample = ""
	flagEntAttr = ""
	flagEntArea = ""
	flagEntLabel = ""
	flagCCLogsUnique = false
	flagSvcData = "{}"
	flagScriptPattern = ""
	flagLabelColor = ""
	flagLabelIcon = ""
	flagLabelDesc = ""
	flagDashView = ""
	flagDashRaw = false
	flagDashFile = ""
	flagDashConfirm = false
	flagDashTitle = ""
	flagDashURLPath = ""
	flagDashIcon = ""
	flagDashSidebar = true
	flagDashAdmin = false
	// Reset all cobra internal flags (including --help) on every command
	// to prevent stale flag state between repeated Execute() calls.
	resetCobraFlags(rootCmd)
}

// resetCobraFlags recursively resets all flags on a command and its children
// back to their default values. This is critical for cobra's built-in --help
// flag which, once set to true, causes all subsequent calls to print help.
func resetCobraFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
	for _, sub := range cmd.Commands() {
		resetCobraFlags(sub)
	}
}

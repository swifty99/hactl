package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/swifty99/hactl/internal/config"
	"github.com/swifty99/hactl/internal/haapi"
)

var flagTplFile string

var tplCmd = &cobra.Command{
	Use:   "tpl",
	Short: "Evaluate Jinja2 templates via HA",
	Long:  "Render Home Assistant Jinja2 templates using the HA template API.",
}

var tplEvalCmd = &cobra.Command{
	Use:   "eval [template]",
	Short: "Evaluate a template",
	Long:  "Evaluate an inline template string or a template from a file (-f).",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTplEval(cmd.Context(), cmd.OutOrStdout(), args)
	},
}

func init() {
	tplEvalCmd.Flags().StringVarP(&flagTplFile, "file", "f", "", "read template from file")
	tplCmd.AddCommand(tplEvalCmd)
	rootCmd.AddCommand(tplCmd)
}

func runTplEval(ctx context.Context, w io.Writer, args []string) error {
	tpl, err := resolveTemplate(args)
	if err != nil {
		return err
	}

	cfg, err := config.Load(flagDir)
	if err != nil {
		return err
	}

	client := haapi.New(cfg.URL, cfg.Token)
	result, err := client.RenderTemplate(ctx, tpl)
	if err != nil {
		return fmt.Errorf("rendering template: %w", err)
	}

	_, _ = fmt.Fprintln(w, result)
	return nil
}

func resolveTemplate(args []string) (string, error) {
	if flagTplFile != "" {
		data, err := os.ReadFile(flagTplFile) //nolint:gosec // file path provided by user via CLI flag
		if err != nil {
			return "", fmt.Errorf("reading template file: %w", err)
		}
		return string(data), nil
	}
	if len(args) == 0 {
		return "", errors.New("provide a template string or use -f <file>")
	}
	return args[0], nil
}

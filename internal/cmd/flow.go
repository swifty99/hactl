package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/swifty99/hactl/internal/config"
	"github.com/swifty99/hactl/internal/format"
	"github.com/swifty99/hactl/internal/haapi"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage config entries and flows",
	Long:  "Start, step through, and inspect config entry options flows and config flows.",
}

var configOptionsCmd = &cobra.Command{
	Use:   "options <entry_id>",
	Short: "Start an options flow for a config entry",
	Long:  "Start an options flow for an existing config entry. Returns the flow ID and initial step schema.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfigOptions(cmd.Context(), cmd.OutOrStdout(), args[0])
	},
}

var configFlowStartCmd = &cobra.Command{
	Use:   "flow-start <domain>",
	Short: "Start a config flow for an integration",
	Long:  "Start a new config flow for a domain/integration. Returns the flow ID and initial step schema.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfigFlowStart(cmd.Context(), cmd.OutOrStdout(), args[0])
	},
}

var flagFlowData string
var flagFlowOptions bool

var configFlowStepCmd = &cobra.Command{
	Use:   "flow-step <flow_id>",
	Short: "Submit data to advance a flow",
	Long:  "Submit data to advance a config/options flow to the next step. Use --options for options flows. Returns the next step or completion result.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfigFlowStep(cmd.Context(), cmd.OutOrStdout(), args[0])
	},
}

var configFlowInspectCmd = &cobra.Command{
	Use:   "flow-inspect <flow_id>",
	Short: "Inspect current flow state",
	Long:  "Show the current step, expected schema fields, and any errors for a flow. Use --options for options flows.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfigFlowInspect(cmd.Context(), cmd.OutOrStdout(), args[0])
	},
}

func init() {
	configFlowStepCmd.Flags().StringVar(&flagFlowData, "data", "{}", "JSON data to submit to the flow step")
	configFlowStepCmd.Flags().BoolVar(&flagFlowOptions, "options", false, "use options flow endpoint (for existing config entries)")
	configFlowInspectCmd.Flags().BoolVar(&flagFlowOptions, "options", false, "use options flow endpoint (for existing config entries)")
	configCmd.AddCommand(configOptionsCmd, configFlowStartCmd, configFlowStepCmd, configFlowInspectCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigOptions(ctx context.Context, w io.Writer, entryID string) error {
	cfg, err := config.Load(flagDir)
	if err != nil {
		return err
	}
	client := haapi.New(cfg.URL, cfg.Token)
	data, err := client.StartOptionsFlow(ctx, entryID)
	if err != nil {
		return fmt.Errorf("starting options flow: %w", err)
	}
	return renderFlowResult(w, data)
}

func runConfigFlowStart(ctx context.Context, w io.Writer, domain string) error {
	cfg, err := config.Load(flagDir)
	if err != nil {
		return err
	}
	client := haapi.New(cfg.URL, cfg.Token)
	data, err := client.StartConfigFlow(ctx, domain)
	if err != nil {
		return fmt.Errorf("starting config flow: %w", err)
	}
	return renderFlowResult(w, data)
}

func runConfigFlowStep(ctx context.Context, w io.Writer, flowID string) error {
	cfg, err := config.Load(flagDir)
	if err != nil {
		return err
	}
	client := haapi.New(cfg.URL, cfg.Token)

	var rawData json.RawMessage
	if jsonErr := json.Unmarshal([]byte(flagFlowData), &rawData); jsonErr != nil {
		return fmt.Errorf("invalid --data JSON: %w", jsonErr)
	}

	data, err := client.StepFlow(ctx, flowID, flagFlowOptions, rawData)
	if err != nil {
		return fmt.Errorf("stepping flow: %w", err)
	}
	return renderFlowResult(w, data)
}

func runConfigFlowInspect(ctx context.Context, w io.Writer, flowID string) error {
	cfg, err := config.Load(flagDir)
	if err != nil {
		return err
	}
	client := haapi.New(cfg.URL, cfg.Token)
	data, err := client.InspectFlow(ctx, flowID, flagFlowOptions)
	if err != nil {
		return fmt.Errorf("inspecting flow: %w", err)
	}
	return renderFlowResult(w, data)
}

func renderFlowResult(w io.Writer, data []byte) error {
	if flagJSON {
		_, err := w.Write(data)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w)
		return err
	}

	flow, err := haapi.ParseFlowResult(data)
	if err != nil {
		return err
	}

	// Header info
	_, _ = fmt.Fprintf(w, "flow_id: %s\n", flow.FlowID)
	_, _ = fmt.Fprintf(w, "type:    %s\n", flow.Type)
	_, _ = fmt.Fprintf(w, "step:    %s\n", flow.StepID)
	if flow.Handler != "" {
		_, _ = fmt.Fprintf(w, "handler: %s\n", flow.Handler)
	}
	if flow.Title != "" {
		_, _ = fmt.Fprintf(w, "title:   %s\n", flow.Title)
	}

	// Errors
	if len(flow.Errors) > 0 {
		_, _ = fmt.Fprintf(w, "\nErrors:\n")
		for field, msg := range flow.Errors {
			_, _ = fmt.Fprintf(w, "  %s: %s\n", field, msg)
		}
	}

	// Schema fields table
	if len(flow.DataSchema) > 0 {
		_, _ = fmt.Fprintf(w, "\n")
		tbl := &format.Table{
			Headers: []string{"Field", "Type", "Required", "Default"},
		}
		for _, f := range flow.DataSchema {
			req := "no"
			if f.Required {
				req = "yes"
			}
			def := ""
			if f.Default != nil {
				def = fmt.Sprintf("%v", f.Default)
			}
			typ := f.Type
			if typ == "" {
				typ = "string"
			}
			tbl.Rows = append(tbl.Rows, []string{f.Name, typ, req, def})
		}
		return tbl.Render(w, format.RenderOpts{Full: true})
	}

	// Result payload for create_entry / abort
	if flow.Type == "create_entry" || flow.Type == "abort" {
		if len(flow.Result) > 0 {
			_, _ = fmt.Fprintf(w, "\nResult: %s\n", string(flow.Result))
		}
	}

	return nil
}

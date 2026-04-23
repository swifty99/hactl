package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/swifty99/hactl/internal/analyze"
	"github.com/swifty99/hactl/internal/config"
	"github.com/swifty99/hactl/internal/format"
	"github.com/swifty99/hactl/internal/haapi"
)

var ccCmd = &cobra.Command{
	Use:   "cc",
	Short: "Inspect custom components",
	Long:  "List and inspect custom (third-party) components installed in HA.",
}

var ccLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List custom components",
	Long:  "Show installed custom components with version and domain.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCCLs(cmd.Context(), cmd.OutOrStdout())
	},
}

var ccShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show custom component details",
	Long:  "Display details for a specific custom component.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCCShow(cmd.Context(), cmd.OutOrStdout(), args[0])
	},
}

var ccLogsCmd = &cobra.Command{
	Use:   "logs <name>",
	Short: "Show logs for a custom component",
	Long:  "Display error log entries related to a specific custom component.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCCLogs(cmd.Context(), cmd.OutOrStdout(), args[0])
	},
}

var flagCCLogsUnique bool

func init() {
	ccLogsCmd.Flags().BoolVar(&flagCCLogsUnique, "unique", false, "deduplicate identical log messages")
	ccCmd.AddCommand(ccLsCmd, ccShowCmd, ccLogsCmd)
	rootCmd.AddCommand(ccCmd)
}

// ccInfo holds info about a custom component.
type ccInfo struct {
	Domain       string
	Version      string
	Requirements []string
}

func runCCLs(ctx context.Context, w io.Writer) error {
	cfg, err := config.Load(flagDir)
	if err != nil {
		return err
	}

	client := haapi.New(cfg.URL, cfg.Token)
	components, err := fetchCustomComponents(ctx, client)
	if err != nil {
		return err
	}

	if len(components) == 0 {
		_, _ = fmt.Fprintln(w, "no custom components found")
		return nil
	}

	tbl := &format.Table{
		Headers: []string{"domain", "version"},
		Rows:    make([][]string, len(components)),
	}
	for i, cc := range components {
		tbl.Rows[i] = []string{
			cc.Domain,
			cc.Version,
		}
	}

	return tbl.Render(w, format.RenderOpts{
		Top:  flagTop,
		Full: flagFull,
		JSON: flagJSON,
	})
}

func runCCShow(ctx context.Context, w io.Writer, name string) error {
	cfg, err := config.Load(flagDir)
	if err != nil {
		return err
	}

	client := haapi.New(cfg.URL, cfg.Token)
	components, err := fetchCustomComponents(ctx, client)
	if err != nil {
		return err
	}

	var found *ccInfo
	for i, cc := range components {
		if cc.Domain == name {
			found = &components[i]
			break
		}
	}

	if found == nil {
		return fmt.Errorf("custom component %q not found", name)
	}

	_, _ = fmt.Fprintf(w, "domain:  %s\n", found.Domain)
	_, _ = fmt.Fprintf(w, "version: %s\n", found.Version)

	// Show related entities
	states, statesErr := client.GetStates(ctx)
	if statesErr == nil {
		var allStates []entityState
		if jsonErr := json.Unmarshal(states, &allStates); jsonErr == nil {
			count := 0
			for _, s := range allStates {
				if strings.HasPrefix(s.EntityID, found.Domain+".") {
					count++
				}
			}
			if count > 0 {
				_, _ = fmt.Fprintf(w, "entities: %d\n", count)
			}
		}
	}

	return nil
}

func runCCLogs(ctx context.Context, w io.Writer, name string) error {
	cfg, err := config.Load(flagDir)
	if err != nil {
		return err
	}

	entries, err := fetchLogEntries(ctx, cfg)
	if err != nil {
		return fmt.Errorf("fetching logs: %w", err)
	}

	entries = analyze.FilterByComponent(entries, name)

	if len(entries) == 0 {
		_, _ = fmt.Fprintf(w, "no log entries for %s\n", name)
		return nil
	}

	if flagCCLogsUnique {
		return renderDedupedLogs(w, entries)
	}

	return renderLogEntriesSimple(w, entries)
}

func renderLogEntriesSimple(w io.Writer, entries []analyze.LogEntry) error {
	tbl := &format.Table{
		Headers: []string{"time", "level", "component", "message"},
		Rows:    make([][]string, len(entries)),
	}
	for i, e := range entries {
		msg := e.Message
		if len(msg) > 60 {
			msg = msg[:57] + "..."
		}
		tbl.Rows[i] = []string{
			analyze.FormatShortTimestamp(e.Timestamp),
			e.Level,
			e.Component,
			msg,
		}
	}

	return tbl.Render(w, format.RenderOpts{
		Top:  flagTop,
		Full: flagFull,
		JSON: flagJSON,
	})
}

// fetchCustomComponents gets custom_components from HA config.
func fetchCustomComponents(ctx context.Context, client *haapi.Client) ([]ccInfo, error) {
	configData, err := client.GetConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching HA config: %w", err)
	}

	var haCfg struct {
		Components []string `json:"components"`
	}
	if unmarshalErr := json.Unmarshal(configData, &haCfg); unmarshalErr != nil {
		return nil, fmt.Errorf("parsing HA config: %w", unmarshalErr)
	}

	// Custom components are typically in the components list but are third-party.
	// HA doesn't directly expose "custom" vs "built-in" via config API.
	// We identify custom components by looking for custom_components entries
	// in the states. Components that are from HACS typically have version attributes.
	statesData, err := client.GetStates(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching states: %w", err)
	}

	var states []struct {
		Attributes map[string]any `json:"attributes"`
		EntityID   string         `json:"entity_id"`
		State      string         `json:"state"`
	}
	if err := json.Unmarshal(statesData, &states); err != nil {
		return nil, fmt.Errorf("parsing states: %w", err)
	}

	// Look for update.* entities which track component versions (HACS pattern)
	var components []ccInfo
	seen := make(map[string]bool)
	for _, s := range states {
		if !strings.HasPrefix(s.EntityID, "update.") {
			continue
		}
		title, _ := s.Attributes["title"].(string)
		installedVersion, _ := s.Attributes["installed_version"].(string)
		if title == "" || installedVersion == "" {
			continue
		}
		domain := strings.TrimPrefix(s.EntityID, "update.")
		// Deduplicate
		if seen[domain] {
			continue
		}
		seen[domain] = true
		components = append(components, ccInfo{
			Domain:  domain,
			Version: installedVersion,
		})
	}

	return components, nil
}

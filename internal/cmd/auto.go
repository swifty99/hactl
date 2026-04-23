package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/swifty99/hactl/internal/config"
	"github.com/swifty99/hactl/internal/format"
	"github.com/swifty99/hactl/internal/haapi"
	"github.com/swifty99/hactl/internal/writer"
	"github.com/swifty99/hactl/pkg/ids"
)

var flagAutoFailing bool
var flagAutoPattern string
var flagAutoTag string
var flagAutoFile string
var flagAutoConfirm bool

var autoCmd = &cobra.Command{
	Use:   "auto",
	Short: "Manage and inspect automations",
	Long:  "List, filter, inspect, diff, and apply Home Assistant automations.",
}

var autoLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List automations",
	Long:  "Show automations table with state, run counts, and error info.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAutoLs(cmd.Context(), cmd.OutOrStdout())
	},
}

var autoShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show automation details and recent traces",
	Long:  "Display automation summary and the last 5 trace runs.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAutoShow(cmd.Context(), cmd.OutOrStdout(), args[0])
	},
}

var autoDiffCmd = &cobra.Command{
	Use:   "diff <id>",
	Short: "Show diff between local YAML and remote automation config",
	Long:  "Compare a local YAML file (-f) against the current HA automation config.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAutoDiff(cmd.Context(), cmd.OutOrStdout(), args[0])
	},
}

var autoApplyCmd = &cobra.Command{
	Use:   "apply <id>",
	Short: "Apply a local YAML config to HA (dry-run by default)",
	Long:  "Validate and write automation config. Use --confirm to actually write + reload.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAutoApply(cmd.Context(), cmd.OutOrStdout(), args[0])
	},
}

func init() {
	autoLsCmd.Flags().BoolVar(&flagAutoFailing, "failing", false, "show only automations with recent errors")
	autoLsCmd.Flags().StringVar(&flagAutoPattern, "pattern", "", "glob pattern to filter automations (e.g. ess_*)")
	autoLsCmd.Flags().StringVar(&flagAutoTag, "tag", "", "filter automations by label/tag (e.g. ess)")
	autoDiffCmd.Flags().StringVarP(&flagAutoFile, "file", "f", "", "local YAML file to diff/apply")
	autoApplyCmd.Flags().StringVarP(&flagAutoFile, "file", "f", "", "local YAML file to apply")
	autoApplyCmd.Flags().BoolVar(&flagAutoConfirm, "confirm", false, "actually write + reload (default is dry-run)")
	autoCmd.AddCommand(autoLsCmd, autoShowCmd, autoDiffCmd, autoApplyCmd)
	rootCmd.AddCommand(autoCmd)
}

// automationEntity is an automation from /api/states.
type automationEntity struct {
	EntityID   string               `json:"entity_id"`
	State      string               `json:"state"`
	Attributes automationAttributes `json:"attributes"`
}

type automationAttributes struct {
	FriendlyName  string   `json:"friendly_name"`
	LastTriggered string   `json:"last_triggered"`
	ID            string   `json:"id"`
	Mode          string   `json:"mode"`
	Labels        []string `json:"labels"`
	Current       int      `json:"current"`
}

// autoRow holds combined state+trace data for one automation.
type autoRow struct {
	id      string
	state   string
	lastErr string
	area    string
	traces  []haapi.TraceSummary
	labels  []string
	runs    int
	errors  int
}

func runAutoLs(ctx context.Context, w io.Writer) error {
	cfg, err := config.Load(flagDir)
	if err != nil {
		return err
	}

	client := haapi.New(cfg.URL, cfg.Token)

	// Fetch all states and filter automations
	autos, err := fetchAutomations(ctx, client)
	if err != nil {
		return err
	}

	// Fetch traces via WebSocket
	traces, wsErr := fetchTraceList(ctx, cfg)
	if wsErr != nil {
		slog.Warn("could not fetch traces, showing basic info only", "error", wsErr)
	}

	// Fetch registry for area/label enrichment
	var rc *registryContext
	ws := haapi.NewWSClient(cfg.URL, cfg.Token)
	if connErr := ws.Connect(ctx); connErr != nil {
		slog.Warn("could not fetch registry context", "error", connErr)
	} else {
		defer func() { _ = ws.Close() }()
		rc, _ = fetchRegistryContext(ctx, ws)
	}

	sinceDur, err := parseSince(flagSince)
	if err != nil {
		return err
	}
	cutoff := time.Now().Add(-sinceDur)

	rows := buildAutoRows(autos, traces, cutoff)

	// Enrich with area from registry
	if rc != nil {
		for i := range rows {
			rows[i].area = rc.areaName("automation." + rows[i].id)
		}
	}

	if flagAutoPattern != "" {
		rows = filterAutosByPattern(rows, flagAutoPattern)
	}

	if flagAutoTag != "" {
		rows = filterAutosByTag(rows, flagAutoTag)
	}

	if flagAutoFailing {
		rows = filterFailing(rows)
	}

	tbl := &format.Table{
		Headers: []string{"id", "state", "area", "labels", "runs_24h", "errors", "last_err"},
		Rows:    make([][]string, len(rows)),
	}
	for i, r := range rows {
		tbl.Rows[i] = []string{
			r.id,
			r.state,
			r.area,
			strings.Join(r.labels, ", "),
			strconv.Itoa(r.runs),
			strconv.Itoa(r.errors),
			r.lastErr,
		}
	}

	return tbl.Render(w, format.RenderOpts{
		Top:     flagTop,
		Full:    flagFull,
		JSON:    flagJSON,
		Compact: true,
	})
}

func runAutoShow(ctx context.Context, w io.Writer, autoID string) error {
	cfg, err := config.Load(flagDir)
	if err != nil {
		return err
	}

	client := haapi.New(cfg.URL, cfg.Token)

	// Resolve the entity
	entityID := autoID
	if !strings.HasPrefix(entityID, "automation.") {
		entityID = "automation." + autoID
	}

	stateData, err := client.GetState(ctx, entityID)
	if err != nil {
		return fmt.Errorf("fetching automation state: %w", err)
	}
	var ent automationEntity
	if err := json.Unmarshal(stateData, &ent); err != nil {
		return fmt.Errorf("parsing automation state: %w", err)
	}

	// Summary line
	_, _ = fmt.Fprintf(w, "%s  state=%s  mode=%s  last_triggered=%s\n",
		ent.EntityID, ent.State,
		ent.Attributes.Mode,
		formatShortTime(ent.Attributes.LastTriggered))

	// Fetch traces
	traces, wsErr := fetchTraceList(ctx, cfg)
	if wsErr != nil {
		_, _ = fmt.Fprintf(w, "traces: unavailable (%v)\n", wsErr)
		return nil
	}

	key := entityID
	autoTraces := traces[key]

	if len(autoTraces) == 0 {
		_, _ = fmt.Fprintln(w, "traces: none")
		return nil
	}

	// Setup IDs registry
	idsPath := filepath.Join(cfg.Dir, "cache", "ids.json")
	reg := ids.NewRegistry(idsPath)
	if loadErr := reg.Load(); loadErr != nil {
		slog.Warn("could not load ids registry", "error", loadErr)
	}

	// Show last 5 traces
	limit := min(5, len(autoTraces))
	recent := autoTraces[:limit]

	_, _ = fmt.Fprintf(w, "traces (last %d):\n", limit)

	tbl := &format.Table{
		Headers: []string{"id", "time", "result", "last_step"},
		Rows:    make([][]string, len(recent)),
	}
	for i, tr := range recent {
		traceKey := tr.Domain + "." + tr.ItemID + "/" + tr.RunID
		shortID := reg.GetOrCreate("trc", traceKey)

		tbl.Rows[i] = []string{
			shortID,
			formatShortTime(tr.Timestamp.Start),
			traceResult(tr),
			tr.LastStep,
		}
	}

	if saveErr := reg.Save(); saveErr != nil {
		slog.Warn("could not save ids registry", "error", saveErr)
	}

	return tbl.Render(w, format.RenderOpts{Full: true})
}

func fetchAutomations(ctx context.Context, client *haapi.Client) ([]automationEntity, error) {
	data, err := client.GetStates(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching states: %w", err)
	}

	var allStates []automationEntity
	if err := json.Unmarshal(data, &allStates); err != nil {
		return nil, fmt.Errorf("parsing states: %w", err)
	}

	autos := make([]automationEntity, 0, len(allStates))
	for _, s := range allStates {
		if strings.HasPrefix(s.EntityID, "automation.") {
			autos = append(autos, s)
		}
	}
	return autos, nil
}

func fetchTraceList(ctx context.Context, cfg *config.Config) (haapi.TraceListResult, error) {
	ws := haapi.NewWSClient(cfg.URL, cfg.Token)
	if err := ws.Connect(ctx); err != nil {
		return nil, fmt.Errorf("websocket connect: %w", err)
	}
	defer func() { _ = ws.Close() }()

	result, err := ws.TraceList(ctx, "automation")
	if err != nil {
		return nil, fmt.Errorf("fetching traces: %w", err)
	}
	return result, nil
}

func buildAutoRows(autos []automationEntity, traces haapi.TraceListResult, cutoff time.Time) []autoRow {
	rows := make([]autoRow, 0, len(autos))
	for _, a := range autos {
		// Use entity_id suffix as the display ID â€” this is what auto show/diff/apply accept.
		id := strings.TrimPrefix(a.EntityID, "automation.")

		row := autoRow{
			id:     id,
			state:  a.State,
			labels: a.Attributes.Labels,
		}

		key := a.EntityID
		if ts, ok := traces[key]; ok {
			row.traces = ts
			for _, tr := range ts {
				t, err := time.Parse(time.RFC3339Nano, tr.Timestamp.Start)
				if err != nil {
					continue
				}
				if t.After(cutoff) {
					row.runs++
					if isTraceError(tr) {
						row.errors++
						if row.lastErr == "" {
							row.lastErr = formatShortTime(tr.Timestamp.Start) + " " + shortenStep(tr.LastStep)
						}
					}
				}
			}
		}

		rows = append(rows, row)
	}
	return rows
}

func filterAutosByPattern(rows []autoRow, pattern string) []autoRow {
	result := make([]autoRow, 0, len(rows))
	for _, r := range rows {
		if matchPattern(r.id, pattern) || matchPattern("automation."+r.id, pattern) {
			result = append(result, r)
		}
	}
	return result
}

func filterAutosByTag(rows []autoRow, tag string) []autoRow {
	result := make([]autoRow, 0, len(rows))
	for _, r := range rows {
		for _, l := range r.labels {
			if strings.EqualFold(l, tag) || strings.Contains(strings.ToLower(l), strings.ToLower(tag)) {
				result = append(result, r)
				break
			}
		}
	}
	return result
}

func filterFailing(rows []autoRow) []autoRow {
	result := make([]autoRow, 0, len(rows))
	for _, r := range rows {
		if r.errors > 0 {
			result = append(result, r)
		}
	}
	return result
}

func isTraceError(tr haapi.TraceSummary) bool {
	return tr.Execution == "error" || tr.Error != ""
}

func traceResult(tr haapi.TraceSummary) string {
	if isTraceError(tr) {
		return "error"
	}
	if tr.Execution == "" {
		return tr.State
	}
	return tr.Execution
}

func shortenStep(step string) string {
	if step == "" {
		return ""
	}
	parts := strings.Split(step, "/")
	if len(parts) >= 2 {
		return parts[0]
	}
	return step
}

func formatShortTime(isoTime string) string {
	if isoTime == "" {
		return "-"
	}
	t, err := time.Parse(time.RFC3339Nano, isoTime)
	if err != nil {
		// Try without nanoseconds
		t, err = time.Parse(time.RFC3339, isoTime)
		if err != nil {
			return isoTime
		}
	}
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	return t.Format("01-02 15:04")
}

// parseSince converts a duration string like "24h" or "7d" to time.Duration.
func parseSince(s string) (time.Duration, error) {
	if after, found := strings.CutSuffix(s, "d"); found {
		days, err := strconv.Atoi(after)
		if err != nil {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}
	return d, nil
}

func runAutoDiff(ctx context.Context, w io.Writer, autoID string) error {
	if flagAutoFile == "" {
		return errors.New("--file / -f is required for diff")
	}

	cfg, err := config.Load(flagDir)
	if err != nil {
		return err
	}

	client := haapi.New(cfg.URL, cfg.Token)
	backupDir := filepath.Join(cfg.Dir, "backups")
	wr := writer.New(client, nil, backupDir)

	diff, err := wr.Diff(ctx, autoID, flagAutoFile)
	if err != nil {
		return err
	}

	if !diff.HasChanges {
		_, _ = fmt.Fprintf(w, "%s: no changes\n", autoID)
		return nil
	}

	_, _ = fmt.Fprintf(w, "%s: diff\n", autoID)
	for _, line := range diff.Lines {
		_, _ = fmt.Fprintln(w, line)
	}
	return nil
}

func runAutoApply(ctx context.Context, w io.Writer, autoID string) error {
	if flagAutoFile == "" {
		return errors.New("--file / -f is required for apply")
	}

	cfg, err := config.Load(flagDir)
	if err != nil {
		return err
	}

	client := haapi.New(cfg.URL, cfg.Token)
	backupDir := filepath.Join(cfg.Dir, "backups")

	// Connect WebSocket for validation
	var wsClient *haapi.WSClient
	ws := haapi.NewWSClient(cfg.URL, cfg.Token)
	if connectErr := ws.Connect(ctx); connectErr != nil {
		slog.Warn("could not connect WebSocket for validation", "error", connectErr)
	} else {
		wsClient = ws
		defer func() { _ = ws.Close() }()
	}

	wr := writer.New(client, wsClient, backupDir)

	// Show diff first
	diff, diffErr := wr.Diff(ctx, autoID, flagAutoFile)
	switch {
	case diffErr != nil:
		slog.Warn("could not generate diff", "error", diffErr)
	case diff.HasChanges:
		_, _ = fmt.Fprintf(w, "diff:\n")
		for _, line := range diff.Lines {
			_, _ = fmt.Fprintln(w, line)
		}
	default:
		_, _ = fmt.Fprintf(w, "no changes detected\n")
		return nil
	}

	result, err := wr.Apply(ctx, autoID, flagAutoFile, flagAutoConfirm)
	if err != nil {
		return err
	}

	if result.DryRun {
		_, _ = fmt.Fprintf(w, "\ndry-run: no changes written (use --confirm to apply)\n")
		if result.BackupPath != "" {
			_, _ = fmt.Fprintf(w, "backup: %s\n", result.BackupPath)
		}
		return nil
	}

	_, _ = fmt.Fprintf(w, "\napplied: %s\n", autoID)
	if result.BackupPath != "" {
		_, _ = fmt.Fprintf(w, "backup:  %s\n", result.BackupPath)
	}
	if result.Reloaded {
		_, _ = fmt.Fprintf(w, "reload:  ok\n")
	}
	return nil
}

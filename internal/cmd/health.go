package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/swifty99/hactl/internal/config"
	"github.com/swifty99/hactl/internal/haapi"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Show Home Assistant health overview",
	Long:  "Display HA version, recorder status, and error count.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHealth(cmd.Context(), cmd.OutOrStdout())
	},
}

func init() {
	rootCmd.AddCommand(healthCmd)
}

// healthResult holds structured health data for JSON output.
type healthResult struct {
	Version        string `json:"version"`
	State          string `json:"state"`
	RecorderStatus string `json:"recorder"`
	LocationName   string `json:"location"`
	TimeZone       string `json:"timezone"`
	ErrorCount     int    `json:"errors"`
	SafeMode       bool   `json:"safe_mode,omitempty"`
}

// haConfig holds the subset of /api/config we care about.
type haConfig struct {
	UnitSystem      any      `json:"unit_system"`
	Version         string   `json:"version"`
	LocationName    string   `json:"location_name"`
	State           string   `json:"state"`
	ExternalURL     string   `json:"external_url"`
	InternalURL     string   `json:"internal_url"`
	Currency        string   `json:"currency"`
	TimeZone        string   `json:"time_zone"`
	ConfigDir       string   `json:"config_dir"`
	Components      []string `json:"components"`
	AllowlistExtURL []string `json:"allowlist_external_urls"`
	SafeMode        bool     `json:"safe_mode"`
}

func runHealth(ctx context.Context, w io.Writer) error {
	cfg, err := config.Load(flagDir)
	if err != nil {
		return err
	}

	client := haapi.New(cfg.URL, cfg.Token)

	// Fetch config (version, state, components)
	configData, err := client.GetConfig(ctx)
	if err != nil {
		return fmt.Errorf("fetching HA config: %w", err)
	}

	var haCfg haConfig
	if unmarshalErr := json.Unmarshal(configData, &haCfg); unmarshalErr != nil {
		return fmt.Errorf("parsing HA config: %w", unmarshalErr)
	}

	// Check recorder
	recorderStatus := "not loaded"
	if slices.Contains(haCfg.Components, "recorder") {
		recorderStatus = "ok"
	}

	// Fetch error log and count lines (non-fatal: some HA setups disable system_log)
	errorCount := -1
	errLog, err := client.GetErrorLog(ctx)
	if err != nil {
		slog.Warn("could not fetch error log", "error", err)
	} else {
		errorCount = countErrorLines(string(errLog))
	}

	// Output
	hr := healthResult{
		Version:        haCfg.Version,
		State:          haCfg.State,
		RecorderStatus: recorderStatus,
		ErrorCount:     errorCount,
		LocationName:   haCfg.LocationName,
		TimeZone:       haCfg.TimeZone,
		SafeMode:       haCfg.SafeMode,
	}

	if flagJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(hr)
	}

	if errorCount >= 0 {
		_, _ = fmt.Fprintf(w, "HA %s  state=%s  recorder=%s  errors=%d\n", haCfg.Version, haCfg.State, recorderStatus, errorCount)
	} else {
		_, _ = fmt.Fprintf(w, "HA %s  state=%s  recorder=%s  errors=n/a\n", haCfg.Version, haCfg.State, recorderStatus)
	}
	_, _ = fmt.Fprintf(w, "location=%s  tz=%s\n", haCfg.LocationName, haCfg.TimeZone)
	if haCfg.SafeMode {
		_, _ = fmt.Fprintf(w, "âš  SAFE MODE ACTIVE\n")
	}

	return nil
}

// countErrorLines counts lines containing ERROR in the HA error log.
func countErrorLines(log string) int {
	count := 0
	for line := range strings.SplitSeq(log, "\n") {
		if strings.Contains(line, "ERROR") {
			count++
		}
	}
	return count
}

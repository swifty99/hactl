package cmd

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/swifty99/hactl/internal/config"
	"github.com/swifty99/hactl/internal/format"
	"github.com/swifty99/hactl/internal/haapi"
)

var floorCmd = &cobra.Command{
	Use:   "floor",
	Short: "Discover floors",
	Long:  "List Home Assistant floors.",
}

var floorLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all floors",
	Long:  "Show all floors registered in Home Assistant.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runFloorLs(cmd.Context(), cmd.OutOrStdout())
	},
}

func init() {
	floorCmd.AddCommand(floorLsCmd)
	rootCmd.AddCommand(floorCmd)
}

func runFloorLs(ctx context.Context, w io.Writer) error {
	cfg, err := config.Load(flagDir)
	if err != nil {
		return err
	}

	ws := haapi.NewWSClient(cfg.URL, cfg.Token)
	if connErr := ws.Connect(ctx); connErr != nil {
		return fmt.Errorf("connecting to HA: %w", connErr)
	}
	defer func() { _ = ws.Close() }()

	floors, err := ws.FloorRegistryList(ctx)
	if err != nil {
		return fmt.Errorf("fetching floors: %w", err)
	}

	if len(floors) == 0 {
		_, _ = fmt.Fprintln(w, "no floors")
		return nil
	}

	tbl := &format.Table{
		Headers: []string{"floor_id", "name", "level", "icon"},
		Rows:    make([][]string, len(floors)),
	}
	for i, f := range floors {
		levelStr := ""
		if f.Level != nil {
			levelStr = strconv.Itoa(*f.Level)
		}
		tbl.Rows[i] = []string{
			f.FloorID,
			f.Name,
			levelStr,
			f.Icon,
		}
	}

	return tbl.Render(w, format.RenderOpts{
		Top:     flagTop,
		Full:    flagFull,
		JSON:    flagJSON,
		Compact: true,
	})
}

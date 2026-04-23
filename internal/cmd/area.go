package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/swifty99/hactl/internal/config"
	"github.com/swifty99/hactl/internal/format"
	"github.com/swifty99/hactl/internal/haapi"
)

var areaCmd = &cobra.Command{
	Use:   "area",
	Short: "Discover areas (rooms)",
	Long:  "List Home Assistant areas (rooms) and their assignments.",
}

var areaLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all areas",
	Long:  "Show all areas (rooms) registered in Home Assistant.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAreaLs(cmd.Context(), cmd.OutOrStdout())
	},
}

func init() {
	areaCmd.AddCommand(areaLsCmd)
	rootCmd.AddCommand(areaCmd)
}

func runAreaLs(ctx context.Context, w io.Writer) error {
	cfg, err := config.Load(flagDir)
	if err != nil {
		return err
	}

	ws := haapi.NewWSClient(cfg.URL, cfg.Token)
	if connErr := ws.Connect(ctx); connErr != nil {
		return fmt.Errorf("connecting to HA: %w", connErr)
	}
	defer func() { _ = ws.Close() }()

	areas, err := ws.AreaRegistryList(ctx)
	if err != nil {
		return fmt.Errorf("fetching areas: %w", err)
	}

	if len(areas) == 0 {
		_, _ = fmt.Fprintln(w, "no areas")
		return nil
	}

	// Resolve floor names
	floors, floorErr := ws.FloorRegistryList(ctx)
	floorMap := make(map[string]string, len(floors))
	if floorErr == nil {
		for _, f := range floors {
			floorMap[f.FloorID] = f.Name
		}
	}

	// Resolve label names
	labels, labelErr := ws.LabelRegistryList(ctx)
	labelMap := make(map[string]string, len(labels))
	if labelErr == nil {
		for _, l := range labels {
			labelMap[l.LabelID] = l.Name
		}
	}

	tbl := &format.Table{
		Headers: []string{"area_id", "name", "floor", "labels"},
		Rows:    make([][]string, len(areas)),
	}
	for i, a := range areas {
		floorName := floorMap[a.FloorID]
		var lblNames []string
		for _, lid := range a.Labels {
			if name, ok := labelMap[lid]; ok {
				lblNames = append(lblNames, name)
			} else {
				lblNames = append(lblNames, lid)
			}
		}
		tbl.Rows[i] = []string{
			a.AreaID,
			a.Name,
			floorName,
			joinStrings(lblNames),
		}
	}

	return tbl.Render(w, format.RenderOpts{
		Top:     flagTop,
		Full:    flagFull,
		JSON:    flagJSON,
		Compact: true,
	})
}

func joinStrings(s []string) string {
	return strings.Join(s, ", ")
}

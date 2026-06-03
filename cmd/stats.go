package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/history"
	"github.com/spf13/cobra"
)

func newStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show cleanup statistics",
		Long:  "Display all-time aggregated cleanup statistics with per-ecosystem breakdown.",
		RunE: func(cmd *cobra.Command, args []string) error {
			dataDir := getDataDir()

			entries, err := history.Read(dataDir)
			if err != nil {
				return fmt.Errorf("reading history: %w", err)
			}

			if len(entries) == 0 {
				fmt.Println("No statistics available yet. Run `devclean clean` to start tracking.")
				return nil
			}

			printStats(entries)
			return nil
		},
	}

	return cmd
}

type ecosystemStats struct {
	name       string
	targets    int
	freedBytes int64
}

func printStats(entries []history.Entry) {
	totalRuns := len(entries)
	var totalTargets int
	var totalFreed int64
	var earliest, latest time.Time

	ecoMap := make(map[string]*ecosystemStats)

	for _, e := range entries {
		totalTargets += e.Targets
		totalFreed += e.FreedBytes

		if earliest.IsZero() || e.Timestamp.Before(earliest) {
			earliest = e.Timestamp
		}
		if latest.IsZero() || e.Timestamp.After(latest) {
			latest = e.Timestamp
		}

		for _, eco := range e.Ecosystems {
			if _, ok := ecoMap[eco]; !ok {
				ecoMap[eco] = &ecosystemStats{name: eco}
			}
			ecoMap[eco].targets += e.Targets / len(e.Ecosystems) // approximate per-eco
			ecoMap[eco].freedBytes += e.FreedBytes / int64(len(e.Ecosystems))
		}
	}

	fmt.Println("DevClean Statistics")
	fmt.Println("───────────────────")
	fmt.Printf("Total runs:     %d\n", totalRuns)
	fmt.Printf("Total cleaned:  %d targets\n", totalTargets)
	fmt.Printf("Total freed:    %s\n", formatBytes(totalFreed))
	if !earliest.IsZero() {
		fmt.Printf("Period:         %s → %s\n", earliest.Format("2006-01-02"), latest.Format("2006-01-02"))
	}

	if len(ecoMap) > 0 {
		fmt.Println()
		fmt.Println("Ecosystem Breakdown (by freed space)")
		fmt.Println("────────────────────────────────────")

		ecos := make([]ecosystemStats, 0, len(ecoMap))
		for _, s := range ecoMap {
			ecos = append(ecos, *s)
		}
		sort.Slice(ecos, func(i, j int) bool {
			return ecos[i].freedBytes > ecos[j].freedBytes
		})

		maxFreed := ecos[0].freedBytes
		maxBarWidth := 30

		for _, eco := range ecos {
			barWidth := 0
			if maxFreed > 0 {
				barWidth = int(float64(eco.freedBytes) / float64(maxFreed) * float64(maxBarWidth))
			}
			if barWidth < 1 && eco.freedBytes > 0 {
				barWidth = 1
			}
			bar := strings.Repeat("█", barWidth)
			pct := 0
			if totalFreed > 0 {
				pct = int(float64(eco.freedBytes) / float64(totalFreed) * 100)
			}
			fmt.Printf("%-8s  %-30s  %s (%d%%)\n", eco.name, bar, formatBytes(eco.freedBytes), pct)
		}
	}
}

func init() {
	// Ensure we don't shadow os import
	_ = os.Stdout
}

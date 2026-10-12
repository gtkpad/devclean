package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/history"
	"github.com/spf13/cobra"
)

func newHistoryCmd() *cobra.Command {
	var (
		jsonOutput bool
		since      string
		ecosystem  string
	)

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show cleanup history",
		Long:  "Display past cleanup runs with details on targets, freed space, and ecosystems.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			dataDir := getDataDir()

			// Truncate old entries
			retention := time.Duration(cfg.History.RetentionDays) * 24 * time.Hour
			_ = history.Truncate(dataDir, retention)

			opts := history.FilterOpts{}
			if since != "" {
				dur, err := parseDuration(since)
				if err != nil {
					return fmt.Errorf("invalid --since value %q: %w", since, err)
				}
				opts.Since = time.Now().Add(-dur)
			}
			if ecosystem != "" {
				opts.Ecosystem = ecosystem
			}

			entries, err := history.ReadFiltered(dataDir, opts)
			if err != nil {
				return fmt.Errorf("reading history: %w", err)
			}

			if len(entries) == 0 {
				fmt.Println("No cleanup history found. Run `devclean clean` to start recording.")
				return nil
			}

			if jsonOutput {
				data, err := json.MarshalIndent(entries, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(data))
				return nil
			}

			printHistoryTable(entries)
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON array")
	cmd.Flags().StringVar(&since, "since", "", "filter entries newer than duration (e.g., 7d, 30d, 1h)")
	cmd.Flags().StringVar(&ecosystem, "ecosystem", "", "filter by ecosystem name")

	return cmd
}

func getDataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "devclean")
}

// parseDuration parses human-friendly durations like "7d", "30d", "1h".
func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func printHistoryTable(entries []history.Entry) {
	fmt.Printf("%-20s  %-8s  %-10s  %-20s  %s\n", "Date", "Targets", "Freed", "Ecosystems", "Duration")
	fmt.Printf("%-20s  %-8s  %-10s  %-20s  %s\n", "────", "───────", "─────", "──────────", "────────")

	for _, e := range entries {
		date := e.Timestamp.Format("2006-01-02 15:04")
		freed := formatBytes(e.FreedBytes)
		ecos := strings.Join(e.Ecosystems, ", ")
		dur := e.Duration.Round(100 * time.Millisecond).String()
		fmt.Printf("%-20s  %-8d  %-10s  %-20s  %s\n", date, e.Targets, freed, ecos, dur)
	}
}

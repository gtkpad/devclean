package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gtkpad/devclean/internal/cleaner/docker"
	"github.com/gtkpad/devclean/internal/cleaner/dotnet"
	"github.com/gtkpad/devclean/internal/cleaner/golang"
	"github.com/gtkpad/devclean/internal/cleaner/java"
	"github.com/gtkpad/devclean/internal/cleaner/node"
	"github.com/gtkpad/devclean/internal/cleaner/python"
	"github.com/gtkpad/devclean/internal/cleaner/rust"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/watch"
	"github.com/spf13/cobra"
	"log/slog"
)

func newWatchCmd() *cobra.Command {
	var threshold string
	var interval string

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Run a one-shot scan and notify if space exceeds threshold",
		Long:  "Scans all ecosystems and reports total reclaimable space. Sends desktop notification if threshold exceeded.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfigWithProfile()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			thresholdBytes := cfg.Watch.ThresholdMB * 1024 * 1024
			if threshold != "" {
				parsed, err := parseBytes(threshold)
				if err != nil {
					return fmt.Errorf("invalid --threshold value %q: %w", threshold, err)
				}
				thresholdBytes = parsed
			}

			resolver := platform.New()
			paths := resolver.Resolve()
			eng := engine.New(cfg, resolver)

			eng.Register(docker.New())
			eng.Register(golang.New())
			eng.Register(node.New(cfg.ScanPaths))
			eng.Register(dotnet.New(paths, cfg.ScanPaths))
			eng.Register(python.New(paths.PythonUVCache, cfg.ScanPaths))
			eng.Register(rust.New(cfg.ScanPaths))
			eng.Register(java.New(cfg.ScanPaths))

			w := &watch.Watcher{
				ScanFn:         eng.Scan,
				ThresholdBytes: thresholdBytes,
				Notifier:       watch.NewBeeepNotifier(),
				Logger:         slog.Default(),
			}

			ctx := context.Background()
			totalBytes, targetCount, err := w.RunOnce(ctx)
			if err != nil {
				return fmt.Errorf("watch scan failed: %w", err)
			}

			fmt.Fprintf(os.Stdout, "Found %d targets, %s reclaimable\n",
				targetCount, humanSize(totalBytes))

			if totalBytes >= thresholdBytes {
				fmt.Fprintf(os.Stdout, "⚠ Above threshold (%s) — notification sent\n",
					humanSize(thresholdBytes))
			} else {
				fmt.Fprintf(os.Stdout, "✓ Below threshold (%s)\n",
					humanSize(thresholdBytes))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&threshold, "threshold", "", "notification threshold (e.g. 2GB, 500MB)")
	cmd.Flags().StringVar(&interval, "interval", "", "scan interval for daemon mode (e.g. 6h, 1h)")

	return cmd
}

// parseBytes parses a human-readable byte size string (e.g. "2GB", "500MB", "1TB").
func parseBytes(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty size string")
	}

	s = strings.ToUpper(s)

	multipliers := map[string]int64{
		"TB": 1024 * 1024 * 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
		"MB": 1024 * 1024,
		"KB": 1024,
		"B":  1,
	}

	for suffix, mult := range multipliers {
		if strings.HasSuffix(s, suffix) {
			numStr := strings.TrimSuffix(s, suffix)
			numStr = strings.TrimSpace(numStr)
			val, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid number %q", numStr)
			}
			return int64(val * float64(mult)), nil
		}
	}

	// No suffix — treat as bytes
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	return val, nil
}

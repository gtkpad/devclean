package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gtkpad/devclean/internal/cleaner/docker"
	"github.com/gtkpad/devclean/internal/cleaner/dotnet"
	"github.com/gtkpad/devclean/internal/cleaner/golang"
	"github.com/gtkpad/devclean/internal/cleaner/node"
	"github.com/gtkpad/devclean/internal/cleaner/python"
	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/scheduler"
	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	var auto bool
	var force bool
	var dryRun bool
	var ecosystem string

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Delete scan targets to free disk space",
		Long:  "Removes build artifacts, caches, and unused dependencies. Requires --auto or --force flag.",
		RunE: func(cmd *cobra.Command, args []string) error {
			startTime := time.Now()

			if !auto && !force {
				fmt.Fprintln(os.Stdout, `No deletion mode specified. Use:
  devclean clean --auto   Apply TTL rules from config
  devclean clean --force  Delete ALL scan targets

Run 'devclean scan' to preview what would be cleaned.`)
				return nil
			}

			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			resolver := platform.New()
			paths := resolver.Resolve()
			eng := engine.New(cfg, resolver)

			eng.Register(docker.New())
			eng.Register(golang.New())
			eng.Register(node.New(cfg.ScanPaths))
			eng.Register(dotnet.New(paths, cfg.ScanPaths))
			eng.Register(python.New(paths.PythonUVCache))

			ctx := context.Background()
			ch := eng.Scan(ctx)
			results := engine.Collect(ctx, ch)

			if ecosystem != "" {
				results = filterByEcosystem(results, ecosystem)
			}

			bl := safety.NewBlacklist(cfg.ProtectedPatterns)

			var totalFreed int64
			var totalCount int

			for _, r := range results {
				if r.Error != nil || len(r.Targets) == 0 {
					continue
				}

				var targets []domain.Target
				if auto {
					maxAge := getMaxAgeDays(cfg, r.Ecosystem)
					targets = filterByTTL(r.Targets, maxAge)
				} else {
					targets = r.Targets
				}

				targets = rejectBlacklisted(targets, bl)

				if len(targets) == 0 {
					continue
				}

				cleaner := eng.GetCleaner(r.Ecosystem)
				if cleaner == nil {
					continue
				}

				for _, t := range targets {
					if !quiet {
						if dryRun {
							fmt.Fprintf(os.Stdout, "Would delete: %s (%s)\n", t.Path, humanSize(t.SizeBytes))
						} else {
							fmt.Fprintf(os.Stdout, "Deleting: %s\n", t.Path)
						}
					}
				}

				if !dryRun {
					if err := cleaner.Clean(ctx, targets); err != nil {
						fmt.Fprintf(os.Stderr, "Error cleaning %s: %v\n", r.Ecosystem, err)
						continue
					}
				}

				for _, t := range targets {
					totalFreed += t.SizeBytes
					totalCount++
				}
			}

			if totalCount == 0 {
				fmt.Fprintln(os.Stderr, "Nothing to clean")
				os.Exit(2)
			}

			if auto && !dryRun {
				_ = scheduler.RecordRun(paths.ConfigDir, scheduler.RunResult{
					Timestamp:    time.Now(),
					TargetsCount: totalCount,
					BytesFreed:   totalFreed,
					Duration:     time.Since(startTime).Round(time.Millisecond).String(),
				})
			}

			if dryRun {
				fmt.Fprintf(os.Stdout, "\nDry-run: would free %s from %d targets\n", humanSize(totalFreed), totalCount)
			} else {
				fmt.Fprintf(os.Stdout, "\nFreed: %s from %d targets\n", humanSize(totalFreed), totalCount)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&auto, "auto", false, "apply TTL rules from config and delete matching targets")
	cmd.Flags().BoolVar(&force, "force", false, "delete ALL scan targets without TTL filter")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview what would be deleted without removing anything")
	cmd.Flags().StringVarP(&ecosystem, "ecosystem", "e", "", "filter by ecosystem name")

	return cmd
}

func filterByTTL(targets []domain.Target, maxAgeDays int) []domain.Target {
	if maxAgeDays == 0 {
		return targets
	}

	cutoff := time.Now().Add(-time.Duration(maxAgeDays) * 24 * time.Hour)
	var filtered []domain.Target
	for _, t := range targets {
		if t.LastAccessed.Before(cutoff) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func rejectBlacklisted(targets []domain.Target, bl *safety.Blacklist) []domain.Target {
	var safe []domain.Target
	for _, t := range targets {
		if !bl.IsBlacklisted(t.Path) {
			safe = append(safe, t)
		}
	}
	return safe
}

func getMaxAgeDays(cfg *config.Config, ecosystem string) int {
	switch ecosystem {
	case "docker":
		return cfg.Ecosystems.Docker.MaxAgeDays
	case "node":
		return cfg.Ecosystems.Node.MaxAgeDays
	case "go":
		return cfg.Ecosystems.Go.MaxAgeDays
	case "dotnet":
		return cfg.Ecosystems.DotNet.MaxAgeDays
	case "python":
		return cfg.Ecosystems.Python.MaxAgeDays
	default:
		return 0
	}
}

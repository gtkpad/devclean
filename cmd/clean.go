package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gtkpad/devclean/internal/cleaner/docker"
	"github.com/gtkpad/devclean/internal/cleaner/dotnet"
	"github.com/gtkpad/devclean/internal/cleaner/golang"
	"github.com/gtkpad/devclean/internal/cleaner/java"
	"github.com/gtkpad/devclean/internal/cleaner/node"
	"github.com/gtkpad/devclean/internal/cleaner/python"
	"github.com/gtkpad/devclean/internal/cleaner/rust"
	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/history"
	"github.com/gtkpad/devclean/internal/hooks"
	"github.com/gtkpad/devclean/internal/intelligence"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/scheduler"
	"github.com/gtkpad/devclean/internal/trash"
	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	var auto bool
	var force bool
	var dryRun bool
	var ecosystem string
	var useTrash bool
	var permanent bool
	var smart bool

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

			if auto && force {
				return fmt.Errorf("--auto and --force are mutually exclusive")
			}

			if useTrash && permanent {
				return fmt.Errorf("--trash and --permanent are mutually exclusive")
			}

			if smart && !auto {
				return fmt.Errorf("--smart requires --auto (use: devclean clean --auto --smart)")
			}

			if smart && force {
				return fmt.Errorf("--smart and --force are mutually exclusive")
			}

			cfg, err := loadConfigWithProfile()
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
			eng.Register(python.New(paths.PythonUVCache, cfg.ScanPaths))
			eng.Register(rust.New(cfg.ScanPaths))
			eng.Register(java.New(cfg.ScanPaths))

			if ecosystem != "" {
				if err := validateEcosystem(ecosystem, eng.Ecosystems()); err != nil {
					return err
				}
			}

			ctx := context.Background()
			ch := eng.Scan(ctx)
			results := engine.Collect(ctx, ch)

			if ecosystem != "" {
				results = filterByEcosystem(results, ecosystem)
			}

			bl := safety.NewBlacklist(cfg.ProtectedPatterns)
			hookRunner := hooks.New(cfg.Hooks)

			var scorer *intelligence.Scorer
			if smart {
				scorer = intelligence.NewScorer(cfg.Intelligence)
			}

			// Resolve trash usage: --permanent > --trash > config
			trashActive := cfg.Safety.TrashEnabled
			if useTrash {
				trashActive = true
			}
			if permanent {
				trashActive = false
			}

			var trashMgr *trash.Manager
			if trashActive {
				trashMgr = trash.New(cfg.Safety)
				// Enforce retention and max size before new moves
				if err := trashMgr.EnforceRetention(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: trash retention enforcement failed: %v\n", err)
				}
				if err := trashMgr.EnforceMaxSize(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: trash max size enforcement failed: %v\n", err)
				}
			}

			var totalFreed int64
			var totalCount int
			var cleanedPaths []string
			ecosystemSet := map[string]struct{}{}

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

				if smart && scorer != nil {
					var smartFiltered []domain.Target
					for _, t := range targets {
						score, err := scorer.Score(ctx, t)
						if err != nil {
							continue
						}
						if score.Total >= cfg.Intelligence.Threshold {
							smartFiltered = append(smartFiltered, t)
						}
					}
					if !quiet {
						fmt.Fprintf(os.Stdout, "Smart mode [%s]: %d/%d targets above threshold (%.2f)\n",
							r.Ecosystem, len(smartFiltered), len(targets), cfg.Intelligence.Threshold)
					}
					targets = smartFiltered
				}

				targets = rejectBlacklisted(targets, bl)

				if len(targets) == 0 {
					continue
				}

				for _, t := range targets {
					if !quiet {
						if dryRun {
							fmt.Fprintf(os.Stdout, "Would delete: %s (%s)\n", t.Path, humanSize(t.SizeBytes))
						} else if trashActive {
							fmt.Fprintf(os.Stdout, "Moving to trash: %s\n", t.Path)
						} else {
							fmt.Fprintf(os.Stdout, "Deleting: %s\n", t.Path)
						}
					}
				}

				if !dryRun {
					// Pre-delete hook
					hookData := hooks.HookData{
						Ecosystem:   r.Ecosystem,
						TargetCount: len(targets),
					}
					if err := hookRunner.RunPreDelete(ctx, r.Ecosystem, hookData); err != nil {
						fmt.Fprintf(os.Stderr, "Pre-delete hook failed for %s: %v (skipping)\n", r.Ecosystem, err)
						continue
					}

					if trashActive {
						if err := trashMgr.Move(ctx, targets, r.Ecosystem); err != nil {
							fmt.Fprintf(os.Stderr, "Error trashing %s: %v\n", r.Ecosystem, err)
							continue
						}
					} else {
						cleaner := eng.GetCleaner(r.Ecosystem)
						if cleaner == nil {
							continue
						}
						if err := cleaner.Clean(ctx, targets); err != nil {
							fmt.Fprintf(os.Stderr, "Error cleaning %s: %v\n", r.Ecosystem, err)
							continue
						}
					}

					// Post-delete hook (warn only)
					hookData.FreedSize = humanSize(totalFreed)
					if err := hookRunner.RunPostDelete(ctx, r.Ecosystem, hookData); err != nil {
						fmt.Fprintf(os.Stderr, "Post-delete hook warning: %v\n", err)
					}
				}

				for _, t := range targets {
					totalFreed += t.SizeBytes
					totalCount++
					cleanedPaths = append(cleanedPaths, t.Path)
				}
				ecosystemSet[r.Ecosystem] = struct{}{}
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

			// Record history entry
			if !dryRun && totalCount > 0 {
				ecosystems := make([]string, 0, len(ecosystemSet))
				for eco := range ecosystemSet {
					ecosystems = append(ecosystems, eco)
				}
				mode := "force"
				if auto {
					mode = "auto"
				}
				dataDir := getDataDir()
				entry := history.Entry{
					Timestamp:  time.Now(),
					Targets:    totalCount,
					Paths:      cleanedPaths,
					FreedBytes: totalFreed,
					Ecosystems: ecosystems,
					Duration:   time.Since(startTime).Round(time.Millisecond),
					Mode:       mode,
				}
				if err := history.Append(dataDir, entry); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to record history: %v\n", err)
				}
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
	cmd.Flags().BoolVar(&useTrash, "trash", false, "move targets to trash instead of permanent deletion")
	cmd.Flags().BoolVar(&permanent, "permanent", false, "force permanent deletion even when trash is enabled")
	cmd.Flags().BoolVar(&smart, "smart", false, "only delete targets above score threshold (use with --auto)")

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

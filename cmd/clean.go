package cmd

import (
	"context"
	"errors"
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
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/hooks"
	"github.com/gtkpad/devclean/internal/intelligence"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/safety"
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
		Use:          "clean",
		Short:        "Delete scan targets to free disk space",
		Long:         "Removes build artifacts, caches, and unused dependencies. Requires --auto or --force flag.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
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

			var scorer *intelligence.Scorer
			if smart {
				scorer = intelligence.NewScorer(cfg.Intelligence)
			}

			targetsByEco := make(map[string][]domain.Target)
			totalCandidates := 0

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
				if len(targets) > 0 {
					targetsByEco[r.Ecosystem] = targets
					totalCandidates += len(targets)
				}
			}

			if totalCandidates == 0 {
				return errors.New("nothing to clean")
			}

			dataDir := getDataDir()
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
				if err := trashMgr.EnforceRetention(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: trash retention enforcement failed: %v\n", err)
				}
				if err := trashMgr.EnforceMaxSize(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: trash max size enforcement failed: %v\n", err)
				}
			} else {
				trashMgr = trash.New(cfg.Safety)
			}

			hookRunner := hooks.New(cfg.Hooks)
			coord := deleter.New(eng, cfg, bl, trashMgr, hookRunner, paths.ConfigDir, dataDir)

			var useTrashOpt *bool
			if useTrash {
				t := true
				useTrashOpt = &t
			} else if permanent {
				f := false
				useTrashOpt = &f
			}

			mode := "force"
			if auto {
				mode = "auto"
			}
			opts := deleter.Options{
				DryRun:   dryRun,
				UseTrash: useTrashOpt,
				Mode:     mode,
				Auto:     auto,
				OnProgress: func(ev deleter.ProgressEvent) {
					if quiet {
						return
					}
					switch ev.Action {
					case deleter.ActionWouldDelete:
						fmt.Fprintf(os.Stdout, "Would delete: %s (%s)\n", ev.Target.Path, humanSize(ev.Target.SizeBytes))
					case deleter.ActionMovingTrash:
						fmt.Fprintf(os.Stdout, "Moving to trash: %s\n", ev.Target.Path)
					case deleter.ActionDeletingDirect, deleter.ActionVirtualDirect:
						fmt.Fprintf(os.Stdout, "Deleting: %s\n", ev.Target.Path)
					}
				},
				OnWarning: func(msg string) {
					fmt.Fprintln(os.Stderr, "Warning:", msg)
				},
			}

			res, err := coord.Delete(ctx, targetsByEco, opts)
			if err != nil {
				return fmt.Errorf("deletion failed: %w", err)
			}

			for _, err := range res.Errors {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}

			if dryRun {
				fmt.Fprintf(os.Stdout, "\nDry-run: would free %s from %d targets\n", humanSize(res.TotalFreed), res.TotalCount)
			} else {
				fmt.Fprintf(os.Stdout, "\nFreed: %s from %d targets\n", humanSize(res.TotalFreed), res.TotalCount)
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

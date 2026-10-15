package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/hooks"
	"github.com/gtkpad/devclean/internal/intelligence"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/trash"
	"github.com/gtkpad/devclean/internal/ui"
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
	var all bool
	var format string
	var preset string

	cmd := &cobra.Command{
		Use:          "clean",
		Short:        "Delete scan targets to free disk space",
		Long:         "Removes build artifacts, caches, and unused dependencies. Requires --auto or --force flag.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "table" && format != "json" {
				return fmt.Errorf("unknown format: %q (supported: table, json)", format)
			}

			if preset != "" {
				if preset != "safe" && preset != "standard" && preset != "aggressive" {
					return fmt.Errorf("unknown preset: %q (supported: safe, standard, aggressive)", preset)
				}
				if force {
					return fmt.Errorf("--preset and --force are mutually exclusive")
				}
				auto = true
			}

			if !auto && !force {
				fmt.Fprintln(cmd.OutOrStdout(), `No deletion mode specified. Use:
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

			if preset == "aggressive" {
				zeroAllMaxAgeDays(cfg)
				if !permanent {
					useTrash = true
				}
			} else if all {
				zeroAllMaxAgeDays(cfg)
			}

			resolver := platform.New()
			paths := resolver.Resolve()
			eng := engine.New(cfg, resolver)

			engine.RegisterDefaultCleaners(eng, cfg, resolver)

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

				if preset == "safe" {
					targets = filterSafePreset(targets)
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
					if !quiet && format != "json" {
						fmt.Fprintf(cmd.OutOrStdout(), "Smart mode [%s]: %d/%d targets above threshold (%.2f)\n",
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

			trashMgr := trash.New(cfg.Safety)

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
					if quiet || format == "json" {
						return
					}
					switch ev.Action {
					case deleter.ActionWouldDelete:
						fmt.Fprintf(cmd.OutOrStdout(), "Would delete: %s (%s)\n", ev.Target.Path, humanSize(ev.Target.SizeBytes))
					case deleter.ActionMovingTrash:
						fmt.Fprintf(cmd.OutOrStdout(), "Moving to trash: %s\n", ev.Target.Path)
					case deleter.ActionDeletingDirect, deleter.ActionVirtualDirect:
						fmt.Fprintf(cmd.OutOrStdout(), "Deleting: %s\n", ev.Target.Path)
					}
				},
				OnWarning: func(msg string) {
					if !quiet {
						fmt.Fprintln(os.Stderr, "Warning:", msg)
					}
				},
			}

			diskBefore, errBefore := fs.GetDiskSpace(".")

			res, err := coord.Delete(ctx, targetsByEco, opts)
			if err != nil {
				return fmt.Errorf("deletion failed: %w", err)
			}

			for _, err := range res.Errors {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}

			diskAfter, errAfter := fs.GetDiskSpace(".")

			var diskBeforeFree, diskAfterFree *uint64
			if errBefore == nil {
				b := diskBefore.FreeBytes
				diskBeforeFree = &b
			}
			if errAfter == nil {
				a := diskAfter.FreeBytes
				diskAfterFree = &a
			}

			var trashCount, permanentCount int
			if trashActive {
				trashCount = res.TotalCount
			} else {
				permanentCount = res.TotalCount
			}

			var ecoMetrics []ui.EcosystemMetric
			for eco, targets := range targetsByEco {
				var freed int64
				for _, t := range targets {
					freed += t.SizeBytes
				}
				ecoMetrics = append(ecoMetrics, ui.EcosystemMetric{
					Name:       eco,
					FreedBytes: freed,
					Count:      len(targets),
				})
			}
			sort.Slice(ecoMetrics, func(i, j int) bool {
				if ecoMetrics[i].FreedBytes != ecoMetrics[j].FreedBytes {
					return ecoMetrics[i].FreedBytes > ecoMetrics[j].FreedBytes
				}
				return ecoMetrics[i].Name < ecoMetrics[j].Name
			})

			title := "Clean Summary"
			if dryRun {
				title = "Clean Dry-Run Summary"
			}

			reportData := ui.ReportData{
				Title:          title,
				TotalFreed:     res.TotalFreed,
				TotalCount:     res.TotalCount,
				TrashCount:     trashCount,
				PermanentCount: permanentCount,
				Duration:       res.Duration,
				DiskBefore:     diskBeforeFree,
				DiskAfter:      diskAfterFree,
				Ecosystems:     ecoMetrics,
			}

			if quiet {
				return nil
			}

			if format == "json" {
				return ui.FormatReportJSON(cmd.OutOrStdout(), reportData)
			}

			ui.RenderReport(cmd.OutOrStdout(), reportData)
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
	cmd.Flags().BoolVar(&all, "all", false, "apply zero TTL across all ecosystems")
	cmd.Flags().StringVar(&preset, "preset", "", "cleanup preset strategy: safe (global caches only), standard (config TTLs), aggressive (TTL=0)")
	cmd.Flags().StringVar(&format, "format", "table", "output format (table, json)")

	return cmd
}

func filterByTTL(targets []domain.Target, maxAgeDays int) []domain.Target {
	if maxAgeDays == 0 {
		return targets
	}

	cutoff := time.Now().Add(-time.Duration(maxAgeDays) * 24 * time.Hour)
	var filtered []domain.Target
	for _, t := range targets {
		if t.TTLEligible {
			filtered = append(filtered, t)
		} else if t.Category == "" && t.SizeBytes == 0 && !t.LastAccessed.IsZero() && t.LastAccessed.Before(cutoff) {
			// Fallback for legacy test targets where TTLEligible was not evaluated by a cleaner
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func filterSafePreset(targets []domain.Target) []domain.Target {
	var safe []domain.Target
	for _, t := range targets {
		if t.Risk != domain.RiskLow {
			continue
		}
		// Exclui artefatos de compilação ou dependências locais de projetos
		switch t.Category {
		case "cmake-build", "vcpkg-installed",
			"flutter-build", "flutter-dart-tool",
			"terraform-local",
			"cocoapods-pods",
			"elixir-build", "elixir-deps",
			"cargo-target", "node", "node-modules", "node-dist", "nuxt-cache",
			"dotnet-build", "maven-target", "gradle-build", "nextjs-build", "nextjs-cache",
			"python-venv-orphan", "python-pycache",
			"derived-data", "xcode-derived-data", "archives":
			continue
		default:
			safe = append(safe, t)
		}
	}
	return safe
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
	case "rust":
		return cfg.Ecosystems.Rust.MaxAgeDays
	case "java":
		return cfg.Ecosystems.Java.MaxAgeDays
	case "ai":
		return cfg.Ecosystems.AI.MaxAgeDays
	case "pnpm":
		return cfg.Ecosystems.Pnpm.MaxAgeDays
	case "bun":
		return cfg.Ecosystems.Bun.MaxAgeDays
	case "nextjs":
		return cfg.Ecosystems.NextJS.MaxAgeDays
	case "xcode":
		return cfg.Ecosystems.Xcode.MaxAgeDays
	case "homebrew":
		return cfg.Ecosystems.Homebrew.MaxAgeDays
	case "cpp":
		return cfg.Ecosystems.CPP.MaxAgeDays
	case "flutter":
		return cfg.Ecosystems.Flutter.MaxAgeDays
	case "terraform":
		return cfg.Ecosystems.Terraform.MaxAgeDays
	case "cocoapods":
		return cfg.Ecosystems.CocoaPods.MaxAgeDays
	case "elixir":
		return cfg.Ecosystems.Elixir.MaxAgeDays
	default:
		return 0
	}
}

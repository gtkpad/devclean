package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

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
	"golang.org/x/term"
)

var (
	purgeStdinReader io.Reader = os.Stdin
	purgeIsTerminal            = func(fd int) bool {
		return term.IsTerminal(fd)
	}
)

type repoCheckResult struct {
	eligible bool
	reason   string
}

func newPurgeCmd() *cobra.Command {
	var inactiveDays int
	var mergedOnly bool
	var dryRun bool
	var yes bool
	var useTrash bool
	var permanent bool
	var format string

	cmd := &cobra.Command{
		Use:          "purge",
		Short:        "Purge build artifacts in inactive or merged repositories",
		Long:         "Removes heavy build artifacts and orphan dependencies from Git repositories without modifying git state.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if useTrash && permanent {
				return fmt.Errorf("--trash and --permanent are mutually exclusive")
			}
			if format != "table" && format != "json" {
				return fmt.Errorf("unknown format: %q (supported: table, json)", format)
			}

			cfg, err := loadConfigWithProfile()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			resolver := platform.New()
			paths := resolver.Resolve()
			eng := engine.New(cfg, resolver)
			engine.RegisterDefaultCleaners(eng, cfg, resolver)

			ctx := context.Background()
			ch := eng.Scan(ctx)
			results := engine.Collect(ctx, ch)

			bl := safety.NewBlacklist(cfg.ProtectedPatterns)

			repoCache := make(map[string]repoCheckResult)

			var eligibleTargets []domain.Target
			targetsByEco := make(map[string][]domain.Target)
			targetRepoMap := make(map[string]string)

			for _, r := range results {
				if r.Error != nil || len(r.Targets) == 0 {
					continue
				}

				for _, t := range r.Targets {
					// D-03: Exclude virtual targets
					if t.IsVirtual() {
						continue
					}

					// Blacklist protection
					if bl.IsBlacklisted(t.Path) {
						continue
					}

					// D-03, D-15: Locate git repository
					workTreeDir, gitDir := intelligence.FindGitRepoLocal(t.Path)
					if gitDir == "" || workTreeDir == "" {
						// Non-git directory: silently ignore per D-15, NFR-04
						continue
					}

					chk, evaluated := repoCache[workTreeDir]
					if !evaluated {
						chk = evaluateRepoForPurge(ctx, workTreeDir, inactiveDays, mergedOnly, quiet)
						repoCache[workTreeDir] = chk
					}

					if !chk.eligible {
						continue
					}

					eligibleTargets = append(eligibleTargets, t)
					targetsByEco[r.Ecosystem] = append(targetsByEco[r.Ecosystem], t)
					targetRepoMap[t.Path] = workTreeDir
				}
			}

			if len(eligibleTargets) == 0 {
				if !quiet && format == "table" {
					fmt.Fprintln(cmd.OutOrStdout(), "Nenhum artefato elegível para purge encontrado.")
				}
				return nil
			}

			// Interactive Preview & Confirmation
			if !quiet && format == "table" {
				renderPurgePreview(cmd.OutOrStdout(), eligibleTargets, targetRepoMap)
			}

			if !yes && !dryRun {
				isTTY := purgeIsTerminal(int(os.Stdin.Fd()))
				if !isTTY {
					return fmt.Errorf("confirmação interativa indisponível sem TTY; use --yes para prosseguir")
				}

				fmt.Fprint(cmd.OutOrStdout(), "Deseja prosseguir com o expurgo? [y/N]: ")
				reader := bufio.NewReader(purgeStdinReader)
				ans, err := reader.ReadString('\n')
				if err != nil && err != io.EOF {
					return fmt.Errorf("lendo confirmação: %w", err)
				}
				ans = strings.TrimSpace(strings.ToLower(ans))
				if ans != "y" && ans != "yes" {
					fmt.Fprintln(cmd.OutOrStdout(), "Expurgo cancelado pelo usuário.")
					return nil
				}
			}

			// Coordinator Deletion Execution
			dataDir := getDataDir()
			trashActive := cfg.Safety.TrashEnabled
			if useTrash {
				trashActive = true
			}
			if permanent {
				trashActive = false
			}

			trashMgr := trash.New(cfg.Safety)
			if trashActive && !dryRun {
				if err := trashMgr.EnforceRetention(ctx); err != nil {
					if !quiet {
						fmt.Fprintf(os.Stderr, "Warning: trash retention enforcement failed: %v\n", err)
					}
				}
				if err := trashMgr.EnforceMaxSize(ctx); err != nil {
					if !quiet {
						fmt.Fprintf(os.Stderr, "Warning: trash max size enforcement failed: %v\n", err)
					}
				}
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

			diskBefore, errBefore := fs.GetDiskSpace(".")

			opts := deleter.Options{
				DryRun:   dryRun,
				UseTrash: useTrashOpt,
				Mode:     "purge",
				Auto:     false,
				OnProgress: func(ev deleter.ProgressEvent) {
					if quiet {
						return
					}
					switch ev.Action {
					case deleter.ActionWouldDelete:
						fmt.Fprintf(cmd.OutOrStdout(), "Would purge: %s (%s)\n", ev.Target.Path, humanSize(ev.Target.SizeBytes))
					case deleter.ActionMovingTrash:
						fmt.Fprintf(cmd.OutOrStdout(), "Moving to trash: %s\n", ev.Target.Path)
					case deleter.ActionDeletingDirect, deleter.ActionVirtualDirect:
						fmt.Fprintf(cmd.OutOrStdout(), "Purging: %s\n", ev.Target.Path)
					}
				},
				OnWarning: func(msg string) {
					if !quiet {
						fmt.Fprintln(os.Stderr, "Warning:", msg)
					}
				},
			}

			res, err := coord.Delete(ctx, targetsByEco, opts)
			if err != nil {
				return fmt.Errorf("purge failed: %w", err)
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

			reportData := ui.ReportData{
				Title:          "DevClean Purge Summary",
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

	cmd.Flags().IntVar(&inactiveDays, "inactive-days", 60, "minimum days without commits to consider repository inactive")
	cmd.Flags().BoolVar(&mergedOnly, "merged-only", false, "purge only repositories whose branch is merged into main/master")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview what would be purged without deleting")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip interactive confirmation prompt")
	cmd.Flags().BoolVar(&useTrash, "trash", false, "move purged artifacts to trash")
	cmd.Flags().BoolVar(&permanent, "permanent", false, "permanently delete purged artifacts")
	cmd.Flags().StringVar(&format, "format", "table", "output format (table, json)")

	return cmd
}

func evaluateRepoForPurge(ctx context.Context, repoDir string, inactiveDays int, mergedOnly bool, isQuiet bool) repoCheckResult {
	lastCommit, err := intelligence.GetRepoLastCommitTime(ctx, repoDir)
	if err != nil {
		if !isQuiet {
			fmt.Fprintf(os.Stderr, "Aviso: falha ao inspecionar histórico do repositório %s: %v\n", repoDir, err)
		}
		return repoCheckResult{eligible: false, reason: "git log error"}
	}
	if lastCommit.IsZero() {
		return repoCheckResult{eligible: false, reason: "no commits"}
	}

	if inactiveDays > 0 {
		age := time.Since(lastCommit)
		cutoff := time.Duration(inactiveDays) * 24 * time.Hour
		if age < cutoff {
			return repoCheckResult{eligible: false, reason: "active repository"}
		}
	}

	if mergedOnly {
		clean, err := intelligence.IsWorkingTreeClean(ctx, repoDir)
		if err != nil || !clean {
			if !isQuiet {
				fmt.Fprintf(os.Stderr, "Aviso: repositório %s possui alterações não comitadas (dirty working tree); ignorado por segurança.\n", repoDir)
			}
			return repoCheckResult{eligible: false, reason: "dirty working tree"}
		}

		merged, err := intelligence.IsBranchMerged(ctx, repoDir)
		if err != nil {
			if !isQuiet {
				fmt.Fprintf(os.Stderr, "Aviso: falha ao verificar status de branch em %s: %v\n", repoDir, err)
			}
			return repoCheckResult{eligible: false, reason: "branch merge check failed"}
		}
		if !merged {
			return repoCheckResult{eligible: false, reason: "unmerged branch"}
		}
	}

	return repoCheckResult{eligible: true}
}

func renderPurgePreview(w io.Writer, targets []domain.Target, repoPaths map[string]string) {
	fmt.Fprintf(w, "%-35s %-50s %10s\n", "REPOSITORY", "PATH", "SIZE")
	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 97))

	var totalBytes int64
	for _, t := range targets {
		repo := repoPaths[t.Path]
		if len(repo) > 35 {
			repo = "..." + repo[len(repo)-32:]
		}
		path := t.Path
		if len(path) > 50 {
			path = "..." + path[len(path)-47:]
		}
		fmt.Fprintf(w, "%-35s %-50s %10s\n", repo, path, humanSize(t.SizeBytes))
		totalBytes += t.SizeBytes
	}
	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 97))
	fmt.Fprintf(w, "Total: %d artefatos elegíveis (%s)\n\n", len(targets), humanSize(totalBytes))
}

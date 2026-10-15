package deleter

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/history"
	"github.com/gtkpad/devclean/internal/hooks"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/scheduler"
	"github.com/gtkpad/devclean/internal/trash"
)

type CleanerProvider interface {
	GetCleaner(name string) domain.Cleaner
}

type ActionType string

const (
	ActionWouldDelete    ActionType = "would_delete"
	ActionMovingTrash    ActionType = "moving_to_trash"
	ActionDeletingDirect ActionType = "deleting_direct"
	ActionVirtualDirect  ActionType = "virtual_direct"
)

type ProgressEvent struct {
	Ecosystem string
	Target    domain.Target
	Action    ActionType
}

type Options struct {
	DryRun     bool
	UseTrash   *bool  // nil: uses cfg.Safety.TrashEnabled; non-nil: CLI/caller override
	Mode       string // "auto", "force", "tui", "mcp"
	Auto       bool   // if true, updates scheduler.RecordRun
	OnProgress func(event ProgressEvent)
	OnWarning  func(msg string)
}

type Result struct {
	TotalFreed        int64
	TotalCount        int
	CleanedPaths      []string
	Ecosystems        []string
	SkippedReviewOnly []domain.Target
	Errors            []error
	Duration          time.Duration
}

type Coordinator struct {
	cleaners  CleanerProvider
	cfg       *config.Config
	blacklist *safety.Blacklist
	trashMgr  *trash.Manager
	hooks     *hooks.Runner
	configDir string
	dataDir   string
}

func New(
	cleaners CleanerProvider,
	cfg *config.Config,
	bl *safety.Blacklist,
	trashMgr *trash.Manager,
	hookRunner *hooks.Runner,
	configDir string,
	dataDir string,
) *Coordinator {
	return &Coordinator{
		cleaners:  cleaners,
		cfg:       cfg,
		blacklist: bl,
		trashMgr:  trashMgr,
		hooks:     hookRunner,
		configDir: configDir,
		dataDir:   dataDir,
	}
}

// Delete coordinates the complete 6-stage deletion lifecycle:
// 1. Safety validation & blacklist filtering
// 2. Partitioning filesystem vs virtual targets
// 3. Pre-delete hooks
// 4. Deletion execution (trash vs direct cleaner vs virtual direct)
// 5. Post-delete hooks
// 6. Audit history & scheduler metrics persistence
func (c *Coordinator) Delete(
	ctx context.Context,
	targetsByEcosystem map[string][]domain.Target,
	opts Options,
) (*Result, error) {
	startTime := time.Now()

	res := &Result{
		CleanedPaths:      make([]string, 0),
		Ecosystems:        make([]string, 0),
		SkippedReviewOnly: make([]domain.Target, 0),
		Errors:            make([]error, 0),
	}

	trashActive := false
	if c.cfg != nil {
		trashActive = c.cfg.Safety.TrashEnabled
	}
	if opts.UseTrash != nil {
		trashActive = *opts.UseTrash
	}

	ecos := make([]string, 0, len(targetsByEcosystem))
	for eco := range targetsByEcosystem {
		ecos = append(ecos, eco)
	}
	sort.Strings(ecos)

	for _, eco := range ecos {
		select {
		case <-ctx.Done():
			res.Duration = time.Since(startTime)
			return res, ctx.Err()
		default:
		}

		rawTargets := targetsByEcosystem[eco]
		if len(rawTargets) == 0 {
			continue
		}

		// Stage 1: Safety Validation, Blacklist & ReviewOnly filtering
		var safeTargets []domain.Target
		for _, t := range rawTargets {
			if c.blacklist != nil && (c.blacklist.IsBlacklisted(t.Path) || isSymlinkToBlacklist(t.Path, c.blacklist)) {
				continue
			}

			if t.ReviewOnly {
				res.SkippedReviewOnly = append(res.SkippedReviewOnly, t)
				if opts.OnWarning != nil {
					reason := t.ReviewReason
					if reason == "" {
						reason = domain.ReviewReasonSharedStore
					}
					opts.OnWarning(fmt.Sprintf("Item de revisão retido [%s]: %s (%s)", reason, t.Path, humanSize(t.SizeBytes)))
				}
				continue
			}

			safeTargets = append(safeTargets, t)
		}
		if len(safeTargets) == 0 {
			continue
		}

		// Stage 2: Partitioning Filesystem vs Virtual
		var fsTargets []domain.Target
		var virtualTargets []domain.Target
		for _, t := range safeTargets {
			if t.IsVirtual() {
				virtualTargets = append(virtualTargets, t)
			} else {
				fsTargets = append(fsTargets, t)
			}
		}

		// Stage 3: Pre-Delete Hooks
		if !opts.DryRun && c.hooks != nil {
			hookData := hooks.HookData{
				Ecosystem:   eco,
				TargetCount: len(safeTargets),
			}
			if err := c.hooks.RunPreDelete(ctx, eco, hookData); err != nil {
				res.Errors = append(res.Errors, fmt.Errorf("pre-delete hook failed for %s: %w", eco, err))
				continue
			}
		}

		// Stage 4: Deletion Execution
		var ecoFreed int64
		var ecoCount int
		var ecoCleanedPaths []string
		ecoSuccess := false

		if opts.DryRun {
			for _, t := range safeTargets {
				if opts.OnProgress != nil {
					opts.OnProgress(ProgressEvent{
						Ecosystem: eco,
						Target:    t,
						Action:    ActionWouldDelete,
					})
				}
				ecoFreed += t.SizeBytes
				ecoCount++
				ecoCleanedPaths = append(ecoCleanedPaths, t.Path)
			}
			ecoSuccess = true
		} else {
			// Filesystem targets
			if len(fsTargets) > 0 {
				if trashActive && c.trashMgr != nil {
					for _, t := range fsTargets {
						if opts.OnProgress != nil {
							opts.OnProgress(ProgressEvent{
								Ecosystem: eco,
								Target:    t,
								Action:    ActionMovingTrash,
							})
						}
					}
					if err := c.trashMgr.Move(ctx, fsTargets, eco); err != nil {
						res.Errors = append(res.Errors, fmt.Errorf("trashing %s: %w", eco, err))
					} else {
						for _, t := range fsTargets {
							ecoFreed += t.SizeBytes
							ecoCount++
							ecoCleanedPaths = append(ecoCleanedPaths, t.Path)
						}
						ecoSuccess = true
					}
				} else {
					var cleaner domain.Cleaner
					if c.cleaners != nil {
						cleaner = c.cleaners.GetCleaner(eco)
					}
					if cleaner == nil {
						res.Errors = append(res.Errors, fmt.Errorf("no cleaner found for ecosystem %s", eco))
					} else {
						for _, t := range fsTargets {
							if opts.OnProgress != nil {
								opts.OnProgress(ProgressEvent{
									Ecosystem: eco,
									Target:    t,
									Action:    ActionDeletingDirect,
								})
							}
						}
						if err := cleaner.Clean(ctx, fsTargets); err != nil {
							res.Errors = append(res.Errors, fmt.Errorf("cleaning %s: %w", eco, err))
						} else {
							for _, t := range fsTargets {
								ecoFreed += t.SizeBytes
								ecoCount++
								ecoCleanedPaths = append(ecoCleanedPaths, t.Path)
							}
							ecoSuccess = true
						}
					}
				}
			}

			// Virtual targets
			if len(virtualTargets) > 0 {
				var cleaner domain.Cleaner
				if c.cleaners != nil {
					cleaner = c.cleaners.GetCleaner(eco)
				}
				if cleaner == nil {
					res.Errors = append(res.Errors, fmt.Errorf("no cleaner found for virtual targets in ecosystem %s", eco))
				} else {
					if trashActive && opts.OnWarning != nil {
						opts.OnWarning(fmt.Sprintf("[%s] %d alvos virtuais limpos diretamente (lixeira ignorada)", eco, len(virtualTargets)))
					}
					for _, t := range virtualTargets {
						if opts.OnProgress != nil {
							opts.OnProgress(ProgressEvent{
								Ecosystem: eco,
								Target:    t,
								Action:    ActionVirtualDirect,
							})
						}
					}
					if err := cleaner.Clean(ctx, virtualTargets); err != nil {
						res.Errors = append(res.Errors, fmt.Errorf("cleaning virtual targets for %s: %w", eco, err))
					} else {
						for _, t := range virtualTargets {
							ecoFreed += t.SizeBytes
							ecoCount++
							ecoCleanedPaths = append(ecoCleanedPaths, t.Path)
						}
						ecoSuccess = true
					}
				}
			}
		}

		if ecoSuccess {
			res.TotalFreed += ecoFreed
			res.TotalCount += ecoCount
			res.CleanedPaths = append(res.CleanedPaths, ecoCleanedPaths...)
			res.Ecosystems = append(res.Ecosystems, eco)

			// Stage 5: Post-Delete Hooks
			if !opts.DryRun && c.hooks != nil {
				postData := hooks.HookData{
					Ecosystem:   eco,
					FreedSize:   humanSize(ecoFreed),
					TargetCount: ecoCount,
				}
				if err := c.hooks.RunPostDelete(ctx, eco, postData); err != nil {
					if opts.OnWarning != nil {
						opts.OnWarning(fmt.Sprintf("Post-delete hook warning for %s: %v", eco, err))
					}
				}
			}
		}
	}

	res.Duration = time.Since(startTime)

	// Stage 6: Audit & Scheduler Metrics Persistence
	if !opts.DryRun && res.TotalCount > 0 {
		mode := opts.Mode
		if mode == "" {
			if opts.Auto {
				mode = "auto"
			} else {
				mode = "force"
			}
		}

		if c.dataDir != "" {
			entry := history.Entry{
				Timestamp:  startTime,
				Targets:    res.TotalCount,
				Paths:      res.CleanedPaths,
				FreedBytes: res.TotalFreed,
				Ecosystems: res.Ecosystems,
				Duration:   res.Duration.Round(time.Millisecond),
				Mode:       mode,
			}
			if err := history.Append(c.dataDir, entry); err != nil {
				if opts.OnWarning != nil {
					opts.OnWarning(fmt.Sprintf("Warning: failed to record history: %v", err))
				}
			}
		}

		if opts.Auto && c.configDir != "" {
			runResult := scheduler.RunResult{
				Timestamp:    startTime,
				TargetsCount: res.TotalCount,
				BytesFreed:   res.TotalFreed,
				Duration:     res.Duration.Round(time.Millisecond).String(),
			}
			if err := scheduler.RecordRun(c.configDir, runResult); err != nil {
				if opts.OnWarning != nil {
					opts.OnWarning(fmt.Sprintf("Warning: failed to record scheduler run: %v", err))
				}
			}
		}
	}

	// Stage 6.1: Silent Trash Auto-Expiration & Quota Management (D-09, D-10)
	if !opts.DryRun && c.trashMgr != nil {
		_ = c.trashMgr.EnforceRetention(ctx)

		quotaBefore := c.trashMgr.QuotaStatus()
		_ = c.trashMgr.EnforceMaxSize(ctx)
		quotaAfter := c.trashMgr.QuotaStatus()

		if quotaBefore.Exceeded || quotaAfter.Exceeded {
			reportQuota := quotaBefore
			if !reportQuota.Exceeded {
				reportQuota = quotaAfter
			}
			if opts.OnWarning != nil {
				opts.OnWarning(fmt.Sprintf(
					"Aviso: Lixeira atingiu %s (limite configurado: %s). Lotes mais antigos foram expurgados.",
					humanSize(reportQuota.CurrentBytes), humanSize(reportQuota.MaxBytes),
				))
			}
		}
	}

	return res, nil
}

func humanSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}

	units := []string{"KB", "MB", "GB", "TB"}
	size := float64(bytes) / 1024
	i := 0
	for size >= 1024 && i < len(units)-1 {
		size /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", size, units[i])
}

func isSymlinkToBlacklist(path string, bl *safety.Blacklist) bool {
	if bl == nil {
		return false
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	return resolved != path && bl.IsBlacklisted(resolved)
}


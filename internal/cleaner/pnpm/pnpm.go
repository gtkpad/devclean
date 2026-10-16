package pnpm

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
)

type pnpmCleaner struct {
	exec cleaner.CommandExecutor
}

// New returns a new pnpm cleaner using the default command executor.
func New() domain.Cleaner {
	return NewWithExecutor(&cleaner.DefaultExecutor{})
}

// NewWithExecutor returns a new pnpm cleaner using the specified command executor.
func NewWithExecutor(exec cleaner.CommandExecutor) domain.Cleaner {
	return &pnpmCleaner{exec: exec}
}

func (c *pnpmCleaner) Name() string {
	return "pnpm"
}

func (c *pnpmCleaner) IsInstalled() bool {
	if _, err := c.exec.LookPath("pnpm"); err == nil {
		return true
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	candidates := []string{
		filepath.Join(home, "Library", "pnpm", "store"),
		filepath.Join(home, ".local", "share", "pnpm", "store"),
		filepath.Join(home, ".pnpm-store"),
		filepath.Join(home, "Library", "Caches", "pnpm"),
		filepath.Join(home, ".cache", "pnpm"),
	}
	for _, p := range candidates {
		if devfs.DirExists(p) {
			return true
		}
	}
	return false
}

func (c *pnpmCleaner) discoverStorePaths(ctx context.Context) []string {
	var paths []string
	seen := make(map[string]bool)

	out, err := c.exec.Output(ctx, "pnpm", "store", "path")
	if err == nil {
		cliPath := strings.TrimSpace(string(out))
		if cliPath != "" {
			resolved, resErr := filepath.EvalSymlinks(cliPath)
			if resErr != nil {
				resolved = cliPath
			}
			resolved = filepath.Clean(resolved)
			if devfs.DirExists(resolved) && !seen[resolved] {
				seen[resolved] = true
				paths = append(paths, resolved)
			}
		}
	}

	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		candidates := []string{
			filepath.Join(home, "Library", "pnpm", "store"),
			filepath.Join(home, ".local", "share", "pnpm", "store"),
			filepath.Join(home, ".pnpm-store"),
		}
		for _, cand := range candidates {
			resolved, resErr := filepath.EvalSymlinks(cand)
			if resErr != nil {
				resolved = cand
			}
			resolved = filepath.Clean(resolved)
			if devfs.DirExists(resolved) && !seen[resolved] {
				seen[resolved] = true
				paths = append(paths, resolved)
			}
		}
	}

	return paths
}

func (c *pnpmCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	var targets []domain.Target
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	// 1. Shared pnpm store discovery (D-10, D-11)
	for _, sPath := range c.discoverStorePaths(ctx) {
		info, err := os.Stat(sPath)
		if err != nil {
			continue
		}
		usage, _ := devfs.CalcUsage(sPath)
		size := usage.AllocatedBytes
		if size == 0 {
			size = usage.ApparentBytes
		}
		if size == 0 {
			continue
		}

		sum := sha256.Sum256([]byte(sPath))
		id := fmt.Sprintf("pnpm-store-%x", sum[:4])
		targets = append(targets, domain.Target{
			ID:            id,
			Name:          "pnpm store",
			Path:          sPath,
			SizeBytes:     size,
			EligibleBytes: 0, // D-11: 100% ReviewOnly
			TTLEligible:   false,
			ReviewOnly:    true,
			ReviewReason:  domain.ReviewReasonSharedStore,
			Risk:          domain.RiskLow,
			LastAccessed:  info.ModTime(),
			Category:      "pnpm-store",
			ProviderMetrics: &domain.ProviderMetrics{
				SizeBytes:      size,
				AllocatedBytes: usage.AllocatedBytes,
				ApparentBytes:  usage.ApparentBytes,
				TotalCount:     usage.FileCount,
				Partial:        usage.Partial,
				StatusNote:     "content-addressable shared package store",
			},
		})
	}

	// 2. Physical HTTP metadata cache (D-04)
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates := []string{
			filepath.Join(home, "Library", "Caches", "pnpm"),
			filepath.Join(home, ".cache", "pnpm"),
		}
		seen := make(map[string]bool)
		for _, cacheDir := range candidates {
			if seen[cacheDir] || !devfs.DirExists(cacheDir) {
				continue
			}
			seen[cacheDir] = true

			info, err := os.Stat(cacheDir)
			if err != nil {
				continue
			}
			size, _ := devfs.CalcDirSize(cacheDir)
			if size == 0 {
				continue
			}

			ttlEligible := false
			eligibleBytes := int64(0)
			if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
				ttlEligible = true
				eligibleBytes = size
			}

			sum := sha256.Sum256([]byte(cacheDir))
			targets = append(targets, domain.Target{
				ID:            fmt.Sprintf("%x", sum[:6]),
				Name:          "pnpm metadata cache",
				Path:          cacheDir,
				SizeBytes:     size,
				EligibleBytes: eligibleBytes,
				TTLEligible:   ttlEligible,
				Risk:          domain.RiskLow,
				LastAccessed:  info.ModTime(),
				Category:      "pnpm-cache",
			})
		}
	}

	return targets, nil
}

func (c *pnpmCleaner) Clean(_ context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if t.ReviewOnly || t.Category == "pnpm-store" || t.Category == "virtual-pnpm-store" || strings.HasPrefix(t.Path, "pnpm://") {
			continue // INVIOLABLE: never delete or prune shared pnpm store (D-11, D-12)
		}

		if safety.IsCoreBlacklisted(t.Path) {
			continue
		}
		if isLink, err := safety.IsSymlink(t.Path); err == nil && isLink {
			continue
		}
		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}

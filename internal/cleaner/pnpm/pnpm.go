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

func (c *pnpmCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	var targets []domain.Target
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	// 1. Virtual pnpm store prune target
	out, err := c.exec.Output(ctx, "pnpm", "store", "path")
	if err == nil {
		storePath := strings.TrimSpace(string(out))
		if storePath != "" {
			size, _ := devfs.CalcDirSize(storePath)
			if size > 0 {
				targets = append(targets, domain.Target{
					ID:            "pnpm-store-prune",
					Name:          "pnpm store (unreferenced packages)",
					Path:          "pnpm://store/prune",
					SizeBytes:     size,
					EligibleBytes: size,
					TTLEligible:   true,
					Risk:          domain.RiskLow,
					Category:      "virtual-pnpm-store",
				})
			}
		}
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

func (c *pnpmCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if t.Path == "pnpm://store/prune" {
			if _, err := c.exec.Output(ctx, "pnpm", "store", "prune"); err != nil {
				return fmt.Errorf("pnpm store prune failed: %w", err)
			}
			continue
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

package homebrew

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
)

type homebrewCleaner struct {
	exec cleaner.CommandExecutor
}

// New creates a new homebrewCleaner with the default command executor.
func New() domain.Cleaner {
	return NewWithExecutor(&cleaner.DefaultExecutor{})
}

// NewWithExecutor creates a new homebrewCleaner with a custom command executor.
func NewWithExecutor(exec cleaner.CommandExecutor) domain.Cleaner {
	return &homebrewCleaner{exec: exec}
}

// Name returns the identifier of the cleaner.
func (c *homebrewCleaner) Name() string {
	return "homebrew"
}

// IsInstalled checks if the brew CLI is available in PATH or its default cache exists on disk.
func (c *homebrewCleaner) IsInstalled() bool {
	if _, err := c.exec.LookPath("brew"); err == nil {
		return true
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	candidates := []string{
		filepath.Join(home, "Library", "Caches", "Homebrew"),
		filepath.Join(home, ".cache", "Homebrew"),
	}
	for _, p := range candidates {
		if devfs.DirExists(p) {
			return true
		}
	}
	return false
}


// Scan queries brew --cache and returns a virtual cleanup target if the cache size > 0.
func (c *homebrewCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	out, err := c.exec.Output(ctx, "brew", "--cache")
	if err != nil {
		return nil, nil
	}

	cacheDir := strings.TrimSpace(string(out))
	if cacheDir == "" {
		return nil, nil
	}

	if isExcluded(cacheDir, rules.ExcludePaths) || isExcluded("brew://cleanup", rules.ExcludePaths) {
		return nil, nil
	}

	info, err := os.Stat(cacheDir)
	if err != nil || !info.IsDir() {
		return nil, nil
	}

	size, _ := devfs.CalcDirSize(cacheDir)
	if size == 0 {
		return nil, nil
	}

	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)
	ttlEligible := false
	eligibleBytes := int64(0)
	if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
		ttlEligible = true
		eligibleBytes = size
	}

	return []domain.Target{
		{
			ID:            "homebrew-cleanup-cache",
			Name:          "Homebrew cache (stale bottles & downloads)",
			Path:          "brew://cleanup",
			SizeBytes:     size,
			EligibleBytes: eligibleBytes,
			TTLEligible:   ttlEligible,
			Risk:          domain.RiskLow,
			LastAccessed:  info.ModTime(),
			Category:      "virtual-homebrew-cache",
		},
	}, nil
}

// Clean executes brew cleanup -s for virtual targets, with fallback to physical cache cleanup.
func (c *homebrewCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if t.Path == "brew://cleanup" {
			if _, err := c.exec.Output(ctx, "brew", "cleanup", "-s"); err != nil {
				// Fallback seguro: remover diretamente os arquivos de cache físico (D-11)
				out, errCache := c.exec.Output(ctx, "brew", "--cache")
				if errCache != nil {
					return fmt.Errorf("brew cleanup failed: %w (cache fallback lookup failed: %v)", err, errCache)
				}
				cacheDir := strings.TrimSpace(string(out))
				if cacheDir == "" || !filepath.IsAbs(cacheDir) || cacheDir == "/" || strings.Contains(cacheDir, "Cellar") {
					return fmt.Errorf("brew cleanup failed: %w (invalid or unsafe fallback cache path: %q)", err, cacheDir)
				}
				if errRemove := os.RemoveAll(cacheDir); errRemove != nil {
					return fmt.Errorf("brew cleanup fallback removal failed: %w", errRemove)
				}
				continue
			}
			continue
		}

		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}

func isExcluded(path string, excludes []string) bool {
	base := filepath.Base(path)
	for _, exc := range excludes {
		if strings.HasPrefix(path, exc) {
			return true
		}
		if matched, _ := doublestar.Match(exc, path); matched {
			return true
		}
		if matched, _ := doublestar.Match(exc, base); matched {
			return true
		}
	}
	return false
}

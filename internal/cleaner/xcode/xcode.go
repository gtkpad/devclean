package xcode

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
)

type xcodeCleaner struct {
	exec    cleaner.CommandExecutor
	baseDir string
	goos    string
}

// New creates a new xcodeCleaner with default executor and home directory.
func New() domain.Cleaner {
	return NewWithExecutor(&cleaner.DefaultExecutor{})
}

// NewWithExecutor creates a new xcodeCleaner with a custom command executor.
func NewWithExecutor(exec cleaner.CommandExecutor) domain.Cleaner {
	baseDir := ""
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		baseDir = filepath.Join(home, "Library", "Developer", "Xcode")
	}
	return &xcodeCleaner{
		exec:    exec,
		baseDir: baseDir,
		goos:    runtime.GOOS,
	}
}

// NewWithDirAndExecutor creates a new xcodeCleaner with explicit baseDir and executor for testing.
func NewWithDirAndExecutor(baseDir string, exec cleaner.CommandExecutor) domain.Cleaner {
	return &xcodeCleaner{
		exec:    exec,
		baseDir: baseDir,
		goos:    runtime.GOOS,
	}
}

// NewWithDirExecutorAndOS creates a new xcodeCleaner allowing OS override for testing.
func NewWithDirExecutorAndOS(baseDir string, exec cleaner.CommandExecutor, goos string) domain.Cleaner {
	return &xcodeCleaner{
		exec:    exec,
		baseDir: baseDir,
		goos:    goos,
	}
}

func (c *xcodeCleaner) getOS() string {
	if c.goos != "" {
		return c.goos
	}
	return runtime.GOOS
}

// Name returns the identifier of the cleaner.
func (c *xcodeCleaner) Name() string {
	return "xcode"
}

// IsInstalled checks if running on Darwin and xcrun is available in PATH or DerivedData exists on disk.
func (c *xcodeCleaner) IsInstalled() bool {
	if c.getOS() != "darwin" || c.baseDir == "" {
		return false
	}
	if _, err := c.exec.LookPath("xcrun"); err == nil {
		return true
	}
	return devfs.DirExists(c.baseDir)
}


// Scan looks for individual project directories in DerivedData and emits virtual simulator target.
func (c *xcodeCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	if c.getOS() != "darwin" {
		return nil, nil
	}

	var targets []domain.Target
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	// 1. DerivedData segmentado por projeto (D-09)
	derivedDataDir := filepath.Join(c.baseDir, "DerivedData")
	entries, err := os.ReadDir(derivedDataDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() || e.Name() == "ModuleCache.noindex" {
				continue
			}

			projDir := filepath.Join(derivedDataDir, e.Name())

			if isExcluded(projDir, rules.ExcludePaths) {
				continue
			}

			info, err := e.Info()
			if err != nil {
				continue
			}

			size, _ := devfs.CalcDirSize(projDir)
			if size > 0 {
				eligibleBytes := int64(0)
				ttlEligible := false
				if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
					eligibleBytes = size
					ttlEligible = true
				}

				sum := sha256.Sum256([]byte(projDir))
				targets = append(targets, domain.Target{
					ID:            fmt.Sprintf("%x", sum[:6]),
					Name:          "Xcode DerivedData: " + e.Name(),
					Path:          projDir,
					SizeBytes:     size,
					EligibleBytes: eligibleBytes,
					TTLEligible:   ttlEligible,
					Risk:          domain.RiskLow,
					LastAccessed:  info.ModTime(),
					Category:      "xcode-derived-data",
				})
			}
		}
	}

	// 2. Alvo virtual para simuladores iOS indisponíveis (D-10)
	targets = append(targets, domain.Target{
		ID:            "xcode-simulators-unavailable",
		Name:          "iOS Simulators (unavailable runtimes)",
		Path:          "xcode://simulators/unavailable",
		SizeBytes:     0,
		EligibleBytes: 0,
		TTLEligible:   true,
		Risk:          domain.RiskLow,
		Category:      "virtual-xcode-simulators",
	})

	return targets, nil
}

// Clean removes physical DerivedData directories and triggers simctl for virtual targets.
func (c *xcodeCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if t.Path == "xcode://simulators/unavailable" {
			if _, err := c.exec.Output(ctx, "xcrun", "simctl", "delete", "unavailable"); err != nil {
				return fmt.Errorf("failed to delete unavailable simulators: %w", err)
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

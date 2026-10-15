package rust

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
)

type rustCleaner struct {
	scanPaths []string
}

// New creates a Rust ecosystem cleaner that discovers local target/ directories and global cargo caches.
func New(scanPaths []string) domain.Cleaner {
	return &rustCleaner{scanPaths: scanPaths}
}

func (c *rustCleaner) Name() string { return "rust" }

func (c *rustCleaner) IsInstalled() bool {
	if _, err := os.Stat(filepath.Join(cargoHome(), "bin")); err == nil {
		return true
	}
	// Fallback: check PATH for cargo binary
	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		if _, err := os.Stat(filepath.Join(dir, "cargo")); err == nil {
			return true
		}
	}
	return false
}

func (c *rustCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	var targets []domain.Target

	// 1. Discover local target/ directories with Cargo.toml sibling
	localTargets, err := c.scanLocalTargets(rules)
	if err != nil {
		return nil, err
	}
	targets = append(targets, localTargets...)

	// 2. Discover global cargo caches
	globalTargets := c.scanGlobalCaches(rules)
	targets = append(targets, globalTargets...)

	return targets, nil
}

func (c *rustCleaner) Clean(_ context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}

func (c *rustCleaner) scanLocalTargets(rules domain.EcosystemRules) ([]domain.Target, error) {
	var (
		mu       sync.Mutex
		allPaths []string
		firstErr error
		wg       sync.WaitGroup
		sem      = make(chan struct{}, runtime.NumCPU())
	)

	for _, root := range c.scanPaths {
		sem <- struct{}{}
		wg.Add(1)
		go func(r string) {
			defer wg.Done()
			defer func() { <-sem }()

			paths, err := walkForRustTargets(r, rules)
			mu.Lock()
			defer mu.Unlock()
			if err != nil && firstErr == nil {
				firstErr = err
			}
			allPaths = append(allPaths, paths...)
		}(root)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	if len(allPaths) == 0 {
		return nil, nil
	}

	sizes, err := devfs.CalcDirSizes(context.Background(), allPaths)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)
	targets := make([]domain.Target, 0, len(allPaths))
	for _, p := range allPaths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}

		parentDir := filepath.Dir(p)
		manifests := []string{
			filepath.Join(parentDir, "Cargo.toml"),
			filepath.Join(parentDir, "Cargo.lock"),
		}

		hash := sha256.Sum256([]byte(p))
		id := fmt.Sprintf("%x", hash[:6])

		size := sizes[p]
		ttlEligible := false
		eligibleBytes := int64(0)
		if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
			ttlEligible = true
			eligibleBytes = size
		}

		targets = append(targets, domain.Target{
			ID:            id,
			Name:          filepath.Base(parentDir) + "/target",
			Path:          p,
			SizeBytes:     size,
			EligibleBytes: eligibleBytes,
			TTLEligible:   ttlEligible,
			Risk:          cleaner.ClassifyLocalArtifactRisk(p, manifests, rules.MaxAgeDays),
			LastAccessed:  info.ModTime(),
			Category:      "cargo-target",
		})
	}

	return targets, nil
}

func (c *rustCleaner) scanGlobalCaches(rules domain.EcosystemRules) []domain.Target {
	home := cargoHome()
	var targets []domain.Target
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	caches := []struct {
		name     string
		subdir   string
		category string
	}{
		{"cargo registry", "registry", "cargo-registry"},
		{"cargo git", "git", "cargo-git"},
	}

	for _, cache := range caches {
		path := filepath.Join(home, cache.subdir)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		size, _ := devfs.CalcDirSize(path)
		hash := sha256.Sum256([]byte(path))
		id := fmt.Sprintf("%x", hash[:6])

		ttlEligible := false
		eligibleBytes := int64(0)
		if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
			ttlEligible = true
			eligibleBytes = size
		}

		targets = append(targets, domain.Target{
			ID:            id,
			Name:          cache.name,
			Path:          path,
			SizeBytes:     size,
			EligibleBytes: eligibleBytes,
			TTLEligible:   ttlEligible,
			Risk:          domain.RiskLow,
			LastAccessed:  info.ModTime(),
			Category:      cache.category,
		})
	}

	return targets
}

func walkForRustTargets(root string, rules domain.EcosystemRules) ([]string, error) {
	var found []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			return nil
		}

		if !d.IsDir() {
			return nil
		}

		if d.Type()&os.ModeSymlink != 0 {
			return filepath.SkipDir
		}
		isLink, linkErr := safety.IsSymlink(path)
		if linkErr == nil && isLink {
			return filepath.SkipDir
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr == nil {
			for _, pattern := range rules.ExcludePaths {
				matched, _ := doublestar.Match(pattern, rel)
				if matched {
					return filepath.SkipDir
				}
			}
		}

		if d.Name() == "target" {
			parentDir := filepath.Dir(path)
			// Only claim target/ if Cargo.toml is a sibling
			if _, err := os.Stat(filepath.Join(parentDir, "Cargo.toml")); err == nil {
				found = append(found, path)
			}
			return filepath.SkipDir
		}

		return nil
	})

	return found, err
}

func cargoHome() string {
	if v := os.Getenv("CARGO_HOME"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cargo")
}

package elixir

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
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


type elixirCleaner struct {
	scanPaths []string
}

type elixirFoundTarget struct {
	path     string
	category string
	risk     domain.RiskLevel
}

// New creates an Elixir/Mix ecosystem cleaner implementing domain.Cleaner.
func New(scanPaths []string) domain.Cleaner {
	return &elixirCleaner{scanPaths: scanPaths}
}

// Name returns the ecosystem identifier for Elixir.
func (c *elixirCleaner) Name() string {
	return "elixir"
}

// IsInstalled checks if mix, elixir, or iex toolchains are in PATH,
// if any mix.exs exists in scanPaths, or if the global Hex cache exists.
func (c *elixirCleaner) IsInstalled() bool {
	// 1. Toolchains in PATH
	for _, bin := range []string{"mix", "elixir", "iex"} {
		if _, err := exec.LookPath(bin); err == nil {
			return true
		}
	}

	// 2. Presence of mix.exs in scanPaths (shallow D-16)
	if cleaner.HasShallowManifest(c.scanPaths, "mix.exs") {
		return true
	}

	// 3. Global Hex packages cache

	for _, dir := range globalHexCacheCandidateDirs() {
		if _, err := os.Stat(dir); err == nil {
			return true
		}
	}

	return false
}

// Scan discovers local _build and deps directories conditioned on sibling mix.exs,
// as well as global Hex package caches.
func (c *elixirCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	var targets []domain.Target

	localTargets, err := c.scanLocalTargets(ctx, rules)
	if err != nil {
		return nil, err
	}
	targets = append(targets, localTargets...)

	globalTargets := c.scanGlobalCaches(rules)
	targets = append(targets, globalTargets...)

	return targets, nil
}

// Clean removes targets using os.RemoveAll.
func (c *elixirCleaner) Clean(_ context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}

func (c *elixirCleaner) scanLocalTargets(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	var (
		mu       sync.Mutex
		allFound []elixirFoundTarget
		firstErr error
		wg       sync.WaitGroup
		sem      = make(chan struct{}, runtime.NumCPU())
	)

	for _, root := range c.scanPaths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(r string) {
			defer wg.Done()
			defer func() { <-sem }()

			found, err := walkForElixirTargets(ctx, r, rules)
			mu.Lock()
			defer mu.Unlock()
			if err != nil && firstErr == nil {
				firstErr = err
			}
			allFound = append(allFound, found...)
		}(root)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	if len(allFound) == 0 {
		return nil, nil
	}

	// Deduplicate by path
	uniqueFound := make([]elixirFoundTarget, 0, len(allFound))
	seen := make(map[string]bool, len(allFound))
	paths := make([]string, 0, len(allFound))
	for _, f := range allFound {
		if !seen[f.path] {
			seen[f.path] = true
			uniqueFound = append(uniqueFound, f)
			paths = append(paths, f.path)
		}
	}

	sizes, err := devfs.CalcDirSizes(ctx, paths)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)
	targets := make([]domain.Target, 0, len(uniqueFound))
	for _, f := range uniqueFound {
		info, err := os.Stat(f.path)
		if err != nil {
			continue
		}

		parentDir := filepath.Dir(f.path)
		base := filepath.Base(f.path)

		hash := sha256.Sum256([]byte(f.path))
		id := fmt.Sprintf("%x", hash[:6])

		size := sizes[f.path]
		ttlEligible := rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff)
		eligibleBytes := int64(0)
		if ttlEligible {
			eligibleBytes = size
		}

		targets = append(targets, domain.Target{
			ID:            id,
			Name:          filepath.Base(parentDir) + "/" + base,
			Path:          f.path,
			SizeBytes:     size,
			EligibleBytes: eligibleBytes,
			Risk:          f.risk,
			LastAccessed:  info.ModTime(),
			TTLEligible:   ttlEligible,
			Category:      f.category,
		})
	}

	return targets, nil
}

func (c *elixirCleaner) scanGlobalCaches(rules domain.EcosystemRules) []domain.Target {
	var targets []domain.Target
	seen := make(map[string]bool)
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	for _, p := range globalHexCacheCandidateDirs() {
		if seen[p] {
			continue
		}
		seen[p] = true

		if info, err := os.Stat(p); err == nil {
			size, _ := devfs.CalcDirSize(p)
			hash := sha256.Sum256([]byte(p))
			id := fmt.Sprintf("%x", hash[:6])

			ttlEligible := rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff)
			eligibleBytes := int64(0)
			if ttlEligible {
				eligibleBytes = size
			}

			targets = append(targets, domain.Target{
				ID:            id,
				Name:          "Hex Packages Cache",
				Path:          p,
				SizeBytes:     size,
				EligibleBytes: eligibleBytes,
				Risk:          domain.RiskLow,
				LastAccessed:  info.ModTime(),
				TTLEligible:   ttlEligible,
				Category:      "hex-packages",
			})
		}
	}

	return targets
}

func walkForElixirTargets(ctx context.Context, root string, rules domain.EcosystemRules) ([]elixirFoundTarget, error) {
	var found []elixirFoundTarget

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			return nil
		}

		if err := ctx.Err(); err != nil {
			return err
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

		if d.Name() == ".git" {
			return filepath.SkipDir
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr == nil && rel != "." {
			for _, pattern := range rules.ExcludePaths {
				matched, _ := doublestar.Match(pattern, rel)
				if matched {
					return filepath.SkipDir
				}
			}
		}

		name := d.Name()
		if name == "_build" || name == "deps" {
			parentDir := filepath.Dir(path)
			mixExs := filepath.Join(parentDir, "mix.exs")
			if _, err := os.Stat(mixExs); err == nil {
				category := "elixir-build"
				risk := domain.RiskLow
				if name == "deps" {
					category = "elixir-deps"
					risk = domain.RiskMedium
				}

				found = append(found, elixirFoundTarget{
					path:     path,
					category: category,
					risk:     risk,
				})
				return filepath.SkipDir
			}

			return nil
		}

		return nil
	})

	return found, err
}

func globalHexCacheCandidateDirs() []string {
	var candidateDirs []string
	home, _ := os.UserHomeDir()

	if hexHome := os.Getenv("HEX_HOME"); hexHome != "" {
		candidateDirs = append(candidateDirs, filepath.Join(hexHome, "packages"))
	}

	if home != "" {
		candidateDirs = append(candidateDirs, filepath.Join(home, ".hex", "packages"))
	}

	return candidateDirs
}


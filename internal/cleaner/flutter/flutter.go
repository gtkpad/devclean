package flutter

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
	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
)

type flutterCleaner struct {
	scanPaths []string
}

type flutterFoundTarget struct {
	path     string
	category string
	risk     domain.RiskLevel
}

// New creates a Flutter/Dart ecosystem cleaner implementing domain.Cleaner.
func New(scanPaths []string) domain.Cleaner {
	return &flutterCleaner{scanPaths: scanPaths}
}

// Name returns the ecosystem identifier for Flutter/Dart.
func (c *flutterCleaner) Name() string {
	return "flutter"
}

// IsInstalled checks if flutter or dart toolchains are in PATH,
// or if the pub cache directory exists.
func (c *flutterCleaner) IsInstalled() bool {
	for _, bin := range []string{"flutter", "dart"} {
		if _, err := exec.LookPath(bin); err == nil {
			return true
		}
	}

	home, _ := os.UserHomeDir()
	candidateDirs := []string{
		filepath.Join(home, ".pub-cache"),
	}
	if v := os.Getenv("PUB_CACHE"); v != "" {
		candidateDirs = append(candidateDirs, v)
	}

	for _, dir := range candidateDirs {
		if _, err := os.Stat(dir); err == nil {
			return true
		}
	}

	return false
}

// Scan discovers local .dart_tool and build directories belonging to Flutter/Dart projects,
// as well as global Pub package caches.
func (c *flutterCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
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
func (c *flutterCleaner) Clean(_ context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}

func (c *flutterCleaner) scanLocalTargets(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	var (
		mu       sync.Mutex
		allFound []flutterFoundTarget
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

			found, err := walkForFlutterTargets(ctx, r, rules)
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
	uniqueFound := make([]flutterFoundTarget, 0, len(allFound))
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
		ttlEligible := false
		eligibleBytes := int64(0)
		if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
			ttlEligible = true
			eligibleBytes = size
		}

		targets = append(targets, domain.Target{
			ID:            id,
			Name:          filepath.Base(parentDir) + "/" + base,
			Path:          f.path,
			SizeBytes:     size,
			EligibleBytes: eligibleBytes,
			TTLEligible:   ttlEligible,
			Risk:          f.risk,
			LastAccessed:  info.ModTime(),
			Category:      f.category,
		})
	}

	return targets, nil
}

func (c *flutterCleaner) scanGlobalCaches(rules domain.EcosystemRules) []domain.Target {
	var targets []domain.Target
	seen := make(map[string]bool)
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	home, _ := os.UserHomeDir()

	var pubCachePaths []string
	if v := os.Getenv("PUB_CACHE"); v != "" {
		pubCachePaths = append(pubCachePaths, v)
	}
	if home != "" {
		pubCachePaths = append(pubCachePaths, filepath.Join(home, ".pub-cache"))
	}

	for _, p := range pubCachePaths {
		if seen[p] {
			continue
		}
		seen[p] = true

		if info, err := os.Stat(p); err == nil {
			size, _ := devfs.CalcDirSize(p)
			hash := sha256.Sum256([]byte(p))
			id := fmt.Sprintf("%x", hash[:6])

			ttlEligible := false
			eligibleBytes := int64(0)
			if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
				ttlEligible = true
				eligibleBytes = size
			}

			targets = append(targets, domain.Target{
				ID:            id,
				Name:          "Pub Cache",
				Path:          p,
				SizeBytes:     size,
				EligibleBytes: eligibleBytes,
				TTLEligible:   ttlEligible,
				Risk:          domain.RiskLow,
				LastAccessed:  info.ModTime(),
				Category:      "pub-cache",
			})
		}
	}

	return targets
}

func walkForFlutterTargets(ctx context.Context, root string, rules domain.EcosystemRules) ([]flutterFoundTarget, error) {
	var found []flutterFoundTarget

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
		if name == ".dart_tool" || name == "build" {
			parentDir := filepath.Dir(path)
			pubspec := filepath.Join(parentDir, "pubspec.yaml")
			if _, err := os.Stat(pubspec); err == nil {
				category := "flutter-dart-tool"
				risk := domain.RiskLow
				if name == "build" {
					category = "flutter-build"
					risk = domain.RiskMedium
				}

				found = append(found, flutterFoundTarget{
					path:     path,
					category: category,
					risk:     risk,
				})
				return filepath.SkipDir
			}

			if name == ".dart_tool" {
				return filepath.SkipDir
			}
			return nil
		}

		return nil
	})

	return found, err
}

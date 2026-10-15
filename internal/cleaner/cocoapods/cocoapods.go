package cocoapods

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

type cocoapodsCleaner struct {
	scanPaths []string
}

// New creates a CocoaPods ecosystem cleaner implementing domain.Cleaner.
func New(scanPaths []string) domain.Cleaner {
	return &cocoapodsCleaner{scanPaths: scanPaths}
}

// Name returns the ecosystem identifier for CocoaPods.
func (c *cocoapodsCleaner) Name() string {
	return "cocoapods"
}

// IsInstalled checks if the pod executable is available in PATH,
// if any Podfile exists in scanPaths, or if the global CocoaPods cache exists.
func (c *cocoapodsCleaner) IsInstalled() bool {
	// 1. Toolchain in PATH
	if _, err := exec.LookPath("pod"); err == nil {
		return true
	}

	// 2. Presence of Podfile in scanPaths
	for _, root := range c.scanPaths {
		if hasAnyPodfiles(root) {
			return true
		}
	}

	// 3. Global cache directories
	for _, dir := range globalCacheCandidateDirs() {
		if _, err := os.Stat(dir); err == nil {
			return true
		}
	}

	return false
}

// Scan discovers local Pods directories conditioned on sibling Podfile,
// as well as global CocoaPods caches.
func (c *cocoapodsCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
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
func (c *cocoapodsCleaner) Clean(_ context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}

func (c *cocoapodsCleaner) scanLocalTargets(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	var (
		mu       sync.Mutex
		allPaths []string
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

			paths, err := walkForCocoaPodsTargets(ctx, r, rules)
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

	// Deduplicate by path
	uniquePaths := make([]string, 0, len(allPaths))
	seen := make(map[string]bool, len(allPaths))
	for _, p := range allPaths {
		if !seen[p] {
			seen[p] = true
			uniquePaths = append(uniquePaths, p)
		}
	}

	sizes, err := devfs.CalcDirSizes(ctx, uniquePaths)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)
	targets := make([]domain.Target, 0, len(uniquePaths))
	for _, p := range uniquePaths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}

		parentDir := filepath.Dir(p)
		base := filepath.Base(p)

		hash := sha256.Sum256([]byte(p))
		id := fmt.Sprintf("%x", hash[:6])

		size := sizes[p]
		ttlEligible := rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff)
		eligibleBytes := int64(0)
		if ttlEligible {
			eligibleBytes = size
		}

		targets = append(targets, domain.Target{
			ID:            id,
			Name:          filepath.Base(parentDir) + "/" + base,
			Path:          p,
			SizeBytes:     size,
			EligibleBytes: eligibleBytes,
			Risk:          domain.RiskMedium,
			LastAccessed:  info.ModTime(),
			TTLEligible:   ttlEligible,
			Category:      "cocoapods-pods",
		})
	}

	return targets, nil
}

func (c *cocoapodsCleaner) scanGlobalCaches(rules domain.EcosystemRules) []domain.Target {
	var targets []domain.Target
	seen := make(map[string]bool)
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	for _, p := range globalCacheCandidateDirs() {
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
				Name:          "CocoaPods Cache",
				Path:          p,
				SizeBytes:     size,
				EligibleBytes: eligibleBytes,
				Risk:          domain.RiskLow,
				LastAccessed:  info.ModTime(),
				TTLEligible:   ttlEligible,
				Category:      "cocoapods-cache",
			})
		}
	}

	return targets
}

func walkForCocoaPodsTargets(ctx context.Context, root string, rules domain.EcosystemRules) ([]string, error) {
	var found []string

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

		if d.Name() == "Pods" {
			parentDir := filepath.Dir(path)
			podfile := filepath.Join(parentDir, "Podfile")
			if _, err := os.Stat(podfile); err == nil {
				found = append(found, path)
				return filepath.SkipDir
			}
			return nil
		}

		return nil
	})

	return found, err
}

func globalCacheCandidateDirs() []string {
	var candidateDirs []string
	home, _ := os.UserHomeDir()

	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		candidateDirs = append(candidateDirs, filepath.Join(xdg, "CocoaPods"))
	}

	if home != "" {
		if runtime.GOOS == "darwin" {
			candidateDirs = append(candidateDirs, filepath.Join(home, "Library", "Caches", "CocoaPods"))
		}
		candidateDirs = append(candidateDirs, filepath.Join(home, ".cache", "CocoaPods"))
	}

	return candidateDirs
}

func hasAnyPodfiles(root string) bool {
	found := false
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "Podfile" {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}

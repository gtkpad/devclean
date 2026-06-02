package node

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
	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
)

type nodeCleaner struct {
	scanPaths []string
}

// New creates a Node.js ecosystem cleaner that recursively scans for node_modules.
func New(scanPaths []string) domain.Cleaner {
	return &nodeCleaner{scanPaths: scanPaths}
}

func (c *nodeCleaner) Name() string { return "node" }

func (c *nodeCleaner) IsInstalled() bool { return true }

func (c *nodeCleaner) Scan(_ context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
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

			paths, err := walkScanPath(r, rules)
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

	targets := make([]domain.Target, 0, len(allPaths))
	for _, p := range allPaths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}

		parentName := filepath.Base(filepath.Dir(p))
		hash := sha256.Sum256([]byte(p))
		id := fmt.Sprintf("%x", hash[:6])

		targets = append(targets, domain.Target{
			ID:           id,
			Name:         parentName + "/node_modules",
			Path:         p,
			SizeBytes:    sizes[p],
			Risk:         classifyRisk(p, rules.MaxAgeDays),
			LastAccessed: info.ModTime(),
			Category:     "node",
		})
	}

	return targets, nil
}

func (c *nodeCleaner) Clean(_ context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}

func walkScanPath(root string, rules domain.EcosystemRules) ([]string, error) {
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

		// Check symlinks — WalkDir may not report symlink info for directories on all platforms
		if d.Type()&os.ModeSymlink != 0 {
			return filepath.SkipDir
		}
		isLink, linkErr := safety.IsSymlink(path)
		if linkErr == nil && isLink {
			return filepath.SkipDir
		}

		// Check exclude globs using relative path from root
		rel, relErr := filepath.Rel(root, path)
		if relErr == nil {
			for _, pattern := range rules.ExcludePaths {
				matched, _ := doublestar.Match(pattern, rel)
				if matched {
					return filepath.SkipDir
				}
			}
		}

		// Short-circuit: found node_modules — collect and don't descend
		if d.Name() == "node_modules" {
			found = append(found, path)
			return filepath.SkipDir
		}

		return nil
	})

	return found, err
}

func classifyRisk(nodeModulesPath string, maxAgeDays int) domain.RiskLevel {
	info, err := os.Stat(nodeModulesPath)
	if err != nil {
		return domain.RiskLow
	}

	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)
	nodeModulesOld := info.ModTime().Before(cutoff)

	// Check parent directory for lock files
	parentDir := filepath.Dir(nodeModulesPath)
	lockFiles := []string{"package-lock.json", "yarn.lock", "pnpm-lock.yaml"}

	lockFileRecent := false
	for _, lf := range lockFiles {
		lfPath := filepath.Join(parentDir, lf)
		lfInfo, lfErr := os.Stat(lfPath)
		if lfErr == nil && lfInfo.ModTime().After(cutoff) {
			lockFileRecent = true
			break
		}
	}

	switch {
	case !nodeModulesOld:
		return domain.RiskHigh
	case nodeModulesOld && lockFileRecent:
		return domain.RiskMedium
	default:
		return domain.RiskLow
	}
}

package dotnet

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/safety"
)

// projectManifests are the files that indicate a .NET project root.
var projectManifests = []string{".csproj", ".fsproj", ".sln"}

type dotnetCleaner struct {
	paths     platform.Paths
	scanPaths []string
}

// New creates a .NET ecosystem cleaner.
func New(paths platform.Paths, scanPaths []string) domain.Cleaner {
	return &dotnetCleaner{paths: paths, scanPaths: scanPaths}
}

func (c *dotnetCleaner) Name() string { return "dotnet" }

func (c *dotnetCleaner) IsInstalled() bool {
	_, err := exec.LookPath("dotnet")
	return err == nil
}

func (c *dotnetCleaner) Scan(rules domain.EcosystemRules) ([]domain.Target, error) {
	targets, err := c.scanGlobalCaches(rules)
	if err != nil {
		return nil, err
	}

	localTargets, err := c.scanLocalBuildOutputs(rules)
	if err != nil {
		return nil, err
	}

	targets = append(targets, localTargets...)
	return targets, nil
}

func (c *dotnetCleaner) scanGlobalCaches(rules domain.EcosystemRules) ([]domain.Target, error) {
	type cacheDir struct {
		path     string
		category string
	}

	dirs := []cacheDir{
		{c.paths.DotNetNuGet, "nuget-packages"},
		{c.paths.DotNetHTTP, "nuget-http-cache"},
		{c.paths.DotNetTemp, "nuget-temp"},
	}

	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)
	var targets []domain.Target

	for _, d := range dirs {
		if d.path == "" {
			continue
		}
		if isExcluded(d.path, rules.ExcludePaths) {
			continue
		}

		info, err := os.Stat(d.path)
		if err != nil {
			continue
		}

		lastAccessed := info.ModTime()
		if rules.MaxAgeDays > 0 && lastAccessed.After(cutoff) {
			continue
		}

		size, err := devfs.CalcDirSize(d.path)
		if err != nil || size == 0 {
			continue
		}

		targets = append(targets, domain.Target{
			ID:           d.category + ":" + d.path,
			Name:         d.category,
			Path:         d.path,
			SizeBytes:    size,
			Risk:         domain.RiskLow,
			LastAccessed: lastAccessed,
			Category:     d.category,
		})
	}

	return targets, nil
}

func (c *dotnetCleaner) scanLocalBuildOutputs(rules domain.EcosystemRules) ([]domain.Target, error) {
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

			paths, err := walkForBuildOutputs(r, rules)
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

		dirName := filepath.Base(p)
		projectName := filepath.Base(filepath.Dir(p))
		hash := sha256.Sum256([]byte(p))
		id := fmt.Sprintf("%x", hash[:6])

		targets = append(targets, domain.Target{
			ID:           id,
			Name:         projectName + "/" + dirName,
			Path:         p,
			SizeBytes:    sizes[p],
			Risk:         domain.RiskLow,
			LastAccessed: info.ModTime(),
			Category:     "dotnet-build",
		})
	}

	return targets, nil
}

func (c *dotnetCleaner) Clean(targets []domain.Target) error {
	for _, t := range targets {
		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}

// walkForBuildOutputs walks a root directory looking for bin/ and obj/ dirs
// that are adjacent to a .NET project manifest (.csproj, .fsproj, .sln).
func walkForBuildOutputs(root string, rules domain.EcosystemRules) ([]string, error) {
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

		// Skip symlinks
		if d.Type()&os.ModeSymlink != 0 {
			return filepath.SkipDir
		}
		isLink, linkErr := safety.IsSymlink(path)
		if linkErr == nil && isLink {
			return filepath.SkipDir
		}

		// Check exclude globs
		rel, relErr := filepath.Rel(root, path)
		if relErr == nil {
			for _, pattern := range rules.ExcludePaths {
				matched, _ := doublestar.Match(pattern, rel)
				if matched {
					return filepath.SkipDir
				}
			}
		}

		// Don't descend into node_modules or .git
		name := d.Name()
		if name == "node_modules" || name == ".git" {
			return filepath.SkipDir
		}

		// Check for bin/ or obj/ with adjacent project manifest
		if name == "bin" || name == "obj" {
			parentDir := filepath.Dir(path)
			if hasProjectManifest(parentDir) {
				found = append(found, path)
			}
			return filepath.SkipDir
		}

		return nil
	})

	return found, err
}

// hasProjectManifest checks if a directory contains a .NET project file.
func hasProjectManifest(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		for _, manifest := range projectManifests {
			if strings.EqualFold(ext, manifest) {
				return true
			}
		}
	}
	return false
}

func isExcluded(path string, excludes []string) bool {
	for _, exc := range excludes {
		if strings.HasPrefix(path, exc) {
			return true
		}
	}
	return false
}

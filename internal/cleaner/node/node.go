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
	"github.com/gtkpad/devclean/internal/cleaner"
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
		allFound []foundTarget
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

			found, err := walkScanPath(r, rules)
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
		// Still check global caches even with no local results
		return c.scanGlobalCaches(rules), nil
	}

	paths := make([]string, len(allFound))
	for i, f := range allFound {
		paths[i] = f.path
	}

	sizes, err := devfs.CalcDirSizes(context.Background(), paths)
	if err != nil {
		return nil, err
	}

	targets := make([]domain.Target, 0, len(allFound))
	for _, f := range allFound {
		info, err := os.Stat(f.path)
		if err != nil {
			continue
		}

		hash := sha256.Sum256([]byte(f.path))
		id := fmt.Sprintf("%x", hash[:6])

		var name string
		var risk domain.RiskLevel

		switch f.category {
		case "node":
			parentName := filepath.Base(filepath.Dir(f.path))
			name = parentName + "/node_modules"
			risk = classifyRisk(f.path, rules.MaxAgeDays)
		case "nuxt-cache":
			parentName := filepath.Base(filepath.Dir(f.path))
			name = parentName + "/" + filepath.Base(f.path)
			risk = cleaner.ClassifyLocalArtifactRisk(f.path, f.manifests, rules.MaxAgeDays)
		case "node-dist":
			parentName := filepath.Base(filepath.Dir(f.path))
			name = parentName + "/dist"
			risk = cleaner.ClassifyLocalArtifactRisk(f.path, f.manifests, rules.MaxAgeDays)
		default:
			name = filepath.Base(f.path)
			risk = domain.RiskLow
		}

		cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)
		size := sizes[f.path]
		ttlEligible := false
		eligibleBytes := int64(0)
		if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
			ttlEligible = true
			eligibleBytes = size
		}

		targets = append(targets, domain.Target{
			ID:            id,
			Name:          name,
			Path:          f.path,
			SizeBytes:     size,
			EligibleBytes: eligibleBytes,
			TTLEligible:   ttlEligible,
			Risk:          risk,
			LastAccessed:  info.ModTime(),
			Category:      f.category,
		})
	}

	// Append global caches
	targets = append(targets, c.scanGlobalCaches(rules)...)

	return targets, nil
}

func (c *nodeCleaner) Clean(_ context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if safety.IsCoreBlacklisted(t.Path) {
			continue
		}
		if isLink, err := safety.IsSymlink(t.Path); err == nil && isLink {
			continue // Refuse to traverse symlinks
		}
		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}

type foundTarget struct {
	path      string
	category  string
	manifests []string
}

type globalCacheDef struct {
	category string
	name     string
	risk     domain.RiskLevel
	paths    []string
}

func getGlobalCacheCandidates(home string) []globalCacheDef {
	darwinCaches := filepath.Join(home, "Library", "Caches")
	xdgCaches := filepath.Join(home, ".cache")

	puppeteerPaths := []string{
		filepath.Join(xdgCaches, "puppeteer"),
		filepath.Join(home, ".puppeteer"),
	}
	if env := os.Getenv("PUPPETEER_CACHE_DIR"); env != "" {
		puppeteerPaths = []string{env}
	}

	playwrightPaths := []string{
		filepath.Join(darwinCaches, "ms-playwright"),
		filepath.Join(xdgCaches, "ms-playwright"),
	}
	if env := os.Getenv("PLAYWRIGHT_BROWSERS_PATH"); env != "" {
		playwrightPaths = []string{env}
	}

	cypressPaths := []string{
		filepath.Join(darwinCaches, "Cypress"),
		filepath.Join(xdgCaches, "Cypress"),
	}
	if env := os.Getenv("CYPRESS_CACHE_FOLDER"); env != "" {
		cypressPaths = []string{env}
	}

	return []globalCacheDef{
		{
			category: "npm-cache",
			name:     "npm cache",
			risk:     domain.RiskLow,
			paths:    []string{filepath.Join(home, ".npm", "_cacache")},
		},
		{
			category: "npx-cache",
			name:     "npx cache",
			risk:     domain.RiskLow,
			paths:    []string{filepath.Join(home, ".npm", "_npx")},
		},
		{
			category: "npm-logs",
			name:     "npm logs",
			risk:     domain.RiskLow,
			paths:    []string{filepath.Join(home, ".npm", "_logs")},
		},
		{
			category: "yarn-cache",
			name:     "yarn cache",
			risk:     domain.RiskLow,
			paths:    []string{yarnCachePath(home)},
		},
		{
			category: "corepack-cache",
			name:     "Corepack cache",
			risk:     domain.RiskLow,
			paths: []string{
				filepath.Join(darwinCaches, "node", "corepack"),
				filepath.Join(xdgCaches, "node", "corepack"),
			},
		},
		{
			category: "node-puppeteer",
			name:     "Puppeteer browser cache",
			risk:     domain.RiskMedium,
			paths:    puppeteerPaths,
		},
		{
			category: "node-playwright",
			name:     "Playwright browser cache",
			risk:     domain.RiskMedium,
			paths:    playwrightPaths,
		},
		{
			category: "node-cypress",
			name:     "Cypress binary cache",
			risk:     domain.RiskLow,
			paths:    cypressPaths,
		},
	}
}

func (c *nodeCleaner) scanGlobalCaches(rules domain.EcosystemRules) []domain.Target {
	home, _ := os.UserHomeDir()
	if home == "" {
		return nil
	}

	var targets []domain.Target
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)
	seenPaths := make(map[string]bool)

	for _, def := range getGlobalCacheCandidates(home) {
		for _, path := range def.paths {
			if path == "" || seenPaths[path] {
				continue
			}
			seenPaths[path] = true

			info, err := os.Stat(path)
			if err != nil || !info.IsDir() {
				continue
			}

			size, _ := devfs.CalcDirSize(path)
			if size == 0 {
				continue
			}

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
				Name:          def.name,
				Path:          path,
				SizeBytes:     size,
				EligibleBytes: eligibleBytes,
				TTLEligible:   ttlEligible,
				Risk:          def.risk,
				LastAccessed:  info.ModTime(),
				Category:      def.category,
			})
		}
	}

	return targets
}

func yarnCachePath(home string) string {
	if v := os.Getenv("YARN_CACHE_FOLDER"); v != "" {
		return v
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Caches", "Yarn")
	}
	return filepath.Join(home, ".cache", "yarn")
}

func walkScanPath(root string, rules domain.EcosystemRules) ([]foundTarget, error) {
	var found []foundTarget

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

		switch d.Name() {
		case "node_modules":
			found = append(found, foundTarget{path: path, category: "node"})
			return filepath.SkipDir

		case ".nuxt":
			parentDir := filepath.Dir(path)
			manifests := findNuxtManifests(parentDir)
			found = append(found, foundTarget{path: path, category: "nuxt-cache", manifests: manifests})
			return filepath.SkipDir

		case "dist":
			parentDir := filepath.Dir(path)
			// Exclude non-JS projects
			if isNonJSProject(parentDir) {
				return filepath.SkipDir
			}
			if pkg := findAncestorPackageJSON(path, root, 3); pkg != "" {
				found = append(found, foundTarget{path: path, category: "node-dist", manifests: []string{pkg}})
			}
			return filepath.SkipDir
		}

		return nil
	})

	return found, err
}

func findNuxtManifests(dir string) []string {
	candidates := []string{
		filepath.Join(dir, "nuxt.config.ts"),
		filepath.Join(dir, "nuxt.config.js"),
		filepath.Join(dir, "package.json"),
	}
	var manifests []string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			manifests = append(manifests, c)
		}
	}
	return manifests
}

func isNonJSProject(dir string) bool {
	indicators := []string{"Makefile", "CMakeLists.txt", "setup.py"}
	for _, f := range indicators {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			return true
		}
	}
	return false
}

func findAncestorPackageJSON(distPath, walkRoot string, maxLevels int) string {
	dir := filepath.Dir(distPath)
	for i := 0; i < maxLevels; i++ {
		pkg := filepath.Join(dir, "package.json")
		if _, err := os.Stat(pkg); err == nil {
			return pkg
		}
		parent := filepath.Dir(dir)
		if parent == dir || parent == walkRoot {
			break
		}
		dir = parent
	}
	return ""
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

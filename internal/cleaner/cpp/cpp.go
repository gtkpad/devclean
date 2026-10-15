package cpp

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
	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
)


type cppCleaner struct {
	scanPaths []string
}

// New creates a C/C++ cleaner implementing domain.Cleaner.
func New(scanPaths []string) domain.Cleaner {
	return &cppCleaner{scanPaths: scanPaths}
}

// Name returns the ecosystem identifier for C/C++.
func (c *cppCleaner) Name() string {
	return "cpp"
}

// IsInstalled checks if C/C++ toolchains (cmake, gcc, g++, clang, conan, vcpkg, ninja, make)
// or package manager configuration directories (~/.conan, ~/.conan2) exist.
func (c *cppCleaner) IsInstalled() bool {
	tools := []string{"cmake", "gcc", "g++", "clang", "conan", "vcpkg", "ninja", "make"}
	for _, t := range tools {
		if _, err := exec.LookPath(t); err == nil {
			return true
		}
	}

	if cleaner.HasShallowManifest(c.scanPaths, "CMakeLists.txt", "Makefile", "meson.build", "conanfile.txt", "conanfile.py", "vcpkg.json") {
		return true
	}

	home, _ := os.UserHomeDir()

	candidateDirs := []string{
		filepath.Join(home, ".conan"),
		filepath.Join(home, ".conan2"),
	}
	if v := os.Getenv("CONAN_USER_HOME"); v != "" {
		candidateDirs = append(candidateDirs, filepath.Join(v, ".conan"))
	}
	if v := os.Getenv("CONAN_HOME"); v != "" {
		candidateDirs = append(candidateDirs, v)
	}

	for _, dir := range candidateDirs {
		if _, err := os.Stat(dir); err == nil {
			return true
		}
	}

	return false
}

// Scan discovers local CMake builds, vcpkg_installed directories, and global caches.
func (c *cppCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
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

// Clean removes targets with preventive recursive chmod to handle read-only Conan/vcpkg files.
func (c *cppCleaner) Clean(_ context.Context, targets []domain.Target) error {
	for _, t := range targets {
		_ = devfs.ChmodRecursive(t.Path, 0o755, 0o666)
		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}

func (c *cppCleaner) scanLocalTargets(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
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

			paths, err := walkForCppTargets(ctx, r, rules)
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

	// Deduplicate paths
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

		category := "cmake-build"
		if base == "vcpkg_installed" {
			category = "vcpkg-installed"
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
			Name:          filepath.Base(parentDir) + "/" + base,
			Path:          p,
			SizeBytes:     size,
			EligibleBytes: eligibleBytes,
			TTLEligible:   ttlEligible,
			Risk:          domain.RiskMedium,
			LastAccessed:  info.ModTime(),
			Category:      category,
		})
	}

	return targets, nil
}

func (c *cppCleaner) scanGlobalCaches(rules domain.EcosystemRules) []domain.Target {
	var targets []domain.Target
	seen := make(map[string]bool)
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	home, _ := os.UserHomeDir()

	// 1. Conan 1 cache: $CONAN_USER_HOME/.conan/data or ~/.conan/data
	var conan1Paths []string
	if v := os.Getenv("CONAN_USER_HOME"); v != "" {
		conan1Paths = append(conan1Paths, filepath.Join(v, ".conan", "data"))
	}
	conan1Paths = append(conan1Paths, filepath.Join(home, ".conan", "data"))
	for _, p := range conan1Paths {
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
				Name:          "Conan 1 Cache",
				Path:          p,
				SizeBytes:     size,
				EligibleBytes: eligibleBytes,
				TTLEligible:   ttlEligible,
				Risk:          domain.RiskLow,
				LastAccessed:  info.ModTime(),
				Category:      "conan-cache",
			})
		}
	}

	// 2. Conan 2 cache: $CONAN_HOME/p or ~/.conan2/p
	var conan2Paths []string
	if v := os.Getenv("CONAN_HOME"); v != "" {
		conan2Paths = append(conan2Paths, filepath.Join(v, "p"))
	}
	conan2Paths = append(conan2Paths, filepath.Join(home, ".conan2", "p"))
	for _, p := range conan2Paths {
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
				Name:          "Conan 2 Cache",
				Path:          p,
				SizeBytes:     size,
				EligibleBytes: eligibleBytes,
				TTLEligible:   ttlEligible,
				Risk:          domain.RiskLow,
				LastAccessed:  info.ModTime(),
				Category:      "conan2-cache",
			})
		}
	}

	// 3. vcpkg global: ~/Library/Caches/vcpkg (macOS), $XDG_CACHE_HOME/vcpkg or ~/.cache/vcpkg (Linux)
	var vcpkgPaths []string
	if v := os.Getenv("VCPKG_DEFAULT_BINARY_CACHE"); v != "" {
		vcpkgPaths = append(vcpkgPaths, v)
	}
	if runtime.GOOS == "darwin" {
		vcpkgPaths = append(vcpkgPaths, filepath.Join(home, "Library", "Caches", "vcpkg"))
	}
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		vcpkgPaths = append(vcpkgPaths, filepath.Join(xdg, "vcpkg"))
	}
	vcpkgPaths = append(vcpkgPaths, filepath.Join(home, ".cache", "vcpkg"))

	for _, p := range vcpkgPaths {
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
				Name:          "vcpkg Cache",
				Path:          p,
				SizeBytes:     size,
				EligibleBytes: eligibleBytes,
				TTLEligible:   ttlEligible,
				Risk:          domain.RiskLow,
				LastAccessed:  info.ModTime(),
				Category:      "vcpkg-cache",
			})
		}
	}

	return targets
}

func walkForCppTargets(ctx context.Context, root string, rules domain.EcosystemRules) ([]string, error) {
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

		if isCMakeBuildDir(path) || isVcpkgInstalledDir(path) {
			found = append(found, path)
			return filepath.SkipDir
		}

		return nil
	})

	return found, err
}

func isCMakeBuildDir(path string) bool {
	name := filepath.Base(path)
	if name != "build" && !strings.HasPrefix(name, "cmake-build-") {
		return false
	}
	// 1. Contém CMakeCache.txt interno
	if _, err := os.Stat(filepath.Join(path, "CMakeCache.txt")); err == nil {
		return true
	}
	// 2. Ou possui CMakeLists.txt como irmão no diretório pai
	parent := filepath.Dir(path)
	if _, err := os.Stat(filepath.Join(parent, "CMakeLists.txt")); err == nil {
		return true
	}
	return false
}

func isVcpkgInstalledDir(path string) bool {
	if filepath.Base(path) != "vcpkg_installed" {
		return false
	}
	parent := filepath.Dir(path)
	if _, err := os.Stat(filepath.Join(parent, "vcpkg.json")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(parent, "CMakeLists.txt")); err == nil {
		return true
	}
	return false
}

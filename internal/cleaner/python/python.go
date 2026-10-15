package python

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
	"time"

	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
)

type pythonCleaner struct {
	cachePath string
	scanPaths []string
}

// New creates a Python ecosystem cleaner.
func New(cachePath string, scanPaths []string) domain.Cleaner {
	return &pythonCleaner{cachePath: cachePath, scanPaths: scanPaths}
}

func (c *pythonCleaner) Name() string { return "python" }

func (c *pythonCleaner) IsInstalled() bool {
	if _, err := os.Stat(c.cachePath); err == nil {
		return true
	}
	for _, cmd := range []string{"python3", "python", "uv", "pip"} {
		if _, err := exec.LookPath(cmd); err == nil {
			return true
		}
	}
	return false
}

func (c *pythonCleaner) Scan(_ context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	var targets []domain.Target

	// 1. Existing uv-cache logic
	if uvTarget := c.scanUVCache(rules); uvTarget != nil {
		targets = append(targets, *uvTarget)
	}

	// 2. Pip cache
	if pipTarget := c.scanPipCache(rules); pipTarget != nil {
		targets = append(targets, *pipTarget)
	}

	// 3. Conda packages
	if condaTarget := c.scanCondaPkgs(rules); condaTarget != nil {
		targets = append(targets, *condaTarget)
	}

	// 4. Walk scanPaths for orphaned venvs and __pycache__
	for _, root := range c.scanPaths {
		venvTargets, pycacheSize := c.walkScanPath(root, rules)
		targets = append(targets, venvTargets...)

		// Aggregate __pycache__ per root
		if pycacheSize > 0 {
			hash := sha256.Sum256([]byte("pycache:" + root))
			id := fmt.Sprintf("%x", hash[:6])
			targets = append(targets, domain.Target{
				ID:            id,
				Name:          "__pycache__ (" + filepath.Base(root) + ")",
				Path:          root,
				SizeBytes:     pycacheSize,
				EligibleBytes: pycacheSize,
				TTLEligible:   true,
				Risk:          domain.RiskLow,
				LastAccessed:  time.Now(),
				Category:      "python-pycache",
			})
		}
	}

	return targets, nil
}

func (c *pythonCleaner) Clean(_ context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if t.Category == "python-pycache" {
			// For pycache, the path is the root — we need to walk and remove all __pycache__ dirs
			_ = filepath.WalkDir(t.Path, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() && d.Name() == "__pycache__" {
					os.RemoveAll(path)
					return filepath.SkipDir
				}
				return nil
			})
			continue
		}
		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}

func (c *pythonCleaner) scanUVCache(rules domain.EcosystemRules) *domain.Target {
	if c.cachePath == "" {
		return nil
	}

	for _, exc := range rules.ExcludePaths {
		if strings.HasPrefix(c.cachePath, exc) {
			return nil
		}
	}

	info, err := os.Stat(c.cachePath)
	if err != nil {
		return nil
	}

	lastAccessed := info.ModTime()
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	size, err := devfs.CalcDirSize(c.cachePath)
	if err != nil || size == 0 {
		return nil
	}

	eligibleBytes := int64(0)
	ttlEligible := false
	if rules.MaxAgeDays == 0 || lastAccessed.Before(cutoff) {
		eligibleBytes = size
		ttlEligible = true
	}

	return &domain.Target{
		ID:            "uv-cache:" + c.cachePath,
		Name:          "uv-cache",
		Path:          c.cachePath,
		SizeBytes:     size,
		EligibleBytes: eligibleBytes,
		TTLEligible:   ttlEligible,
		Risk:          domain.RiskLow,
		LastAccessed:  lastAccessed,
		Category:      "uv-cache",
	}
}

func (c *pythonCleaner) scanPipCache(rules domain.EcosystemRules) *domain.Target {
	path := pipCachePath()
	if path == "" {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	size, _ := devfs.CalcDirSize(path)
	if size == 0 {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)
	lastAccessed := info.ModTime()
	eligibleBytes := int64(0)
	ttlEligible := false
	if rules.MaxAgeDays == 0 || lastAccessed.Before(cutoff) {
		eligibleBytes = size
		ttlEligible = true
	}

	hash := sha256.Sum256([]byte(path))
	id := fmt.Sprintf("%x", hash[:6])

	return &domain.Target{
		ID:            id,
		Name:          "pip cache",
		Path:          path,
		SizeBytes:     size,
		EligibleBytes: eligibleBytes,
		TTLEligible:   ttlEligible,
		Risk:          domain.RiskLow,
		LastAccessed:  lastAccessed,
		Category:      "pip-cache",
	}
}

func (c *pythonCleaner) scanCondaPkgs(rules domain.EcosystemRules) *domain.Target {
	path := condaPkgsPath()
	if path == "" {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	size, _ := devfs.CalcDirSize(path)
	if size == 0 {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)
	lastAccessed := info.ModTime()
	eligibleBytes := int64(0)
	ttlEligible := false
	if rules.MaxAgeDays == 0 || lastAccessed.Before(cutoff) {
		eligibleBytes = size
		ttlEligible = true
	}

	hash := sha256.Sum256([]byte(path))
	id := fmt.Sprintf("%x", hash[:6])

	return &domain.Target{
		ID:            id,
		Name:          "conda packages",
		Path:          path,
		SizeBytes:     size,
		EligibleBytes: eligibleBytes,
		TTLEligible:   ttlEligible,
		Risk:          domain.RiskLow,
		LastAccessed:  lastAccessed,
		Category:      "conda-pkgs",
	}
}

func (c *pythonCleaner) walkScanPath(root string, rules domain.EcosystemRules) ([]domain.Target, int64) {
	var venvTargets []domain.Target
	var pycacheSize int64

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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

		switch d.Name() {
		case ".venv", "venv":
			orphaned, risk := isOrphanedVenv(path, rules.MaxAgeDays)
			if orphaned {
				info, _ := os.Stat(path)
				size, _ := devfs.CalcDirSize(path)
				hash := sha256.Sum256([]byte(path))
				id := fmt.Sprintf("%x", hash[:6])

				lastAccessed := time.Time{}
				if info != nil {
					lastAccessed = info.ModTime()
				}

				venvTargets = append(venvTargets, domain.Target{
					ID:            id,
					Name:          filepath.Base(filepath.Dir(path)) + "/" + d.Name(),
					Path:          path,
					SizeBytes:     size,
					EligibleBytes: size,
					TTLEligible:   true,
					Risk:          risk,
					LastAccessed:  lastAccessed,
					Category:      "python-venv-orphan",
				})
			}
			return filepath.SkipDir

		case "__pycache__":
			size, _ := devfs.CalcDirSize(path)
			pycacheSize += size
			return filepath.SkipDir
		}

		return nil
	})

	return venvTargets, pycacheSize
}

var pythonManifests = []string{"pyproject.toml", "setup.py", "setup.cfg", "requirements.txt", "Pipfile"}

func isOrphanedVenv(venvPath string, maxAgeDays int) (bool, domain.RiskLevel) {
	parentDir := filepath.Dir(venvPath)
	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)

	for _, manifest := range pythonManifests {
		info, err := os.Stat(filepath.Join(parentDir, manifest))
		if err == nil {
			if info.ModTime().Before(cutoff) {
				return true, domain.RiskMedium
			}
			return false, ""
		}
	}
	return true, domain.RiskLow
}

func pipCachePath() string {
	if v := os.Getenv("PIP_CACHE_DIR"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		return ""
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Caches", "pip")
	}
	cacheDir := os.Getenv("XDG_CACHE_HOME")
	if cacheDir == "" {
		cacheDir = filepath.Join(home, ".cache")
	}
	return filepath.Join(cacheDir, "pip")
}

func condaPkgsPath() string {
	if v := os.Getenv("CONDA_PREFIX"); v != "" {
		return filepath.Join(v, "pkgs")
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		return ""
	}
	for _, name := range []string{"miniconda3", "anaconda3", "miniforge3"} {
		p := filepath.Join(home, name, "pkgs")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

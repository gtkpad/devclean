package golang

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/fs"
)


type goCleaner struct {
	modCache   string
	buildCache string
}

// New creates a Go ecosystem cleaner.
func New() domain.Cleaner {
	return &goCleaner{}
}

func (c *goCleaner) Name() string { return "go" }

func (c *goCleaner) IsInstalled() bool {
	if _, err := exec.LookPath("go"); err == nil {
		return true
	}
	c.resolvePaths()
	return fs.DirExists(c.modCache) || fs.DirExists(c.buildCache)
}


func (c *goCleaner) Scan(_ context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	c.resolvePaths()

	var targets []domain.Target
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	if c.modCache != "" {
		if t := c.scanCache(c.modCache, "go-mod-cache", rules, cutoff); t != nil {
			targets = append(targets, *t)
		}
	}
	if c.buildCache != "" {
		if t := c.scanCache(c.buildCache, "go-build-cache", rules, cutoff); t != nil {
			targets = append(targets, *t)
		}
	}

	return targets, nil
}

func (c *goCleaner) Clean(_ context.Context, targets []domain.Target) error {
	for _, t := range targets {
		// Go module cache files are read-only; chmod before removing
		_ = filepath.WalkDir(t.Path, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() {
				_ = os.Chmod(path, 0o666)
			}
			return nil
		})
		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}

func (c *goCleaner) resolvePaths() {
	// Check env vars first to avoid exec and respect test isolation
	if v, ok := os.LookupEnv("GOMODCACHE"); ok {
		c.modCache = v
	} else {
		c.modCache = goEnv("GOMODCACHE")
		if c.modCache == "" {
			if gopath := os.Getenv("GOPATH"); gopath != "" {
				c.modCache = filepath.Join(gopath, "pkg", "mod")
			} else if home, err := os.UserHomeDir(); err == nil && home != "" {
				c.modCache = filepath.Join(home, "go", "pkg", "mod")
			}
		}
	}

	if v, ok := os.LookupEnv("GOCACHE"); ok {
		c.buildCache = v
	} else {
		c.buildCache = goEnv("GOCACHE")
		if c.buildCache == "" {
			if home, err := os.UserHomeDir(); err == nil && home != "" {
				if runtime.GOOS == "darwin" {
					c.buildCache = filepath.Join(home, "Library", "Caches", "go-build")
				} else {
					c.buildCache = filepath.Join(home, ".cache", "go-build")
				}
			}
		}
	}
}


func (c *goCleaner) scanCache(path, category string, rules domain.EcosystemRules, cutoff time.Time) *domain.Target {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	for _, exc := range rules.ExcludePaths {
		if strings.HasPrefix(path, exc) {
			return nil
		}
	}

	size, err := fs.CalcDirSize(path)
	if err != nil || size == 0 {
		return nil
	}

	lastAccessed := info.ModTime()
	eligibleBytes := int64(0)
	ttlEligible := false
	if rules.MaxAgeDays == 0 || lastAccessed.Before(cutoff) {
		eligibleBytes = size
		ttlEligible = true
	}

	risk := domain.RiskLow // caches are always re-downloadable
	if category == "go-build-cache" && lastAccessed.After(cutoff) {
		risk = domain.RiskMedium
	}

	return &domain.Target{
		ID:            category + ":" + path,
		Name:          category,
		Path:          path,
		SizeBytes:     size,
		EligibleBytes: eligibleBytes,
		TTLEligible:   ttlEligible,
		Risk:          risk,
		LastAccessed:  lastAccessed,
		Category:      category,
	}
}

func goEnv(key string) string {
	out, err := exec.Command("go", "env", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

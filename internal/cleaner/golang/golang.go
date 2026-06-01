package golang

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gabrielvp/devclean/internal/domain"
	"github.com/gabrielvp/devclean/internal/fs"
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
	_, err := exec.LookPath("go")
	return err == nil
}

func (c *goCleaner) Scan(rules domain.EcosystemRules) ([]domain.Target, error) {
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

func (c *goCleaner) Clean(targets []domain.Target) error {
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
	// Check env vars first to avoid exec
	if v := os.Getenv("GOMODCACHE"); v != "" {
		c.modCache = v
	} else {
		c.modCache = goEnv("GOMODCACHE")
	}

	if v := os.Getenv("GOCACHE"); v != "" {
		c.buildCache = v
	} else {
		c.buildCache = goEnv("GOCACHE")
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
	if rules.MaxAgeDays > 0 && lastAccessed.After(cutoff) {
		return nil
	}

	risk := domain.RiskLow // caches are always re-downloadable
	if category == "go-build-cache" && lastAccessed.After(cutoff) {
		risk = domain.RiskMedium
	}

	return &domain.Target{
		ID:           category + ":" + path,
		Name:         category,
		Path:         path,
		SizeBytes:    size,
		Risk:         risk,
		LastAccessed: lastAccessed,
		Category:     category,
	}
}

func goEnv(key string) string {
	out, err := exec.Command("go", "env", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

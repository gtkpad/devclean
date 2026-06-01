package python

import (
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gabrielvp/devclean/internal/domain"
	"github.com/gabrielvp/devclean/internal/fs"
)

type pythonCleaner struct {
	cachePath string
}

// New creates a Python/uv ecosystem cleaner.
func New(cachePath string) domain.Cleaner {
	return &pythonCleaner{cachePath: cachePath}
}

func (c *pythonCleaner) Name() string { return "python" }

func (c *pythonCleaner) IsInstalled() bool {
	if _, err := os.Stat(c.cachePath); err == nil {
		return true
	}
	_, err := exec.LookPath("uv")
	return err == nil
}

func (c *pythonCleaner) Scan(rules domain.EcosystemRules) ([]domain.Target, error) {
	if c.cachePath == "" {
		return nil, nil
	}

	for _, exc := range rules.ExcludePaths {
		if strings.HasPrefix(c.cachePath, exc) {
			return nil, nil
		}
	}

	info, err := os.Stat(c.cachePath)
	if err != nil {
		return nil, nil
	}

	lastAccessed := info.ModTime()
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)
	if rules.MaxAgeDays > 0 && lastAccessed.After(cutoff) {
		return nil, nil
	}

	size, err := fs.CalcDirSize(c.cachePath)
	if err != nil || size == 0 {
		return nil, nil
	}

	return []domain.Target{
		{
			ID:           "uv-cache:" + c.cachePath,
			Name:         "uv-cache",
			Path:         c.cachePath,
			SizeBytes:    size,
			Risk:         domain.RiskLow,
			LastAccessed: lastAccessed,
			Category:     "uv-cache",
		},
	}, nil
}

func (c *pythonCleaner) Clean(_ []domain.Target) error {
	return nil // Deletion is Phase 4
}

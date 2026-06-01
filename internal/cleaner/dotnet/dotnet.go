package dotnet

import (
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gabrielvp/devclean/internal/domain"
	"github.com/gabrielvp/devclean/internal/fs"
	"github.com/gabrielvp/devclean/internal/platform"
)

type dotnetCleaner struct {
	paths platform.Paths
}

// New creates a .NET ecosystem cleaner.
func New(paths platform.Paths) domain.Cleaner {
	return &dotnetCleaner{paths: paths}
}

func (c *dotnetCleaner) Name() string { return "dotnet" }

func (c *dotnetCleaner) IsInstalled() bool {
	_, err := exec.LookPath("dotnet")
	return err == nil
}

func (c *dotnetCleaner) Scan(rules domain.EcosystemRules) ([]domain.Target, error) {
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

		size, err := fs.CalcDirSize(d.path)
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

func (c *dotnetCleaner) Clean(_ []domain.Target) error {
	return nil // Deletion is Phase 4
}

func isExcluded(path string, excludes []string) bool {
	for _, exc := range excludes {
		if strings.HasPrefix(path, exc) {
			return true
		}
	}
	return false
}

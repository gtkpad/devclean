package bun

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
)

type bunCleaner struct {
	cachePath string
}

// New creates a new Bun cleaner with the default cache path ~/.bun/install/cache.
func New() domain.Cleaner {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return &bunCleaner{cachePath: ""}
	}
	return &bunCleaner{
		cachePath: filepath.Join(home, ".bun", "install", "cache"),
	}
}

// NewWithPath creates a new Bun cleaner with a custom cache path.
func NewWithPath(cachePath string) domain.Cleaner {
	return &bunCleaner{cachePath: cachePath}
}

func (c *bunCleaner) Name() string {
	return "bun"
}

func (c *bunCleaner) IsInstalled() bool {
	if c.cachePath == "" {
		return false
	}
	if _, err := os.Stat(c.cachePath); err == nil {
		return true
	}
	return false
}

func (c *bunCleaner) Scan(_ context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	info, err := os.Stat(c.cachePath)
	if err != nil || !info.IsDir() {
		return nil, nil
	}

	size, _ := devfs.CalcDirSize(c.cachePath)
	if size == 0 {
		return nil, nil
	}

	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)
	lastAccessed := info.ModTime()

	eligibleBytes := int64(0)
	ttlEligible := false
	if rules.MaxAgeDays == 0 || lastAccessed.Before(cutoff) {
		eligibleBytes = size
		ttlEligible = true
	}

	sum := sha256.Sum256([]byte(c.cachePath))
	id := fmt.Sprintf("%x", sum[:6])

	return []domain.Target{
		{
			ID:            id,
			Name:          "Bun package cache",
			Path:          c.cachePath,
			SizeBytes:     size,
			EligibleBytes: eligibleBytes,
			TTLEligible:   ttlEligible,
			Risk:          domain.RiskLow,
			LastAccessed:  lastAccessed,
			Category:      "bun-cache",
		},
	}, nil
}

func (c *bunCleaner) Clean(_ context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}

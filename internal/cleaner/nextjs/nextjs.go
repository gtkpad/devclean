package nextjs

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
)

type nextjsCleaner struct {
	scanPaths []string
}

// New creates a new Next.js cleaner scanning the provided roots.
func New(scanPaths []string) domain.Cleaner {
	return &nextjsCleaner{scanPaths: scanPaths}
}

func (c *nextjsCleaner) Name() string {
	return "nextjs"
}

func (c *nextjsCleaner) IsInstalled() bool {
	return true
}

func (c *nextjsCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	var targets []domain.Target
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	for _, root := range c.scanPaths {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
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
			if isLink, linkErr := safety.IsSymlink(path); linkErr == nil && isLink {
				return filepath.SkipDir
			}

			rel, relErr := filepath.Rel(root, path)
			if relErr == nil {
				for _, pattern := range rules.ExcludePaths {
					if matched, _ := doublestar.Match(pattern, rel); matched {
						return filepath.SkipDir
					}
				}
			}

			if d.Name() == ".next" {
				cacheDir := filepath.Join(path, "cache")
				if info, err := os.Stat(cacheDir); err == nil && info.IsDir() {
					if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
						size, _ := devfs.CalcDirSize(cacheDir)
						if size > 0 {
							sum := sha256.Sum256([]byte(cacheDir))
							targets = append(targets, domain.Target{
								ID:           fmt.Sprintf("%x", sum[:6]),
								Name:         filepath.Base(filepath.Dir(path)) + "/.next/cache",
								Path:         cacheDir,
								SizeBytes:    size,
								Risk:         domain.RiskLow,
								LastAccessed: info.ModTime(),
								Category:     "nextjs-cache",
							})
						}
					}
				}
				return filepath.SkipDir
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return targets, nil
}

func (c *nextjsCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}

package installer

import (
	"context"
	"os"

	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
)

// Cleaner implements domain.Cleaner for orphan installer packages.
type Cleaner struct {
	opts Options
}

// NewCleaner creates a new domain.Cleaner for installers.
func NewCleaner(opts Options) domain.Cleaner {
	return &Cleaner{opts: opts}
}

// Name returns the identifier for this cleaner ("installer").
func (c *Cleaner) Name() string {
	return "installer"
}

// IsInstalled returns true since installer cleanup relies solely on standard filesystem operations.
func (c *Cleaner) IsInstalled() bool {
	return true
}

// Scan performs installer candidate discovery and maps them to domain.Target entries.
func (c *Cleaner) Scan(ctx context.Context, _ domain.EcosystemRules) ([]domain.Target, error) {
	scanner := NewScanner(c.opts, nil)
	candidates, err := scanner.Scan(ctx)
	if err != nil {
		return nil, err
	}

	targets := make([]domain.Target, len(candidates))
	for i, cand := range candidates {
		targets[i] = domain.Target{
			ID:           "installer:" + cand.Path,
			Name:         cand.Detail,
			Path:         cand.Path,
			SizeBytes:    cand.SizeBytes,
			Risk:         domain.RiskLow,
			LastAccessed: cand.ModTime,
			Category:     "installer",
		}
	}
	return targets, nil
}

// Clean physically removes targets, applying ChmodRecursive beforehand to unlock read-only files.
func (c *Cleaner) Clean(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_ = devfs.ChmodRecursive(t.Path, 0o755, 0o666)
		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}

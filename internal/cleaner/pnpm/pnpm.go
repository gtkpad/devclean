package pnpm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
)

type pnpmCleaner struct {
	exec cleaner.CommandExecutor
}

// New returns a new pnpm cleaner using the default command executor.
func New() domain.Cleaner {
	return NewWithExecutor(&cleaner.DefaultExecutor{})
}

// NewWithExecutor returns a new pnpm cleaner using the specified command executor.
func NewWithExecutor(exec cleaner.CommandExecutor) domain.Cleaner {
	return &pnpmCleaner{exec: exec}
}

func (c *pnpmCleaner) Name() string {
	return "pnpm"
}

func (c *pnpmCleaner) IsInstalled() bool {
	if _, err := c.exec.LookPath("pnpm"); err == nil {
		return true
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	candidates := []string{
		filepath.Join(home, "Library", "pnpm", "store"),
		filepath.Join(home, ".local", "share", "pnpm", "store"),
		filepath.Join(home, ".pnpm-store"),
	}
	for _, p := range candidates {
		if devfs.DirExists(p) {
			return true
		}
	}
	return false
}


func (c *pnpmCleaner) Scan(ctx context.Context, _ domain.EcosystemRules) ([]domain.Target, error) {
	out, err := c.exec.Output(ctx, "pnpm", "store", "path")
	if err != nil {
		return nil, nil
	}

	storePath := strings.TrimSpace(string(out))
	if storePath == "" {
		return nil, nil
	}

	size, _ := devfs.CalcDirSize(storePath)
	if size == 0 {
		return nil, nil
	}

	return []domain.Target{
		{
			ID:            "pnpm-store-prune",
			Name:          "pnpm store (unreferenced packages)",
			Path:          "pnpm://store/prune",
			SizeBytes:     size,
			EligibleBytes: size,
			TTLEligible:   true,
			Risk:          domain.RiskLow,
			Category:      "virtual-pnpm-store",
		},
	}, nil
}

func (c *pnpmCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if t.Path == "pnpm://store/prune" {
			if _, err := c.exec.Output(ctx, "pnpm", "store", "prune"); err != nil {
				return fmt.Errorf("pnpm store prune failed: %w", err)
			}
		}
	}
	return nil
}

package shell

import (
	"context"
	"os"
	"path/filepath"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
)

type shellCleaner struct {
	exec cleaner.CommandExecutor
}

// New creates a new shellCleaner with the default command executor.
func New() domain.Cleaner {
	return NewWithExecutor(&cleaner.DefaultExecutor{})
}

// NewWithExecutor creates a new shellCleaner with a custom command executor.
func NewWithExecutor(exec cleaner.CommandExecutor) domain.Cleaner {
	return &shellCleaner{exec: exec}
}

// Name returns the ecosystem identifier for the cleaner.
func (c *shellCleaner) Name() string {
	return "shell"
}

// IsInstalled checks if any supported CLI executable (mise, gh, pre-commit, zsh) is available
// in PATH or if any target cache directory or zcompdump file exists on disk (D-10, SCAN-02).
func (c *shellCleaner) IsInstalled() bool {
	// 1. Check binaries in PATH
	for _, bin := range []string{"mise", "gh", "pre-commit", "zsh"} {
		if _, err := c.exec.LookPath(bin); err == nil {
			return true
		}
	}

	// 2. Check disk presence
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}

	candidates := []string{
		filepath.Join(home, "Library", "Caches", "mise"),
		filepath.Join(home, ".cache", "mise"),
		filepath.Join(home, ".local", "share", "mise", "downloads"),
		filepath.Join(home, "Library", "Caches", "gh"),
		filepath.Join(home, ".cache", "gh"),
		filepath.Join(home, ".cache", "pre-commit"),
		filepath.Join(home, ".oh-my-zsh", "cache"),
	}

	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		candidates = append(candidates, filepath.Join(xdgData, "mise", "downloads"))
	}
	if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
		candidates = append(candidates,
			filepath.Join(xdgCache, "mise"),
			filepath.Join(xdgCache, "gh"),
			filepath.Join(xdgCache, "pre-commit"),
		)
	}
	if preCommitHome := os.Getenv("PRE_COMMIT_HOME"); preCommitHome != "" {
		candidates = append(candidates, preCommitHome)
	}
	if zshCache := os.Getenv("ZSH_CACHE_DIR"); zshCache != "" {
		candidates = append(candidates, zshCache)
	}

	for _, p := range candidates {
		if devfs.DirExists(p) {
			return true
		}
	}

	// Check if any .zcompdump* exists in HOME
	if matches, _ := filepath.Glob(filepath.Join(home, ".zcompdump*")); len(matches) > 0 {
		return true
	}

	// Check if any .zcompdump* exists in ZDOTDIR
	if zdotdir := os.Getenv("ZDOTDIR"); zdotdir != "" && zdotdir != home {
		if matches, _ := filepath.Glob(filepath.Join(zdotdir, ".zcompdump*")); len(matches) > 0 {
			return true
		}
	}

	return false
}

// Scan inventories shell and CLI tool caches. Implemented in Task 08-02-02 & Task 08-02-03.
func (c *shellCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	return nil, nil
}

// Clean deletes shell and CLI tool caches. Implemented in Task 08-02-03.
func (c *shellCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	return nil
}

package ai

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)

type aiCleaner struct {
	exec      cleaner.CommandExecutor
	scanPaths []string
	homeDir   string
}

// New creates a new AI cleaner for artificial intelligence tools and editors.
func New(scanPaths []string) domain.Cleaner {
	return NewWithExecutor(&cleaner.DefaultExecutor{}, scanPaths)
}

// NewWithExecutor creates a new AI cleaner with custom command executor.
func NewWithExecutor(exec cleaner.CommandExecutor, scanPaths []string) domain.Cleaner {
	home, _ := os.UserHomeDir()
	return &aiCleaner{
		exec:      exec,
		scanPaths: scanPaths,
		homeDir:   home,
	}
}

// NewWithHome instancia o cleaner com home directory configurável para testes herméticos.
func NewWithHome(scanPaths []string, homeDir string) domain.Cleaner {
	return &aiCleaner{
		exec:      &cleaner.DefaultExecutor{},
		scanPaths: scanPaths,
		homeDir:   homeDir,
	}
}

// NewWithExecutorAndHome creates an AI cleaner with custom executor and custom home.
func NewWithExecutorAndHome(exec cleaner.CommandExecutor, scanPaths []string, homeDir string) domain.Cleaner {
	return &aiCleaner{
		exec:      exec,
		scanPaths: scanPaths,
		homeDir:   homeDir,
	}
}

func (c *aiCleaner) Name() string { return "ai" }

func (c *aiCleaner) IsInstalled() bool { return true }

func (c *aiCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var targets []domain.Target
	if c.homeDir == "" {
		return targets, nil
	}

	// 1. Claude Code (~/.claude)
	claudeTargets, err := scanClaude(filepath.Join(c.homeDir, ".claude"), rules)
	if err == nil {
		targets = append(targets, claudeTargets...)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// 2. VS Code & Cursor (CachedData, logs, .obsolete)
	editorTargets, err := scanEditors(ctx, c.exec, c.homeDir, rules)
	if err == nil {
		targets = append(targets, editorTargets...)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// 3. Ollama (~/.ollama/models)
	ollamaTargets, err := scanOllama(filepath.Join(c.homeDir, ".ollama", "models"), rules)
	if err == nil {
		targets = append(targets, ollamaTargets...)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// 4. HuggingFace (~/.cache/huggingface)
	hfTargets, err := scanHuggingFace(filepath.Join(c.homeDir, ".cache", "huggingface"), rules)
	if err == nil {
		targets = append(targets, hfTargets...)
	}

	return targets, nil
}

func (c *aiCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}
		if t.IsVirtual() {
			continue
		}
		if err := os.RemoveAll(t.Path); err != nil {
			return fmt.Errorf("ai cleaner failed to remove %s: %w", t.Path, err)
		}
	}
	return nil
}

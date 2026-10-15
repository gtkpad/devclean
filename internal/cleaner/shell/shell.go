package shell

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
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

type directoryCacheDef struct {
	category string
	name     string
	risk     domain.RiskLevel
	paths    []string
}

func getDirectoryCacheCandidates(home string) []directoryCacheDef {
	darwinCaches := filepath.Join(home, "Library", "Caches")
	xdgCaches := filepath.Join(home, ".cache")

	miseCachePaths := []string{
		filepath.Join(darwinCaches, "mise"),
		filepath.Join(xdgCaches, "mise"),
	}
	if env := os.Getenv("XDG_CACHE_HOME"); env != "" {
		miseCachePaths = append(miseCachePaths, filepath.Join(env, "mise"))
	}

	miseDownloadPaths := []string{
		filepath.Join(home, ".local", "share", "mise", "downloads"),
	}
	if env := os.Getenv("XDG_DATA_HOME"); env != "" {
		miseDownloadPaths = append(miseDownloadPaths, filepath.Join(env, "mise", "downloads"))
	}

	ghCachePaths := []string{
		filepath.Join(darwinCaches, "gh"),
		filepath.Join(xdgCaches, "gh"),
	}
	if env := os.Getenv("XDG_CACHE_HOME"); env != "" {
		ghCachePaths = append(ghCachePaths, filepath.Join(env, "gh"))
	}

	preCommitPaths := []string{}
	if env := os.Getenv("PRE_COMMIT_HOME"); env != "" {
		preCommitPaths = append(preCommitPaths, env)
	}
	preCommitPaths = append(preCommitPaths, filepath.Join(xdgCaches, "pre-commit"))
	if env := os.Getenv("XDG_CACHE_HOME"); env != "" {
		preCommitPaths = append(preCommitPaths, filepath.Join(env, "pre-commit"))
	}

	omzPaths := []string{}
	if env := os.Getenv("ZSH_CACHE_DIR"); env != "" {
		omzPaths = append(omzPaths, env)
	}
	omzPaths = append(omzPaths, filepath.Join(home, ".oh-my-zsh", "cache"))

	return []directoryCacheDef{
		{
			category: "shell-mise-cache",
			name:     "mise cache",
			risk:     domain.RiskLow,
			paths:    miseCachePaths,
		},
		{
			category: "shell-mise-downloads",
			name:     "mise downloads",
			risk:     domain.RiskLow,
			paths:    miseDownloadPaths,
		},
		{
			category: "shell-gh-cache",
			name:     "GitHub CLI cache",
			risk:     domain.RiskLow,
			paths:    ghCachePaths,
		},
		{
			category: "shell-pre-commit-cache",
			name:     "pre-commit cache",
			risk:     domain.RiskMedium,
			paths:    preCommitPaths,
		},
		{
			category: "shell-omz-cache",
			name:     "Oh My Zsh cache",
			risk:     domain.RiskLow,
			paths:    omzPaths,
		},
	}
}

func scanDirectoryCaches(home string, rules domain.EcosystemRules) []domain.Target {
	var targets []domain.Target
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)
	seenPaths := make(map[string]bool)

	for _, def := range getDirectoryCacheCandidates(home) {
		for _, path := range def.paths {
			if path == "" || seenPaths[path] {
				continue
			}
			seenPaths[path] = true

			if isExcluded(path, rules.ExcludePaths) {
				continue
			}

			info, err := os.Stat(path)
			if err != nil || !info.IsDir() {
				continue
			}

			size, _ := devfs.CalcDirSize(path)
			if size == 0 {
				continue
			}

			hash := sha256.Sum256([]byte(path))
			id := fmt.Sprintf("%x", hash[:6])

			ttlEligible := false
			eligibleBytes := int64(0)
			if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
				ttlEligible = true
				eligibleBytes = size
			}

			targets = append(targets, domain.Target{
				ID:            id,
				Name:          def.name,
				Path:          path,
				SizeBytes:     size,
				EligibleBytes: eligibleBytes,
				TTLEligible:   ttlEligible,
				Risk:          def.risk,
				LastAccessed:  info.ModTime(),
				Category:      def.category,
			})
		}
	}

	return targets
}

func isExcluded(path string, excludes []string) bool {
	base := filepath.Base(path)
	for _, exc := range excludes {
		if strings.HasPrefix(path, exc) {
			return true
		}
		if matched, _ := doublestar.Match(exc, path); matched {
			return true
		}
		if matched, _ := doublestar.Match(exc, base); matched {
			return true
		}
	}
	return false
}

func scanZshCompdumps(home string, rules domain.EcosystemRules) *domain.Target {
	if isExcluded("shell://zsh/compdump", rules.ExcludePaths) {
		return nil
	}

	var files []string

	// Check $HOME
	if matches, err := filepath.Glob(filepath.Join(home, ".zcompdump*")); err == nil {
		files = append(files, matches...)
	}

	// Check $ZDOTDIR if set and distinct from $HOME
	if zdotdir := os.Getenv("ZDOTDIR"); zdotdir != "" && zdotdir != home {
		if matches, err := filepath.Glob(filepath.Join(zdotdir, ".zcompdump*")); err == nil {
			files = append(files, matches...)
		}
	}

	if len(files) == 0 {
		return nil
	}

	var totalSize int64
	var mostRecent time.Time
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)
	seen := make(map[string]bool)

	for _, f := range files {
		if seen[f] {
			continue
		}
		seen[f] = true

		if isExcluded(f, rules.ExcludePaths) {
			continue
		}

		info, err := os.Stat(f)
		if err != nil || info.IsDir() {
			continue
		}

		totalSize += info.Size()
		if info.ModTime().After(mostRecent) {
			mostRecent = info.ModTime()
		}
	}

	if totalSize == 0 {
		return nil
	}

	ttlEligible := false
	eligibleBytes := int64(0)
	if rules.MaxAgeDays == 0 || mostRecent.Before(cutoff) {
		ttlEligible = true
		eligibleBytes = totalSize
	}

	return &domain.Target{
		ID:            "shell-zsh-compdump",
		Name:          "Zsh completion cache",
		Path:          "shell://zsh/compdump",
		SizeBytes:     totalSize,
		EligibleBytes: eligibleBytes,
		TTLEligible:   ttlEligible,
		Risk:          domain.RiskLow,
		LastAccessed:  mostRecent,
		Category:      "shell-zsh-compdump",
	}
}

// Scan inventories shell and CLI tool caches.
func (c *shellCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil, nil
	}

	targets := scanDirectoryCaches(home, rules)
	if zshTarget := scanZshCompdumps(home, rules); zshTarget != nil {
		targets = append(targets, *zshTarget)
	}

	return targets, nil
}

// Clean deletes shell and CLI tool caches.
func (c *shellCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	home, _ := os.UserHomeDir()

	for _, t := range targets {
		if t.Path == "shell://zsh/compdump" {
			var patterns []string
			if home != "" {
				patterns = append(patterns, filepath.Join(home, ".zcompdump*"))
			}
			if zdotdir := os.Getenv("ZDOTDIR"); zdotdir != "" && zdotdir != home {
				patterns = append(patterns, filepath.Join(zdotdir, ".zcompdump*"))
			}

			for _, pat := range patterns {
				matches, err := filepath.Glob(pat)
				if err != nil {
					continue
				}
				for _, f := range matches {
					if safety.IsCoreBlacklisted(f) {
						continue
					}
					info, err := os.Lstat(f)
					if err != nil || info.IsDir() {
						continue
					}
					if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
						return err
					}
				}
			}
			continue
		}

		if safety.IsCoreBlacklisted(t.Path) {
			continue
		}
		if isLink, err := safety.IsSymlink(t.Path); err == nil && isLink {
			continue
		}
		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}


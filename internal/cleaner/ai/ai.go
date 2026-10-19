package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/safety"
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

	// 1. Claude Code (~/.claude, ~/.local/share/claude/versions)
	claudeTargets, err := scanClaude(ctx, c.exec, c.homeDir, rules)
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

// extractEditorIDFromURI parses an editor URI (e.g. editor://code/workspace-backups)
// into editorID ("code") and category ("workspace-backups").
func extractEditorIDFromURI(uri string) (editorID string, category string) {
	trimmed := strings.TrimPrefix(uri, "editor://")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return "", ""
}

// findEditorDef looks up the editorDef matching id.
func findEditorDef(id string) (editorDef, bool) {
	for _, ed := range getEditorDefinitions() {
		if ed.id == id {
			return ed, true
		}
	}
	return editorDef{}, false
}

// cleanEditorVirtualTarget surgically unlinks authorized leaves for an editor virtual target.
func cleanEditorVirtualTarget(homeDir string, t domain.Target) error {
	editorID, category := extractEditorIDFromURI(t.Path)
	ed, ok := findEditorDef(editorID)
	if !ok {
		return nil
	}

	switch category {
	case "workspace-backups":
		if ed.hasWorkspaces && ed.appSupportDir != nil {
			appSupport := ed.appSupportDir(homeDir)
			if appSupport != "" {
				wsRoot := filepath.Join(appSupport, "User", "workspaceStorage")
				entries, err := os.ReadDir(wsRoot)
				if err == nil {
					for _, e := range entries {
						if !e.IsDir() {
							continue
						}
						backupPath := filepath.Join(wsRoot, e.Name(), "state.vscdb.backup")
						info, err := os.Stat(backupPath)
						if err != nil || info.IsDir() {
							continue
						}
						// NEVER remove state.vscdb or workspace.json!
						_ = os.Remove(backupPath)
					}
				}
			}
		}

	case "extensions":
		// 1. CachedExtensionVSIXs
		if ed.appSupportDir != nil {
			appSupport := ed.appSupportDir(homeDir)
			if appSupport != "" {
				vsixDir := filepath.Join(appSupport, "CachedExtensionVSIXs")
				if !safety.IsCoreBlacklisted(vsixDir) {
					if isLink, err := safety.IsSymlink(vsixDir); err == nil && !isLink {
						_ = os.RemoveAll(vsixDir)
					}
				}
			}
		}
		// 2. Obsolete extensions from extRoots
		if ed.extRoots != nil {
			for _, root := range ed.extRoots(homeDir) {
				obsoleteFile := filepath.Join(root, ".obsolete")
				data, err := os.ReadFile(obsoleteFile)
				if err != nil {
					continue
				}
				var obsoleteMap map[string]bool
				if err := json.Unmarshal(data, &obsoleteMap); err != nil {
					continue
				}
				for extFolder, isObsolete := range obsoleteMap {
					if !isObsolete {
						continue // Positive match only! Never touch active extensions
					}
					cleanFolder := filepath.Clean(extFolder)
					if strings.Contains(cleanFolder, "..") || filepath.IsAbs(cleanFolder) {
						continue // Path traversal defense!
					}
					fullDir := filepath.Join(root, cleanFolder)
					rel, err := filepath.Rel(root, fullDir)
					if err != nil || strings.HasPrefix(rel, "..") || rel == "." {
						continue // Escape containment check!
					}
					if isLink, err := safety.IsSymlink(fullDir); err == nil && isLink {
						continue // Symlink defense!
					}
					if safety.IsCoreBlacklisted(fullDir) {
						continue // Core blacklist defense!
					}
					_ = os.RemoveAll(fullDir)
				}
				// Do NOT remove .obsolete file itself!
			}
		}

	case "chromium-cache":
		var candidateDirs []string
		if ed.appSupportDir != nil {
			appSupport := ed.appSupportDir(homeDir)
			if appSupport != "" {
				subLeaves := []string{
					"Cache",
					"Code Cache",
					"GPUCache",
					"DawnCache",
					"DawnWebGPUCache",
					"DawnGraphiteCache",
					filepath.Join("Service Worker", "CacheStorage"),
					filepath.Join("Service Worker", "ScriptCache"),
					"CachedData",
					"webviewCache",
				}
				for _, sub := range subLeaves {
					candidateDirs = append(candidateDirs, filepath.Join(appSupport, sub))
				}
			}
		}
		if ed.cacheDirs != nil {
			candidateDirs = append(candidateDirs, ed.cacheDirs(homeDir)...)
		}
		for _, p := range candidateDirs {
			cleanP := filepath.Clean(p)
			if safety.IsCoreBlacklisted(cleanP) {
				continue
			}
			if isLink, err := safety.IsSymlink(cleanP); err == nil && isLink {
				continue
			}
			_ = os.RemoveAll(cleanP)
		}

	case "logs":
		var candidateDirs []string
		if ed.appSupportDir != nil {
			appSupport := ed.appSupportDir(homeDir)
			if appSupport != "" {
				candidateDirs = append(candidateDirs,
					filepath.Join(appSupport, "logs"),
					filepath.Join(appSupport, "Sentry"),
				)
				crashpadCompleted := filepath.Join(appSupport, "Crashpad", "completed")
				crashpadRoot := filepath.Join(appSupport, "Crashpad")
				if info, err := os.Stat(crashpadCompleted); err == nil && info.IsDir() {
					candidateDirs = append(candidateDirs, crashpadCompleted)
				} else if info, err := os.Stat(crashpadRoot); err == nil && info.IsDir() {
					candidateDirs = append(candidateDirs, crashpadRoot)
				}
			}
		}
		if ed.logDirs != nil {
			candidateDirs = append(candidateDirs, ed.logDirs(homeDir)...)
		}
		for _, p := range candidateDirs {
			cleanP := filepath.Clean(p)
			if safety.IsCoreBlacklisted(cleanP) {
				continue
			}
			if isLink, err := safety.IsSymlink(cleanP); err == nil && isLink {
				continue
			}
			_ = os.RemoveAll(cleanP)
		}
	}

	return nil
}

func (c *aiCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Virtual editor targets are cleaned surgically with pre-deletion process revalidation
		if strings.HasPrefix(t.Path, "editor://") {
			editorID, _ := extractEditorIDFromURI(t.Path)
			if edDef, ok := findEditorDef(editorID); ok {
				running, uncertain := checkProcessStatus(ctx, c.exec, edDef.processNames...)
				if running || uncertain {
					continue // Abort deleting targets of active editor to prevent data corruption (D-07)
				}
			}
			if err := cleanEditorVirtualTarget(c.homeDir, t); err != nil {
				return fmt.Errorf("failed to clean editor target %s: %w", t.Path, err)
			}
			continue
		}

		// Revalidate process for Claude targets before deletion
		if strings.HasPrefix(t.Category, "ai-claude") {
			running, uncertain := checkProcessStatus(ctx, c.exec, "claude")
			if running || uncertain {
				continue // Abort deleting Claude targets if CLI started after scan (D-07)
			}
		}

		if t.IsVirtual() {
			continue
		}

		if safety.IsCoreBlacklisted(t.Path) {
			continue
		}
		if isLink, err := safety.IsSymlink(t.Path); err == nil && isLink {
			continue
		}

		if err := os.RemoveAll(t.Path); err != nil {
			return fmt.Errorf("ai cleaner failed to remove %s: %w", t.Path, err)
		}
	}
	return nil
}

package ai

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
)

type editorDef struct {
	id            string
	name          string
	processNames  []string
	hasExtensions bool
	hasWorkspaces bool
	appSupportDir func(home string) string
	cacheDirs     func(home string) []string
	logDirs       func(home string) []string
	extRoots      func(home string) []string
}

func getEditorDefinitions() []editorDef {
	return []editorDef{
		{
			id:            "code",
			name:          "VS Code",
			processNames:  []string{"Code", "code"},
			hasExtensions: true,
			hasWorkspaces: true,
			appSupportDir: func(home string) string {
				darwinPath := filepath.Join(home, "Library", "Application Support", "Code")
				linuxPath := filepath.Join(home, ".config", "Code")
				if runtime.GOOS == "darwin" {
					if _, err := os.Stat(darwinPath); err == nil {
						return darwinPath
					}
					if _, err := os.Stat(linuxPath); err == nil {
						return linuxPath
					}
					return darwinPath
				}
				if _, err := os.Stat(linuxPath); err == nil {
					return linuxPath
				}
				if _, err := os.Stat(darwinPath); err == nil {
					return darwinPath
				}
				return linuxPath
			},
			cacheDirs: func(home string) []string {
				return []string{
					filepath.Join(home, "Library", "Caches", "com.microsoft.VSCode"),
					filepath.Join(home, "Library", "Caches", "com.microsoft.VSCode.ShipIt"),
					filepath.Join(home, ".cache", "Code"),
				}
			},
			logDirs: nil,
			extRoots: func(home string) []string {
				return []string{filepath.Join(home, ".vscode", "extensions")}
			},
		},
		{
			id:            "code-insiders",
			name:          "VS Code Insiders",
			processNames:  []string{"Code - Insiders", "code-insiders"},
			hasExtensions: true,
			hasWorkspaces: true,
			appSupportDir: func(home string) string {
				darwinPath := filepath.Join(home, "Library", "Application Support", "Code - Insiders")
				linuxPath := filepath.Join(home, ".config", "Code - Insiders")
				if runtime.GOOS == "darwin" {
					if _, err := os.Stat(darwinPath); err == nil {
						return darwinPath
					}
					if _, err := os.Stat(linuxPath); err == nil {
						return linuxPath
					}
					return darwinPath
				}
				if _, err := os.Stat(linuxPath); err == nil {
					return linuxPath
				}
				if _, err := os.Stat(darwinPath); err == nil {
					return darwinPath
				}
				return linuxPath
			},
			cacheDirs: func(home string) []string {
				return []string{
					filepath.Join(home, "Library", "Caches", "com.microsoft.VSCodeInsiders"),
					filepath.Join(home, "Library", "Caches", "com.microsoft.VSCodeInsiders.ShipIt"),
					filepath.Join(home, ".cache", "Code - Insiders"),
				}
			},
			logDirs: nil,
			extRoots: func(home string) []string {
				return []string{filepath.Join(home, ".vscode-insiders", "extensions")}
			},
		},
		{
			id:            "cursor",
			name:          "Cursor",
			processNames:  []string{"Cursor", "cursor"},
			hasExtensions: true,
			hasWorkspaces: true,
			appSupportDir: func(home string) string {
				darwinPath := filepath.Join(home, "Library", "Application Support", "Cursor")
				linuxPath := filepath.Join(home, ".config", "Cursor")
				if runtime.GOOS == "darwin" {
					if _, err := os.Stat(darwinPath); err == nil {
						return darwinPath
					}
					if _, err := os.Stat(linuxPath); err == nil {
						return linuxPath
					}
					return darwinPath
				}
				if _, err := os.Stat(linuxPath); err == nil {
					return linuxPath
				}
				if _, err := os.Stat(darwinPath); err == nil {
					return darwinPath
				}
				return linuxPath
			},
			cacheDirs: func(home string) []string {
				return []string{
					filepath.Join(home, "Library", "Caches", "com.todesktop.230313mzl4w4u92"),
					filepath.Join(home, "Library", "Caches", "com.todesktop.230313mzl4w4u92.ShipIt"),
					filepath.Join(home, "Library", "Caches", "Cursor"),
					filepath.Join(home, ".cache", "Cursor"),
				}
			},
			logDirs: nil,
			extRoots: func(home string) []string {
				return []string{filepath.Join(home, ".cursor", "extensions")}
			},
		},
		{
			id:            "antigravity",
			name:          "Antigravity",
			processNames:  []string{"Antigravity", "antigravity"},
			hasExtensions: true,
			hasWorkspaces: true,
			appSupportDir: func(home string) string {
				darwinPath := filepath.Join(home, "Library", "Application Support", "Antigravity")
				linuxPath := filepath.Join(home, ".config", "Antigravity")
				if runtime.GOOS == "darwin" {
					if _, err := os.Stat(darwinPath); err == nil {
						return darwinPath
					}
					if _, err := os.Stat(linuxPath); err == nil {
						return linuxPath
					}
					return darwinPath
				}
				if _, err := os.Stat(linuxPath); err == nil {
					return linuxPath
				}
				if _, err := os.Stat(darwinPath); err == nil {
					return darwinPath
				}
				return linuxPath
			},
			cacheDirs: func(home string) []string {
				return []string{
					filepath.Join(home, "Library", "Caches", "Antigravity"),
					filepath.Join(home, "Library", "Caches", "com.google.antigravity"),
					filepath.Join(home, "Library", "Caches", "com.google.antigravity.ShipIt"),
					filepath.Join(home, ".cache", "Antigravity"),
				}
			},
			logDirs: nil,
			extRoots: func(home string) []string {
				return []string{
					filepath.Join(home, ".antigravity", "extensions"),
					filepath.Join(home, ".antigravity-ide", "extensions"),
				}
			},
		},
		{
			id:            "windsurf",
			name:          "Windsurf",
			processNames:  []string{"Windsurf", "windsurf"},
			hasExtensions: true,
			hasWorkspaces: true,
			appSupportDir: func(home string) string {
				darwinPath := filepath.Join(home, "Library", "Application Support", "Windsurf")
				linuxPath := filepath.Join(home, ".config", "Windsurf")
				if runtime.GOOS == "darwin" {
					if _, err := os.Stat(darwinPath); err == nil {
						return darwinPath
					}
					if _, err := os.Stat(linuxPath); err == nil {
						return linuxPath
					}
					return darwinPath
				}
				if _, err := os.Stat(linuxPath); err == nil {
					return linuxPath
				}
				if _, err := os.Stat(darwinPath); err == nil {
					return darwinPath
				}
				return linuxPath
			},
			cacheDirs: func(home string) []string {
				return []string{
					filepath.Join(home, "Library", "Caches", "com.exafunction.windsurf"),
					filepath.Join(home, "Library", "Caches", "com.exafunction.windsurf.ShipIt"),
					filepath.Join(home, "Library", "Caches", "Windsurf"),
					filepath.Join(home, ".cache", "Windsurf"),
				}
			},
			logDirs: nil,
			extRoots: func(home string) []string {
				return []string{filepath.Join(home, ".windsurf", "extensions")}
			},
		},
		{
			id:            "claude-desktop",
			name:          "Claude Desktop",
			processNames:  []string{"Claude", "claude-desktop"},
			hasExtensions: false,
			hasWorkspaces: false,
			appSupportDir: func(home string) string {
				darwinPath := filepath.Join(home, "Library", "Application Support", "Claude")
				linuxPath := filepath.Join(home, ".config", "Claude")
				if runtime.GOOS == "darwin" {
					if _, err := os.Stat(darwinPath); err == nil {
						return darwinPath
					}
					if _, err := os.Stat(linuxPath); err == nil {
						return linuxPath
					}
					return darwinPath
				}
				if _, err := os.Stat(linuxPath); err == nil {
					return linuxPath
				}
				if _, err := os.Stat(darwinPath); err == nil {
					return darwinPath
				}
				return linuxPath
			},
			cacheDirs: func(home string) []string {
				return []string{
					filepath.Join(home, "Library", "Caches", "com.anthropic.claudefordesktop"),
					filepath.Join(home, "Library", "Caches", "com.anthropic.claudefordesktop.ShipIt"),
					filepath.Join(home, ".cache", "Claude"),
				}
			},
			logDirs:  nil,
			extRoots: nil,
		},
		{
			id:            "opencode",
			name:          "OpenCode",
			processNames:  []string{"opencode", "OpenCode"},
			hasExtensions: false,
			hasWorkspaces: false,
			appSupportDir: func(home string) string {
				darwinPath := filepath.Join(home, "Library", "Application Support", "opencode")
				linuxPath := filepath.Join(home, ".config", "opencode")
				if runtime.GOOS == "darwin" {
					if _, err := os.Stat(darwinPath); err == nil {
						return darwinPath
					}
					if _, err := os.Stat(linuxPath); err == nil {
						return linuxPath
					}
					return darwinPath
				}
				if _, err := os.Stat(linuxPath); err == nil {
					return linuxPath
				}
				if _, err := os.Stat(darwinPath); err == nil {
					return darwinPath
				}
				return linuxPath
			},
			cacheDirs: func(home string) []string {
				return []string{
					filepath.Join(home, "Library", "Caches", "opencode"),
					filepath.Join(home, ".cache", "opencode"),
				}
			},
			logDirs:  nil,
			extRoots: nil,
		},
		{
			id:            "zed",
			name:          "Zed",
			processNames:  []string{"zed", "Zed"},
			hasExtensions: false,
			hasWorkspaces: false,
			appSupportDir: func(home string) string {
				return ""
			},
			cacheDirs: func(home string) []string {
				return []string{
					filepath.Join(home, "Library", "Caches", "dev.zed.Zed"),
					filepath.Join(home, ".cache", "zed"),
				}
			},
			logDirs: func(home string) []string {
				return []string{
					filepath.Join(home, "Library", "Logs", "Zed"),
					filepath.Join(home, ".local", "state", "zed", "logs"),
				}
			},
			extRoots: nil,
		},
	}
}

func scanEditorChromiumCaches(ed editorDef, home string, cutoff time.Time, maxAgeDays int) (int64, int64, bool, time.Time, []string) {
	var totalSize int64
	var eligibleBytes int64
	var hasEligible bool
	var latestModTime time.Time
	var foundPaths []string

	var candidateDirs []string
	if ed.appSupportDir != nil {
		appSupport := ed.appSupportDir(home)
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
		candidateDirs = append(candidateDirs, ed.cacheDirs(home)...)
	}

	seen := make(map[string]bool)
	for _, p := range candidateDirs {
		cleanP := filepath.Clean(p)
		if seen[cleanP] {
			continue
		}
		seen[cleanP] = true

		info, err := os.Stat(cleanP)
		if err != nil {
			continue
		}

		sz, _ := devfs.CalcDirSize(cleanP)
		if !info.IsDir() {
			sz = info.Size()
		}
		if sz == 0 {
			continue
		}

		totalSize += sz
		foundPaths = append(foundPaths, cleanP)
		if info.ModTime().After(latestModTime) {
			latestModTime = info.ModTime()
		}

		if maxAgeDays == 0 || info.ModTime().Before(cutoff) {
			eligibleBytes += sz
			hasEligible = true
		}
	}

	return totalSize, eligibleBytes, hasEligible, latestModTime, foundPaths
}

func scanEditorLogs(ed editorDef, home string, cutoff time.Time, maxAgeDays int) (int64, int64, bool, time.Time, []string) {
	var totalSize int64
	var eligibleBytes int64
	var hasEligible bool
	var latestModTime time.Time
	var foundPaths []string

	var candidateDirs []string
	if ed.appSupportDir != nil {
		appSupport := ed.appSupportDir(home)
		if appSupport != "" {
			candidateDirs = append(candidateDirs, filepath.Join(appSupport, "logs"))
			candidateDirs = append(candidateDirs, filepath.Join(appSupport, "Sentry"))

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
		candidateDirs = append(candidateDirs, ed.logDirs(home)...)
	}

	seen := make(map[string]bool)
	for _, p := range candidateDirs {
		cleanP := filepath.Clean(p)
		if seen[cleanP] {
			continue
		}
		seen[cleanP] = true

		info, err := os.Stat(cleanP)
		if err != nil {
			continue
		}

		sz, _ := devfs.CalcDirSize(cleanP)
		if !info.IsDir() {
			sz = info.Size()
		}
		if sz == 0 {
			continue
		}

		totalSize += sz
		foundPaths = append(foundPaths, cleanP)
		if info.ModTime().After(latestModTime) {
			latestModTime = info.ModTime()
		}

		if maxAgeDays == 0 || info.ModTime().Before(cutoff) {
			eligibleBytes += sz
			hasEligible = true
		}
	}

	return totalSize, eligibleBytes, hasEligible, latestModTime, foundPaths
}

func scanEditors(ctx context.Context, exec cleaner.CommandExecutor, home string, rules domain.EcosystemRules) ([]domain.Target, error) {
	_ = exec
	if home == "" {
		return nil, nil
	}

	var targets []domain.Target
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	for _, ed := range getEditorDefinitions() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// 1. Chromium & GPU Caches
		totalCacheSize, eligibleCacheBytes, hasEligibleCache, cacheTime, _ := scanEditorChromiumCaches(ed, home, cutoff, rules.MaxAgeDays)
		if totalCacheSize > 0 {
			if cacheTime.IsZero() {
				cacheTime = time.Now()
			}
			vPath := "editor://" + ed.id + "/chromium-cache"
			targets = append(targets, domain.Target{
				ID:            hashPath(vPath),
				Name:          ed.name + ": Chromium & GPU Caches",
				Path:          vPath,
				SizeBytes:     totalCacheSize,
				EligibleBytes: eligibleCacheBytes,
				TTLEligible:   hasEligibleCache,
				Risk:          domain.RiskLow,
				LastAccessed:  cacheTime,
				Category:      "editor-chromium-cache",
			})
		}

		// 2. Logs & Crash Reports
		totalLogSize, eligibleLogBytes, hasEligibleLog, logTime, _ := scanEditorLogs(ed, home, cutoff, rules.MaxAgeDays)
		if totalLogSize > 0 {
			if logTime.IsZero() {
				logTime = time.Now()
			}
			vPath := "editor://" + ed.id + "/logs"
			targets = append(targets, domain.Target{
				ID:            hashPath(vPath),
				Name:          ed.name + ": Logs & Crash Reports",
				Path:          vPath,
				SizeBytes:     totalLogSize,
				EligibleBytes: eligibleLogBytes,
				TTLEligible:   hasEligibleLog,
				Risk:          domain.RiskLow,
				LastAccessed:  logTime,
				Category:      "editor-logs",
			})
		}
	}

	return targets, nil
}

package macos

import (
	"context"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)

// Exclusion table for directories in ~/Library/Caches handled by dedicated cleaners (D-05, SCAN-04).
var dedicatedEcosystemCaches = map[string]string{
	"pnpm":                  "pnpm",
	"uv":                    "python",
	"pip":                   "python",
	"Homebrew":              "homebrew",
	"CocoaPods":             "cocoapods",
	"vcpkg":                 "cpp",
	"mise":                  "shell",
	"gh":                    "shell",
	"ms-playwright":         "node",
	"Cypress":               "node",
	"node":                  "node",
	"Yarn":                  "node",
	"dev.orbstack.OrbStack": "docker",
}

// Prefix exclusions for AI/Electron editors handled by internal/cleaner/ai (D-05).
var dedicatedAIEditorPrefixes = []string{
	"com.microsoft.VSCode",
	"Cursor",
	"Antigravity",
	"com.google.antigravity",
	"Windsurf",
	"com.exafunction.windsurf",
	"com.anthropic.claudefordesktop",
	"opencode",
	"dev.zed.Zed",
}

const largeBundleThresholdBytes = 50 * 1024 * 1024 // 50MB threshold (D-06)

// isExcludedByDedicatedCleaner checks whether a cache folder is handled elsewhere (D-05).
func isExcludedByDedicatedCleaner(dirName string) bool {
	return false
}

// scanUserCaches walks ~/Library/Caches consolidating bundles <= 50MB and breaking out larger ones (MAC-01, D-05, D-06).
func scanUserCaches(ctx context.Context, exec cleaner.CommandExecutor, homeDir string) ([]domain.Target, error) {
	return nil, nil
}

// scanUserLogs walks ~/Library/Logs consolidating log entries into consolidated targets (MAC-01, D-06).
func scanUserLogs(ctx context.Context, homeDir string) ([]domain.Target, error) {
	return nil, nil
}

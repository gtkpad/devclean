package macos

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/fs"
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
	"Google":                 "browsers",
	"BraveSoftware":          "browsers",
	"company.thebrowser.Arc": "browsers",
	"Microsoft Edge":         "browsers",
	"Firefox":                "browsers",
	"com.apple.Safari":       "browsers",
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
	if _, ok := dedicatedEcosystemCaches[dirName]; ok {
		return true
	}
	for _, prefix := range dedicatedAIEditorPrefixes {
		if strings.HasPrefix(dirName, prefix) {
			return true
		}
	}
	return false
}

// scanUserCaches walks ~/Library/Caches consolidating bundles <= 50MB and breaking out larger ones (MAC-01, D-05, D-06).
func scanUserCaches(ctx context.Context, exec cleaner.CommandExecutor, homeDir string) ([]domain.Target, error) {
	cachesRoot := filepath.Join(homeDir, "Library", "Caches")
	entries, err := os.ReadDir(cachesRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var targets []domain.Target
	var consolidatedSize, consolidatedEligible int64
	var consolidatedCount int

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		name := entry.Name()
		fullPath := filepath.Join(cachesRoot, name)

		// 1. Skip items owned by other dedicated cleaners (D-05)
		if isExcludedByDedicatedCleaner(name) {
			continue
		}

		// 2. Skip items matching immutable whitelist (D-15)
		if IsWhitelisted(fullPath) || IsWhitelisted(name) {
			continue
		}

		// 3. Symlink guard: reject symlinks escaping cache root (D-16)
		if entry.Type()&os.ModeSymlink != 0 {
			if !isSafeSymlink(cachesRoot, fullPath) {
				targets = append(targets, domain.Target{
					ID:           "macos-cache-symlink-" + name,
					Name:         fmt.Sprintf("macOS Cache: %s (ambiguous symlink)", name),
					Path:         fullPath,
					ReviewOnly:   true,
					ReviewReason: domain.ReviewReasonAmbiguous,
					Risk:         domain.RiskMedium,
					Category:     "macos-app-caches",
				})
				continue
			}
		}

		usage, _ := fs.CalcUsage(fullPath)
		size := usage.AllocatedBytes
		if size == 0 {
			size = usage.ApparentBytes
		}

		// 4. Process guard check (D-14)
		running, uncertain := IsBundleProcessRunning(ctx, exec, name)
		isActive := running || uncertain

		if size > largeBundleThresholdBytes || isActive {
			// Break out into individual target (D-06)
			reviewOnly := isActive
			reason := ""
			var eligible int64
			if isActive {
				reason = domain.ReviewReasonActiveProcess
				eligible = 0
			} else {
				eligible = size
			}

			targets = append(targets, domain.Target{
				ID:            "macos-cache-" + name,
				Name:          fmt.Sprintf("macOS App Cache: %s", name),
				Path:          fullPath,
				SizeBytes:     size,
				EligibleBytes: eligible,
				TTLEligible:   !reviewOnly,
				ReviewOnly:    reviewOnly,
				ReviewReason:  reason,
				Risk:          domain.RiskLow,
				Category:      "macos-app-caches",
			})
		} else {
			// Consolidate into category bucket (D-06)
			consolidatedSize += size
			consolidatedEligible += size
			consolidatedCount++
		}
	}

	if consolidatedCount > 0 || consolidatedSize > 0 {
		targets = append(targets, domain.Target{
			ID:            "macos-user-app-caches-consolidated",
			Name:          "macOS User App Caches (consolidated)",
			Path:          cachesRoot,
			SizeBytes:     consolidatedSize,
			EligibleBytes: consolidatedEligible,
			TTLEligible:   true,
			ReviewOnly:    false,
			Risk:          domain.RiskLow,
			Category:      "macos-app-caches",
		})
	}

	return targets, nil
}

// scanUserLogs walks ~/Library/Logs consolidating log entries into consolidated targets (MAC-01, D-06).
func scanUserLogs(ctx context.Context, homeDir string) ([]domain.Target, error) {
	logsRoot := filepath.Join(homeDir, "Library", "Logs")
	entries, err := os.ReadDir(logsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var targets []domain.Target
	var consolidatedSize, consolidatedEligible int64
	var consolidatedCount int

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		name := entry.Name()
		fullPath := filepath.Join(logsRoot, name)

		// Exclude DiagnosticReports (handled separately in system.go per D-12)
		if name == "DiagnosticReports" {
			continue
		}

		// Skip items matching immutable whitelist (D-15)
		if IsWhitelisted(fullPath) || IsWhitelisted(name) {
			continue
		}

		// Symlink guard: reject symlinks escaping logs root (D-16)
		if entry.Type()&os.ModeSymlink != 0 {
			if !isSafeSymlink(logsRoot, fullPath) {
				targets = append(targets, domain.Target{
					ID:           "macos-log-symlink-" + name,
					Name:         fmt.Sprintf("macOS Log: %s (ambiguous symlink)", name),
					Path:         fullPath,
					ReviewOnly:   true,
					ReviewReason: domain.ReviewReasonAmbiguous,
					Risk:         domain.RiskMedium,
					Category:     "macos-app-logs",
				})
				continue
			}
		}

		usage, _ := fs.CalcUsage(fullPath)
		size := usage.AllocatedBytes
		if size == 0 {
			size = usage.ApparentBytes
		}

		if size > largeBundleThresholdBytes {
			// Break out large log entries > 50MB into individual targets (D-06)
			targets = append(targets, domain.Target{
				ID:            "macos-log-" + name,
				Name:          fmt.Sprintf("macOS App Log: %s", name),
				Path:          fullPath,
				SizeBytes:     size,
				EligibleBytes: size,
				TTLEligible:   true,
				ReviewOnly:    false,
				Risk:          domain.RiskLow,
				Category:      "macos-app-logs",
			})
		} else {
			// Consolidate into category bucket (D-06)
			consolidatedSize += size
			consolidatedEligible += size
			consolidatedCount++
		}
	}

	if consolidatedCount > 0 || consolidatedSize > 0 {
		targets = append(targets, domain.Target{
			ID:            "macos-user-app-logs-consolidated",
			Name:          "macOS User App Logs (consolidated)",
			Path:          logsRoot,
			SizeBytes:     consolidatedSize,
			EligibleBytes: consolidatedEligible,
			TTLEligible:   true,
			ReviewOnly:    false,
			Risk:          domain.RiskLow,
			Category:      "macos-app-logs",
		})
	}

	return targets, nil
}

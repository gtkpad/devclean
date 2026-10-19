package macos

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/fs"
)

// scanApplicationSupport scans strictly regenerable leaf directories in ~/Library/Application Support
// and isolates the dedicated Spotify PersistentCache target with active process protection (MAC-03, D-07).
func scanApplicationSupport(ctx context.Context, exec cleaner.CommandExecutor, homeDir string) ([]domain.Target, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	appSupportRoot := filepath.Join(homeDir, "Library", "Application Support")
	if _, err := os.Stat(appSupportRoot); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var targets []domain.Target

	// 1. Dedicated Spotify Target (MAC-03, D-07)
	spotifyCache := filepath.Join(appSupportRoot, "Spotify", "PersistentCache")
	if info, err := os.Stat(spotifyCache); err == nil && info.IsDir() {
		// Symlink guard
		linfo, err := os.Lstat(spotifyCache)
		isSafe := true
		if err == nil && linfo.Mode()&os.ModeSymlink != 0 {
			if !isSafeSymlink(appSupportRoot, spotifyCache) {
				isSafe = false
			}
		}

		if isSafe {
			usage, _ := fs.CalcUsage(spotifyCache)
			size := usage.AllocatedBytes
			if size == 0 {
				size = usage.ApparentBytes
				if size == 0 {
					size = info.Size()
				}
			}

			running, uncertain := CheckProcessStatus(ctx, exec, "Spotify")
			isActive := running || uncertain

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
				ID:            "macos-spotify-persistent-cache",
				Name:          "Spotify: PersistentCache",
				Path:          spotifyCache,
				SizeBytes:     size,
				EligibleBytes: eligible,
				TTLEligible:   !reviewOnly,
				ReviewOnly:    reviewOnly,
				ReviewReason:  reason,
				Risk:          domain.RiskLow,
				Category:      "macos-spotify-cache",
			})
		}
	}

	// 2. Discover safe leaves across other applications
	entries, err := os.ReadDir(appSupportRoot)
	if err != nil {
		return targets, err
	}

	var consolidatedSize, consolidatedEligible int64
	var consolidatedCount int
	var consolidatedSubPaths []string

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		appName := entry.Name()
		if appName == "Spotify" {
			continue // Already handled as dedicated target
		}

		appDir := filepath.Join(appSupportRoot, appName)

		isDir := entry.IsDir()
		if !isDir && entry.Type()&os.ModeSymlink != 0 {
			if fi, err := os.Stat(appDir); err == nil && fi.IsDir() {
				isDir = true
			}
		}
		if !isDir {
			continue
		}

		// Whitelist check
		if IsWhitelisted(appDir) || IsWhitelisted(appName) {
			continue
		}

		// Symlink check on appDir
		if entry.Type()&os.ModeSymlink != 0 {
			if !isSafeSymlink(appSupportRoot, appDir) {
				continue
			}
		}

		subEntries, err := os.ReadDir(appDir)
		if err != nil {
			continue
		}

		for _, sub := range subEntries {
			subName := sub.Name()
			lowerSub := strings.ToLower(subName)
			subPath := filepath.Join(appDir, subName)

			if lowerSub == "cache" || lowerSub == "caches" || lowerSub == "logs" {
				info, err := os.Stat(subPath)
				if err != nil || !info.IsDir() {
					continue
				}

				linfo, err := os.Lstat(subPath)
				if err == nil && linfo.Mode()&os.ModeSymlink != 0 {
					if !isSafeSymlink(appDir, subPath) {
						continue
					}
				}

				usage, _ := fs.CalcUsage(subPath)
				size := usage.AllocatedBytes
				if size == 0 {
					size = usage.ApparentBytes
					if size == 0 {
						size = info.Size()
					}
				}

				consolidatedSize += size
				consolidatedEligible += size
				consolidatedCount++
				consolidatedSubPaths = append(consolidatedSubPaths, subPath)
			} else if lowerSub == "crashpad" {
				completedDir := filepath.Join(subPath, "completed")
				info, err := os.Stat(completedDir)
				if err == nil && info.IsDir() {
					linfo, err := os.Lstat(completedDir)
					if err == nil && linfo.Mode()&os.ModeSymlink != 0 {
						if !isSafeSymlink(subPath, completedDir) {
							continue
						}
					}

					usage, _ := fs.CalcUsage(completedDir)
					size := usage.AllocatedBytes
					if size == 0 {
						size = usage.ApparentBytes
						if size == 0 {
							size = info.Size()
						}
					}

					consolidatedSize += size
					consolidatedEligible += size
					consolidatedCount++
					consolidatedSubPaths = append(consolidatedSubPaths, completedDir)
				}
			}
		}
	}

	if consolidatedCount > 0 || consolidatedSize > 0 {
		targets = append(targets, domain.Target{
			ID:             "macos-appsupport-leaves-consolidated",
			Name:           "macOS Application Support Caches & Logs",
			Path:           appSupportRoot,
			SizeBytes:      consolidatedSize,
			EligibleBytes:  consolidatedEligible,
			TTLEligible:    true,
			ReviewOnly:     false,
			Risk:           domain.RiskLow,
			Category:       "macos-appsupport-caches",
			IsConsolidated: true,
			SubPaths:       consolidatedSubPaths,
		})
	}

	return targets, nil
}

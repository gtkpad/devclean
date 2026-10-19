package macos

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/fs"
)

// Allowed subfolder names for group containers (D-08).
var allowedGroupContainerSubdirs = []string{
	"Library/Caches",
	"Library/Logs",
	"Caches",
	"Logs",
}

// sanitizeID replaces characters that may be problematic in target IDs.
func sanitizeID(s string) string {
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	return s
}

// scanGroupContainers scans allowlisted cache and log subfolders in ~/Library/Group Containers (D-08).
// Strictly excludes *.dev.orbstack to protect Docker/OrbStack virtual machine disks (D-08).
// Never emits or targets root group container folders directly.
func scanGroupContainers(ctx context.Context, homeDir string) ([]domain.Target, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	groupsRoot := filepath.Join(homeDir, "Library", "Group Containers")
	entries, err := os.ReadDir(groupsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var targets []domain.Target
	var consolidatedSize, consolidatedEligible int64
	var consolidatedCount int
	var consolidatedSubPaths []string

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		name := entry.Name()
		containerPath := filepath.Join(groupsRoot, name)

		isDir := entry.IsDir()
		if !isDir && entry.Type()&os.ModeSymlink != 0 {
			if fi, err := os.Stat(containerPath); err == nil && fi.IsDir() {
				isDir = true
			}
		}
		if !isDir {
			continue
		}

		// 1. Strictly ignore OrbStack containers (D-08)
		if strings.Contains(name, ".dev.orbstack") || strings.Contains(name, "dev.orbstack") {
			continue
		}

		// 2. Skip items matching immutable whitelist (D-15)
		if IsWhitelisted(containerPath) || IsWhitelisted(name) {
			continue
		}

		// 3. Symlink guard: if the container folder itself is a symlink, verify safety
		if entry.Type()&os.ModeSymlink != 0 {
			if !isSafeSymlink(groupsRoot, containerPath) {
				continue
			}
		}

		// 4. Inspect strictly the allowlisted subdirectories (D-08)
		for _, sub := range allowedGroupContainerSubdirs {
			subPath := filepath.Join(containerPath, filepath.FromSlash(sub))
			info, err := os.Stat(subPath)
			if err != nil || !info.IsDir() {
				continue
			}

			// Symlink guard on subfolder
			linfo, err := os.Lstat(subPath)
			if err == nil && linfo.Mode()&os.ModeSymlink != 0 {
				if !isSafeSymlink(containerPath, subPath) {
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

			if size > largeBundleThresholdBytes {
				// Break out large group container subfolder into dedicated target (D-06)
				targets = append(targets, domain.Target{
					ID:            fmt.Sprintf("macos-group-%s-%s", sanitizeID(name), sanitizeID(sub)),
					Name:          fmt.Sprintf("macOS Group Container: %s (%s)", name, sub),
					Path:          subPath,
					SizeBytes:     size,
					EligibleBytes: size,
					TTLEligible:   true,
					ReviewOnly:    false,
					Risk:          domain.RiskLow,
					Category:      "macos-group-caches",
				})
			} else {
				consolidatedSize += size
				consolidatedEligible += size
				consolidatedCount++
				consolidatedSubPaths = append(consolidatedSubPaths, subPath)
			}
		}
	}

	if consolidatedCount > 0 || consolidatedSize > 0 {
		targets = append(targets, domain.Target{
			ID:             "macos-group-containers-consolidated",
			Name:           "macOS Group Container Caches & Logs",
			Path:           groupsRoot,
			SizeBytes:      consolidatedSize,
			EligibleBytes:  consolidatedEligible,
			TTLEligible:    true,
			ReviewOnly:     false,
			Risk:           domain.RiskLow,
			Category:       "macos-group-caches",
			IsConsolidated: true,
			SubPaths:       consolidatedSubPaths,
		})
	}

	return targets, nil
}

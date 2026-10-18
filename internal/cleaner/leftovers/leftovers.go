package leftovers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
)

// AppLeftoverGroup aggregates residual directories belonging to the same uninstalled application.
type AppLeftoverGroup struct {
	AppName string
	Paths   []string
}

// normalizeAppGroupName derives a canonical, lowercase application group identifier from a directory name.
// If the directory name contains dot notation (e.g. reverse domain 'com.example.oldapp'),
// it extracts the last non-empty segment ('oldapp'). Otherwise it returns the lowercased name.
func normalizeAppGroupName(dirName string) string {
	clean := strings.TrimSpace(dirName)
	if strings.Contains(clean, ".") {
		parts := strings.Split(clean, ".")
		for i := len(parts) - 1; i >= 0; i-- {
			seg := strings.ToLower(strings.TrimSpace(parts[i]))
			if seg != "" {
				return seg
			}
		}
	}
	return strings.ToLower(clean)
}

// isSafeSymlink verifies that linkPath resolves to a target residing strictly inside parentDir
// and does not point to or traverse a blacklisted system root (D-16).
func isSafeSymlink(parentDir, linkPath string) bool {
	resolvedLink, err := filepath.EvalSymlinks(linkPath)
	if err != nil {
		return false
	}
	resolvedParent, err := filepath.EvalSymlinks(parentDir)
	if err != nil {
		resolvedParent = filepath.Clean(parentDir)
	}

	cleanParent := filepath.Clean(resolvedParent)
	cleanResolved := filepath.Clean(resolvedLink)

	if safety.IsCoreBlacklisted(cleanResolved) {
		return false
	}

	rel, err := filepath.Rel(cleanParent, cleanResolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

// scanAppLeftovers scans Application Support, Caches, and Containers inside libraryDir,
// screening candidates through the exclusion shield and installed AppIndex, and consolidating
// multi-directory residuals into one domain.Target per uninstalled application (D-08, D-10, D-11).
func scanAppLeftovers(ctx context.Context, libraryDir string, index *AppIndex) ([]domain.Target, error) {
	if libraryDir == "" {
		home, _ := os.UserHomeDir()
		libraryDir = filepath.Join(home, "Library")
	}

	locations := []string{
		filepath.Join(libraryDir, "Application Support"),
		filepath.Join(libraryDir, "Caches"),
		filepath.Join(libraryDir, "Containers"),
	}

	groups := make(map[string]*AppLeftoverGroup)

	for _, loc := range locations {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		entries, err := os.ReadDir(loc)
		if err != nil {
			continue // Gracefully skip non-existent or inaccessible locations
		}

		for _, e := range entries {
			fullPath := filepath.Join(loc, e.Name())
			linfo, err := os.Lstat(fullPath)
			if err != nil {
				continue
			}

			isSymlink := linfo.Mode()&os.ModeSymlink != 0
			if isSymlink {
				if !isSafeSymlink(loc, fullPath) {
					continue
				}
				st, err := os.Stat(fullPath)
				if err != nil || !st.IsDir() {
					continue
				}
			} else if !e.IsDir() {
				continue
			}

			dirName := e.Name()

			// 1. Multi-tier exclusion shield (Apple system, dedicated cleaners, CLI tools, blacklists)
			if isExcluded(dirName, fullPath) {
				continue
			}

			// 2. Installed application cross-check
			if index != nil && index.Matches(dirName) {
				continue
			}

			// Group candidates by normalized application name (D-10)
			groupKey := normalizeAppGroupName(dirName)
			if groupKey == "" {
				continue
			}

			if groups[groupKey] == nil {
				groups[groupKey] = &AppLeftoverGroup{
					AppName: groupKey,
					Paths:   []string{},
				}
			}
			groups[groupKey].Paths = append(groups[groupKey].Paths, fullPath)
		}
	}

	// Sort group keys for deterministic output ordering
	groupKeys := make([]string, 0, len(groups))
	for k := range groups {
		groupKeys = append(groupKeys, k)
	}
	sort.Strings(groupKeys)

	var targets []domain.Target
	for _, k := range groupKeys {
		g := groups[k]

		// Choose primary path preferring Application Support, then Caches, then Containers
		primaryPath := ""
		for _, p := range g.Paths {
			if strings.Contains(p, "Application Support") {
				primaryPath = p
				break
			}
		}
		if primaryPath == "" {
			for _, p := range g.Paths {
				if strings.Contains(p, "Caches") {
					primaryPath = p
					break
				}
			}
		}
		if primaryPath == "" && len(g.Paths) > 0 {
			primaryPath = g.Paths[0]
		}

		// Calculate exact disk usage across all residual directories (D-11)
		var totalAlloc, totalApparent int64
		for _, p := range g.Paths {
			usage, _ := fs.CalcUsageWithContext(ctx, p)
			totalAlloc += usage.AllocatedBytes
			totalApparent += usage.ApparentBytes
		}
		size := totalAlloc
		if size == 0 {
			size = totalApparent
		}

		targets = append(targets, domain.Target{
			ID:            fmt.Sprintf("app-leftover-%s", sanitizeID(g.AppName)),
			Name:          fmt.Sprintf("App Leftover: %s", g.AppName),
			Path:          primaryPath,
			SizeBytes:     size,
			EligibleBytes: 0,
			TTLEligible:   false,
			ReviewOnly:    true,
			ReviewReason:  domain.ReviewReasonAppLeftover,
			Category:      "leftovers-app",
			Risk:          domain.RiskLow,
			ProviderMetrics: &domain.ProviderMetrics{
				TotalCount:     len(g.Paths),
				SizeBytes:      size,
				ApparentBytes:  totalApparent,
				AllocatedBytes: totalAlloc,
				StatusNote:     "residual directories: " + strings.Join(g.Paths, ", "),
			},
		})
	}

	return targets, nil
}

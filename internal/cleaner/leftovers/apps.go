package leftovers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// AppIndex stores normalized (lowercase) sets of known bundle identifiers
// and application names for fast bidirectional cross-referencing.
type AppIndex struct {
	BundleIDs map[string]bool
	AppNames  map[string]bool
}

// defaultHomebrewRoots defines standard Homebrew Caskroom and Cellar directories across Apple Silicon and Intel paths (D-05, REQ-LFT-02).
var defaultHomebrewRoots = []string{
	"/opt/homebrew/Caskroom",
	"/opt/homebrew/Cellar",
	"/usr/local/Caskroom",
	"/usr/local/Cellar",
}

// newAppIndex allocates an empty AppIndex.
func newAppIndex() *AppIndex {
	return &AppIndex{
		BundleIDs: make(map[string]bool),
		AppNames:  make(map[string]bool),
	}
}

// addBundleID stores a trimmed, lowercased bundle identifier.
func (idx *AppIndex) addBundleID(id string) {
	trimmed := strings.ToLower(strings.TrimSpace(id))
	if trimmed != "" {
		idx.BundleIDs[trimmed] = true
	}
}

// addAppName stores a trimmed, lowercased application or executable name.
func (idx *AppIndex) addAppName(name string) {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	if trimmed != "" {
		idx.AppNames[trimmed] = true
	}
}

// Matches checks whether candidate dirName correlates with any known installed application.
// Matches against exact names, exact bundle IDs, prefix/helper relations, and trailing dot segments.
func (idx *AppIndex) Matches(dirName string) bool {
	cand := strings.ToLower(strings.TrimSpace(dirName))
	if cand == "" {
		return false
	}

	// 1. Direct name or bundle ID match
	if idx.AppNames[cand] || idx.BundleIDs[cand] {
		return true
	}

	// 2. Candidate has dot notation (e.g. reverse domain or helper)
	if strings.Contains(cand, ".") {
		// Prefix / helper process relations
		for id := range idx.BundleIDs {
			if strings.HasPrefix(cand, id) || strings.HasPrefix(id, cand) {
				return true
			}
		}

		// Extract last non-empty segment (e.g. "slack" from "org.something.slack")
		parts := strings.Split(cand, ".")
		for i := len(parts) - 1; i >= 0; i-- {
			seg := strings.TrimSpace(parts[i])
			if seg != "" {
				if idx.AppNames[seg] {
					return true
				}
				break
			}
		}
	}

	return false
}

// scanHomebrewPackages indexes installed Homebrew Caskroom and Cellar packages in pure Go (D-05, REQ-LFT-02).
func (idx *AppIndex) scanHomebrewPackages(ctx context.Context, roots []string) {
	if roots == nil {
		roots = defaultHomebrewRoots
	}
	for _, root := range roots {
		if ctx.Err() != nil {
			return
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			continue // Root does not exist or inaccessible on this architecture
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			idx.addAppName(name)

			// Version-stripped variant: if strings.Contains(name, "@"), extract prefix before @ (e.g. "dotnet-sdk@8" -> "dotnet-sdk")
			if idxAt := strings.Index(name, "@"); idxAt > 0 {
				idx.addAppName(name[:idxAt])
			}
			// Hyphen-to-space variant: if strings.Contains(name, "-"), replace hyphens with spaces (e.g. "copilot-cli" -> "copilot cli")
			if strings.Contains(name, "-") {
				idx.addAppName(strings.ReplaceAll(name, "-", " "))
			}
		}
	}
}

// scanPackageReceipts ingests package identifiers and names from receiptsDir in pure Go (D-05, REQ-LFT-02).
func (idx *AppIndex) scanPackageReceipts(ctx context.Context, receiptsDir string) {
	if receiptsDir == "" {
		receiptsDir = "/var/db/receipts"
	}
	entries, err := os.ReadDir(receiptsDir)
	if err != nil {
		return // Gracefully skip non-existent or inaccessible receipts directory
	}
	for _, e := range entries {
		if ctx.Err() != nil {
			return
		}
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".plist") {
			continue
		}
		bundleID := strings.TrimSuffix(e.Name(), ".plist")
		idx.addBundleID(bundleID)

		receiptPath := filepath.Join(receiptsDir, e.Name())
		if r, err := parseReceiptFile(receiptPath); err == nil {
			if r.PackageIdentifier != "" {
				idx.addBundleID(r.PackageIdentifier)
			}
			if r.PackageFileName != "" {
				base := strings.TrimSuffix(r.PackageFileName, ".pkg")
				idx.addAppName(base)
				if strings.Contains(base, "-") || strings.Contains(base, "_") {
					normalized := strings.ReplaceAll(base, "-", " ")
					normalized = strings.ReplaceAll(normalized, "_", " ")
					idx.addAppName(normalized)
				}
			}
		}
	}
}

// scanInstalledApps discovers installed .app bundles across the provided appRoots
// bounded to depth 3, parses Info.plist via parseAppBundleInfoFile, and populates AppIndex.
// Traversal immediately prunes upon encountering any directory ending in .app (D-05, D-06).
func scanInstalledApps(ctx context.Context, appRoots []string) (*AppIndex, error) {
	index := newAppIndex()
	index.scanHomebrewPackages(ctx, nil)
	index.scanPackageReceipts(ctx, "")

	for _, root := range appRoots {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if _, err := os.Stat(root); err != nil {
			continue // Gracefully skip non-existent root
		}

		cleanRoot := filepath.Clean(root)
		rootDepth := strings.Count(cleanRoot, string(filepath.Separator))

		walkErr := filepath.WalkDir(cleanRoot, func(path string, d os.DirEntry, err error) error {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			if err != nil {
				return nil // Skip inaccessible subpaths
			}

			// Bounded depth: max 3 levels deeper than root
			currentDepth := strings.Count(path, string(filepath.Separator))
			if currentDepth-rootDepth > 3 {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if d.IsDir() && strings.HasSuffix(d.Name(), ".app") {
				appNameNoExt := strings.TrimSuffix(d.Name(), ".app")
				index.addAppName(appNameNoExt)

				// Parse Info.plist inside bundle
				infoPlistPath := filepath.Join(path, "Contents", "Info.plist")
				if info, err := parseAppBundleInfoFile(infoPlistPath); err == nil {
					if info.CFBundleIdentifier != "" {
						index.addBundleID(info.CFBundleIdentifier)
					}
					if info.CFBundleName != "" {
						index.addAppName(info.CFBundleName)
					}
					if info.CFBundleExecutable != "" {
						index.addAppName(info.CFBundleExecutable)
					}
				}

				// CRITICAL PRUNING (D-05): do not descend into .app bundle internals
				return filepath.SkipDir
			}

			return nil
		})

		if walkErr != nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	return index, nil
}

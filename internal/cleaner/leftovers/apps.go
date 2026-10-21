package leftovers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gtkpad/devclean/internal/cleaner"
)

// SpotlightResolver abstracts bundle ID queries via macOS Spotlight metadata (D-07, REQ-LFT-02).
type SpotlightResolver interface {
	ResolveBundleID(ctx context.Context, bundleID string) (bool, error)
}

// DefaultSpotlightResolver executes mdfind with in-memory thread-safe caching (D-07).
type DefaultSpotlightResolver struct {
	exec  cleaner.CommandExecutor
	mu    sync.RWMutex
	cache map[string]bool
}

// NewDefaultSpotlightResolver constructs a DefaultSpotlightResolver with the provided CommandExecutor.
func NewDefaultSpotlightResolver(exec cleaner.CommandExecutor) *DefaultSpotlightResolver {
	if exec == nil {
		exec = &cleaner.DefaultExecutor{}
	}
	return &DefaultSpotlightResolver{
		exec:  exec,
		cache: make(map[string]bool),
	}
}

// ResolveBundleID queries Spotlight via mdfind for CFBundleIdentifier matches.
// Queries are bounded by a 2-second timeout and cached to avoid redundant subprocess invocations.
func (r *DefaultSpotlightResolver) ResolveBundleID(ctx context.Context, bundleID string) (bool, error) {
	if r == nil || r.exec == nil {
		return false, nil
	}

	r.mu.RLock()
	if cached, ok := r.cache[bundleID]; ok {
		r.mu.RUnlock()
		return cached, nil
	}
	r.mu.RUnlock()

	ctxTimeout, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	query := fmt.Sprintf("kMDItemCFBundleIdentifier == %q", bundleID)
	out, err := r.exec.Output(ctxTimeout, "mdfind", "-onlyin", "/", query)
	matched := err == nil && len(strings.TrimSpace(string(out))) > 0

	r.mu.Lock()
	r.cache[bundleID] = matched
	r.mu.Unlock()

	return matched, err
}

// AppIndex stores normalized (lowercase) sets of known bundle identifiers
// and application names for fast bidirectional cross-referencing.
type AppIndex struct {
	BundleIDs map[string]bool
	AppNames  map[string]bool
	spotlight SpotlightResolver
}

// defaultHomebrewRoots defines standard Homebrew Caskroom and Cellar directories across Apple Silicon and Intel paths (D-05, REQ-LFT-02).
var defaultHomebrewRoots = []string{
	"/opt/homebrew/Caskroom",
	"/opt/homebrew/Cellar",
	"/usr/local/Caskroom",
	"/usr/local/Cellar",
}

// versionSuffixRegex matches trailing version suffixes (e.g. "2024.1", "v2", "-1.0.0") (D-08).
var versionSuffixRegex = regexp.MustCompile(`(?i)[v._-]?\d+(\.\d+)*$`)

// newAppIndex allocates an empty AppIndex.
func newAppIndex() *AppIndex {
	return &AppIndex{
		BundleIDs: make(map[string]bool),
		AppNames:  make(map[string]bool),
	}
}

// SetSpotlightResolver configures the SpotlightResolver used for fallback bundle lookups.
func (idx *AppIndex) SetSpotlightResolver(r SpotlightResolver) {
	idx.spotlight = r
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

// Matches checks whether candidate dirName correlates with any known installed application (D-07, D-08, REQ-LFT-02).
// Evaluates direct names/bundles, normalized space/hyphen/underscore forms, version suffix trimming,
// helper prefix/suffix relations, and targeted memoized Spotlight fallback for reverse-DNS candidates.
func (idx *AppIndex) Matches(dirName string) bool {
	cand := strings.ToLower(strings.TrimSpace(dirName))
	if cand == "" {
		return false
	}

	// 1. Direct name or bundle ID match
	if idx.AppNames[cand] || idx.BundleIDs[cand] {
		return true
	}

	// 2. Normalized space/hyphen/underscore variants (e.g. "adspower_global" -> "adspower global")
	normalized := strings.ReplaceAll(cand, "_", " ")
	normalized = strings.ReplaceAll(normalized, "-", " ")
	if idx.AppNames[normalized] {
		return true
	}

	// 3. Version suffix trimming (e.g. "GoLand2024.1" -> "goland", "teams2" -> "teams")
	trimmedVersion := strings.TrimSpace(versionSuffixRegex.ReplaceAllString(cand, ""))
	if trimmedVersion != "" && trimmedVersion != cand {
		if idx.AppNames[trimmedVersion] || idx.BundleIDs[trimmedVersion] {
			return true
		}
	}
	trimmedNorm := strings.TrimSpace(versionSuffixRegex.ReplaceAllString(normalized, ""))
	if trimmedNorm != "" && trimmedNorm != normalized {
		if idx.AppNames[trimmedNorm] || idx.BundleIDs[trimmedNorm] {
			return true
		}
	}

	// 4. Candidate has dot notation (e.g. reverse domain or helper)
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
				segTrimmed := strings.TrimSpace(versionSuffixRegex.ReplaceAllString(seg, ""))
				if segTrimmed != "" && idx.AppNames[segTrimmed] {
					return true
				}
				break
			}
		}

		// 5. Targeted memoized Spotlight fallback for reverse-DNS candidates (D-07)
		if idx.spotlight != nil {
			if matched, _ := idx.spotlight.ResolveBundleID(context.Background(), cand); matched {
				idx.addBundleID(cand)
				return true
			}
		}
	} else {
		// 6. Prefix token matching for dotless candidates (e.g. "Telegram Desktop" begins with "telegram")
		tokens := strings.Fields(normalized)
		if len(tokens) > 1 && idx.AppNames[tokens[0]] {
			return true
		}
		// Match candidate that starts with an installed app name (e.g. "beepertexts" -> "beeper")
		for appName := range idx.AppNames {
			if len(appName) >= 4 && strings.HasPrefix(cand, appName) {
				return true
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

// scanInstalledAppsWithExecutor discovers installed .app bundles across the provided appRoots,
// indexes Homebrew packages and package receipts, and configures a memoized Spotlight resolver (D-05, D-07, REQ-LFT-02).
func scanInstalledAppsWithExecutor(ctx context.Context, exec cleaner.CommandExecutor, appRoots []string) (*AppIndex, error) {
	index := newAppIndex()
	if exec != nil {
		index.spotlight = NewDefaultSpotlightResolver(exec)
	}
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

// scanInstalledApps discovers installed applications using DefaultExecutor.
func scanInstalledApps(ctx context.Context, appRoots []string) (*AppIndex, error) {
	return scanInstalledAppsWithExecutor(ctx, &cleaner.DefaultExecutor{}, appRoots)
}

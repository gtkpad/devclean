package browsers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
)

// chromiumRootAppSupportLeaves contains the 12 root leaves located directly under bDef.AppSupportDir.
var chromiumRootAppSupportLeaves = []string{
	"OptGuideOnDeviceModel",
	"OptGuideOnDeviceClassifierModel",
	"optimization_guide_model_store",
	"OptimizationGuideModelsManifest",
	"OptimizationHints",
	"component_crx_cache",
	"extensions_crx_cache",
	"GraphiteDawnCache",
	"GPUPersistentCache",
	"ShaderCache",
	"GrShaderCache",
	filepath.Join("Crashpad", "completed"),
}

// chromiumProfileAppSupportLeaves contains the allowlisted leaves under prof.AppSupportPath.
var chromiumProfileAppSupportLeaves = []string{
	"GPUCache",
	"DawnCache",
	"DawnWebGPUCache",
	"DawnGraphiteCache",
	"GraphiteDawnCache",
	"GrShaderCache",
	"ShaderCache",
	"Application Cache",
	"AutofillAiModelCache",
	"optimization_guide_hint_cache_store",
	"Code Cache",
	filepath.Join("Service Worker", "CacheStorage"),
}

// chromiumProfileOSCacheLeaves contains the allowlisted leaves under prof.CachePath.
var chromiumProfileOSCacheLeaves = []string{
	"Media Cache",
	"GPUCache",
	"Code Cache",
	"image_cache",
	"DawnGraphiteCache",
	"GraphiteDawnCache",
	"GrShaderCache",
	"ShaderCache",
}

// discoverChromiumProfiles scans both Application Support/Config and OS Cache roots for profile directories.
func discoverChromiumProfiles(appSupportRoot, cacheRoot string) []ProfileInfo {
	var profiles []ProfileInfo
	seen := make(map[string]bool)

	addProfile := func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		var appPath, cPath string
		if appSupportRoot != "" {
			appPath = filepath.Join(appSupportRoot, name)
		}
		if cacheRoot != "" {
			cPath = filepath.Join(cacheRoot, name)
		}
		profiles = append(profiles, ProfileInfo{
			Name:           name,
			AppSupportPath: appPath,
			CachePath:      cPath,
		})
	}

	// 1. Scan AppSupport / Config root
	if appSupportRoot != "" {
		if entries, err := os.ReadDir(appSupportRoot); err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				name := e.Name()
				if name == "Default" || name == "System Profile" || name == "Guest Profile" || strings.HasPrefix(name, "Profile ") {
					addProfile(name)
					continue
				}
				// Also detect profiles containing a Preferences file
				if _, err := os.Stat(filepath.Join(appSupportRoot, name, "Preferences")); err == nil {
					addProfile(name)
				}
			}
		}
	}

	// 2. Scan OS Cache root for mirrored profile folders
	if cacheRoot != "" {
		if entries, err := os.ReadDir(cacheRoot); err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				name := e.Name()
				if name == "Default" || name == "System Profile" || name == "Guest Profile" || strings.HasPrefix(name, "Profile ") {
					addProfile(name)
				}
			}
		}
	}

	return profiles
}

// scanChromiumBrowser scans allowlisted cache subfolders across all discovered profiles for a Chromium browser.
func scanChromiumBrowser(
	ctx context.Context,
	homeDir, goos string,
	bDef BrowserDef,
	isProcessBlocked bool,
	cutoff time.Time,
	maxAgeDays int,
) ([]domain.Target, error) {
	if bDef.DarwinOnly && goos != "darwin" {
		return nil, nil
	}
	if bDef.AppSupportDir == nil || bDef.CacheDir == nil {
		return nil, nil
	}

	appSupportRoot := bDef.AppSupportDir(homeDir, goos)
	cacheRoot := bDef.CacheDir(homeDir, goos)
	if appSupportRoot == "" && cacheRoot == "" {
		return nil, nil
	}

	profiles := discoverChromiumProfiles(appSupportRoot, cacheRoot)

	var subPaths []string
	var totalCacheSize, eligibleCacheBytes int64
	var hasEligibleCache bool
	var latestCacheTime time.Time

	var totalScriptCacheSize int64
	var latestScriptCacheTime time.Time

	// 1. Scan root AppSupport leaves (outside individual profiles)
	if appSupportRoot != "" {
		for _, leaf := range chromiumRootAppSupportLeaves {
			p := filepath.Join(appSupportRoot, leaf)
			if IsSessionOrProtectedData(p) {
				continue
			}
			info, err := os.Stat(p)
			if err != nil {
				continue
			}
			var sz int64
			if !info.IsDir() {
				sz = info.Size()
			} else {
				sz, _ = devfs.CalcDirSize(p)
			}
			if sz == 0 {
				continue
			}

			totalCacheSize += sz
			if info.ModTime().After(latestCacheTime) {
				latestCacheTime = info.ModTime()
			}
			if maxAgeDays == 0 || info.ModTime().Before(cutoff) {
				eligibleCacheBytes += sz
				hasEligibleCache = true
			}
			subPaths = append(subPaths, p)
		}
	}

	for _, prof := range profiles {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// 2. Scan allowlisted AppSupport subfolders
		if prof.AppSupportPath != "" {
			for _, leaf := range chromiumProfileAppSupportLeaves {
				p := filepath.Join(prof.AppSupportPath, leaf)
				if IsSessionOrProtectedData(p) {
					continue
				}
				info, err := os.Stat(p)
				if err != nil {
					continue
				}
				var sz int64
				if !info.IsDir() {
					sz = info.Size()
				} else {
					sz, _ = devfs.CalcDirSize(p)
				}
				if sz == 0 {
					continue
				}

				totalCacheSize += sz
				if info.ModTime().After(latestCacheTime) {
					latestCacheTime = info.ModTime()
				}
				if maxAgeDays == 0 || info.ModTime().Before(cutoff) {
					eligibleCacheBytes += sz
					hasEligibleCache = true
				}
				subPaths = append(subPaths, p)
			}

			// Measure ScriptCache separately (BROW-04, D-05, D-06)
			scPath := filepath.Join(prof.AppSupportPath, "Service Worker", "ScriptCache")
			if info, err := os.Stat(scPath); err == nil && !IsSessionOrProtectedData(scPath) {
				var sz int64
				if !info.IsDir() {
					sz = info.Size()
				} else {
					sz, _ = devfs.CalcDirSize(scPath)
				}
				if sz > 0 {
					totalScriptCacheSize += sz
					if info.ModTime().After(latestScriptCacheTime) {
						latestScriptCacheTime = info.ModTime()
					}
				}
			}
		}

		// 3. Scan allowlisted OS Cache subfolders with hierarchical deduplication (D-05, D-06)
		if prof.CachePath != "" {
			// Check for macOS/Chromium nested <Profile>/Cache directory first
			profileCacheDir := filepath.Join(prof.CachePath, "Cache")
			if info, err := os.Stat(profileCacheDir); err == nil && info.IsDir() {
				if !IsSessionOrProtectedData(profileCacheDir) {
					sz, _ := devfs.CalcDirSize(profileCacheDir)
					if sz > 0 {
						totalCacheSize += sz
						if info.ModTime().After(latestCacheTime) {
							latestCacheTime = info.ModTime()
						}
						if maxAgeDays == 0 || info.ModTime().Before(cutoff) {
							eligibleCacheBytes += sz
							hasEligibleCache = true
						}
						subPaths = append(subPaths, profileCacheDir)
					}
				}
			} else {
				// Fall back to flat <Profile>/Cache_Data (Linux or legacy layout)
				profileCacheData := filepath.Join(prof.CachePath, "Cache_Data")
				if info, err := os.Stat(profileCacheData); err == nil && info.IsDir() {
					if !IsSessionOrProtectedData(profileCacheData) {
						sz, _ := devfs.CalcDirSize(profileCacheData)
						if sz > 0 {
							totalCacheSize += sz
							if info.ModTime().After(latestCacheTime) {
								latestCacheTime = info.ModTime()
							}
							if maxAgeDays == 0 || info.ModTime().Before(cutoff) {
								eligibleCacheBytes += sz
								hasEligibleCache = true
							}
							subPaths = append(subPaths, profileCacheData)
						}
					}
				}
			}

			for _, leaf := range chromiumProfileOSCacheLeaves {
				p := filepath.Join(prof.CachePath, leaf)
				if IsSessionOrProtectedData(p) {
					continue
				}
				info, err := os.Stat(p)
				if err != nil {
					continue
				}
				var sz int64
				if !info.IsDir() {
					sz = info.Size()
				} else {
					sz, _ = devfs.CalcDirSize(p)
				}
				if sz == 0 {
					continue
				}

				totalCacheSize += sz
				if info.ModTime().After(latestCacheTime) {
					latestCacheTime = info.ModTime()
				}
				if maxAgeDays == 0 || info.ModTime().Before(cutoff) {
					eligibleCacheBytes += sz
					hasEligibleCache = true
				}
				subPaths = append(subPaths, p)
			}
		}
	}

	var targets []domain.Target

	// Target 1: Consolidated Main Browser Caches
	if totalCacheSize > 0 {
		if latestCacheTime.IsZero() {
			latestCacheTime = time.Now()
		}
		vPath := "browser://" + bDef.ID + "/caches"
		trg := domain.Target{
			ID:             hashBrowserPath(vPath),
			Name:           bDef.Name + " Caches",
			Path:           vPath,
			SizeBytes:      totalCacheSize,
			EligibleBytes:  eligibleCacheBytes,
			TTLEligible:    hasEligibleCache,
			Risk:           domain.RiskLow,
			LastAccessed:   latestCacheTime,
			Category:       "browser-cache",
			IsConsolidated: true,
			SubPaths:       subPaths,
		}
		if isProcessBlocked {
			trg.ReviewOnly = true
			trg.ReviewReason = domain.ReviewReasonActiveProcess
			trg.EligibleBytes = 0
			trg.TTLEligible = false
		}
		targets = append(targets, trg)
	}

	// Target 2: Dedicated ReviewOnly ScriptCache Target (BROW-04, D-05, D-06)
	if totalScriptCacheSize > 0 && bDef.HasScriptCache {
		if latestScriptCacheTime.IsZero() {
			latestScriptCacheTime = time.Now()
		}
		vPath := "browser://" + bDef.ID + "/scriptcache"
		trg := domain.Target{
			ID:            hashBrowserPath(vPath),
			Name:          bDef.Name + " ScriptCache",
			Path:          vPath,
			SizeBytes:     totalScriptCacheSize,
			EligibleBytes: 0,                                // INVIOLABLE: EligibleBytes must be 0 in v2.3
			TTLEligible:   false,                            // INVIOLABLE: TTLEligible must be false
			ReviewOnly:    true,                             // INVIOLABLE: ReviewOnly must be true
			ReviewReason:  "scriptcache_requires_evidence", // Gated pending acceptance evidence
			Risk:          domain.RiskMedium,
			LastAccessed:  latestScriptCacheTime,
			Category:      "browser-scriptcache",
		}
		targets = append(targets, trg)
	}

	return targets, nil
}

// cleanChromiumBrowser surgically unlinks allowlisted cache leaves across all profiles.
// If subPaths is provided, it unlinks only those verified constituent paths.
// Otherwise, it falls back to full allowlist traversal.
func cleanChromiumBrowser(homeDir, goos string, bDef BrowserDef, subPaths ...[]string) error {
	if bDef.DarwinOnly && goos != "darwin" {
		return nil
	}
	if bDef.AppSupportDir == nil || bDef.CacheDir == nil {
		return nil
	}

	appSupportRoot := bDef.AppSupportDir(homeDir, goos)
	cacheRoot := bDef.CacheDir(homeDir, goos)

	// Branch A: Surgical cleanup of provided constituent SubPaths
	if len(subPaths) > 0 && len(subPaths[0]) > 0 {
		for _, p := range subPaths[0] {
			cleanP := filepath.Clean(p)
			if err := VerifyCleanCandidate(cleanP); err != nil {
				continue
			}
			if isLink, err := safety.IsSymlink(cleanP); err != nil || isLink {
				continue
			}
			// Strict root containment check: ensure cleanP is not container root and is within root
			if (appSupportRoot != "" && cleanP != appSupportRoot && strings.HasPrefix(cleanP, appSupportRoot+string(filepath.Separator))) ||
				(cacheRoot != "" && cleanP != cacheRoot && strings.HasPrefix(cleanP, cacheRoot+string(filepath.Separator))) {
				_ = os.RemoveAll(cleanP)
			}
		}
		return nil
	}

	// Branch B: Fallback traversal across root leaves and discovered profiles (backward compatibility)
	// 1. Root AppSupport leaves
	if appSupportRoot != "" {
		for _, leaf := range chromiumRootAppSupportLeaves {
			p := filepath.Join(appSupportRoot, leaf)
			if err := VerifyCleanCandidate(p); err != nil {
				continue
			}
			if isLink, err := safety.IsSymlink(p); err == nil && !isLink {
				_ = os.RemoveAll(p)
			}
		}
	}

	// 2. Profile AppSupport & OS Cache leaves
	profiles := discoverChromiumProfiles(appSupportRoot, cacheRoot)
	for _, prof := range profiles {
		// AppSupport allowlisted leaves
		if prof.AppSupportPath != "" {
			for _, leaf := range chromiumProfileAppSupportLeaves {
				p := filepath.Join(prof.AppSupportPath, leaf)
				if err := VerifyCleanCandidate(p); err != nil {
					continue
				}
				if isLink, err := safety.IsSymlink(p); err == nil && !isLink {
					_ = os.RemoveAll(p)
				}
			}
		}

		// OS Cache allowlisted leaves (including nested Cache / Cache_Data)
		if prof.CachePath != "" {
			nestedCache := filepath.Join(prof.CachePath, "Cache")
			if info, err := os.Stat(nestedCache); err == nil && info.IsDir() {
				if err := VerifyCleanCandidate(nestedCache); err == nil {
					if isLink, err := safety.IsSymlink(nestedCache); err == nil && !isLink {
						_ = os.RemoveAll(nestedCache)
					}
				}
			}
			flatCacheData := filepath.Join(prof.CachePath, "Cache_Data")
			if info, err := os.Stat(flatCacheData); err == nil && info.IsDir() {
				if err := VerifyCleanCandidate(flatCacheData); err == nil {
					if isLink, err := safety.IsSymlink(flatCacheData); err == nil && !isLink {
						_ = os.RemoveAll(flatCacheData)
					}
				}
			}

			for _, leaf := range chromiumProfileOSCacheLeaves {
				p := filepath.Join(prof.CachePath, leaf)
				if err := VerifyCleanCandidate(p); err != nil {
					continue
				}
				if isLink, err := safety.IsSymlink(p); err == nil && !isLink {
					_ = os.RemoveAll(p)
				}
			}
		}
	}

	return nil
}

// hashBrowserPath produces a deterministic 16-character hex hash from a virtual path.
func hashBrowserPath(path string) string {
	h := sha256.Sum256([]byte(path))
	return hex.EncodeToString(h[:8])
}

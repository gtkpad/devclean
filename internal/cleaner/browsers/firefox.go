package browsers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
)

// discoverFirefoxProfiles discovers profile folders matching *.default*, *.release*, or containing prefs.js.
func discoverFirefoxProfiles(appSupportRoot, cacheRoot string) []ProfileInfo {
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

	if appSupportRoot != "" {
		if entries, err := os.ReadDir(appSupportRoot); err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				name := e.Name()
				if strings.Contains(name, ".default") || strings.Contains(name, ".release") {
					addProfile(name)
					continue
				}
				if _, err := os.Stat(filepath.Join(appSupportRoot, name, "prefs.js")); err == nil {
					addProfile(name)
				}
			}
		}
	}

	if cacheRoot != "" {
		if entries, err := os.ReadDir(cacheRoot); err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				name := e.Name()
				if strings.Contains(name, ".default") || strings.Contains(name, ".release") {
					addProfile(name)
				}
			}
		}
	}

	return profiles
}

// scanFirefox scans allowlisted cache subdirectories across all Firefox profiles.
func scanFirefox(
	ctx context.Context,
	homeDir, goos string,
	bDef BrowserDef,
	isProcessBlocked bool,
	cutoff time.Time,
	maxAgeDays int,
) ([]domain.Target, error) {
	if bDef.AppSupportDir == nil || bDef.CacheDir == nil {
		return nil, nil
	}

	appSupportRoot := bDef.AppSupportDir(homeDir, goos)
	cacheRoot := bDef.CacheDir(homeDir, goos)
	if appSupportRoot == "" && cacheRoot == "" {
		return nil, nil
	}

	profiles := discoverFirefoxProfiles(appSupportRoot, cacheRoot)
	if len(profiles) == 0 {
		return nil, nil
	}

	var totalCacheSize, eligibleCacheBytes int64
	var hasEligibleCache bool
	var latestCacheTime time.Time

	for _, prof := range profiles {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// 1. AppSupport allowlisted leaves
		if prof.AppSupportPath != "" {
			appLeaves := []string{"startupCache", "jumpListCache"}
			for _, leaf := range appLeaves {
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
			}
		}

		// 2. OS Cache allowlisted leaves (e.g. ~/Library/Caches/Firefox/Profiles/<profile>/cache2)
		if prof.CachePath != "" {
			cacheLeaves := []string{"cache2", "startupCache", "thumbnails"}
			for _, leaf := range cacheLeaves {
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
			}
		}
	}

	if totalCacheSize == 0 {
		return nil, nil
	}

	if latestCacheTime.IsZero() {
		latestCacheTime = time.Now()
	}

	vPath := "browser://firefox/caches"
	trg := domain.Target{
		ID:            hashBrowserPath(vPath),
		Name:          "Mozilla Firefox Caches",
		Path:          vPath,
		SizeBytes:     totalCacheSize,
		EligibleBytes: eligibleCacheBytes,
		TTLEligible:   hasEligibleCache,
		Risk:          domain.RiskLow,
		LastAccessed:  latestCacheTime,
		Category:      "browser-cache",
	}

	if isProcessBlocked {
		trg.ReviewOnly = true
		trg.ReviewReason = domain.ReviewReasonActiveProcess
		trg.EligibleBytes = 0
		trg.TTLEligible = false
	}

	// Strictly ZERO ScriptCache targets for Firefox (BROW-04, D-08)
	return []domain.Target{trg}, nil
}

// cleanFirefox removes allowlisted cache leaves while strictly preserving SQLite databases.
func cleanFirefox(homeDir, goos string, bDef BrowserDef) error {
	if bDef.AppSupportDir == nil || bDef.CacheDir == nil {
		return nil
	}

	appSupportRoot := bDef.AppSupportDir(homeDir, goos)
	cacheRoot := bDef.CacheDir(homeDir, goos)
	profiles := discoverFirefoxProfiles(appSupportRoot, cacheRoot)

	for _, prof := range profiles {
		// 1. AppSupport allowlisted leaves
		if prof.AppSupportPath != "" {
			for _, leaf := range []string{"startupCache", "jumpListCache"} {
				p := filepath.Join(prof.AppSupportPath, leaf)
				if err := VerifyCleanCandidate(p); err != nil {
					continue
				}
				if isLink, err := safety.IsSymlink(p); err == nil && !isLink {
					_ = os.RemoveAll(p)
				}
			}
		}

		// 2. OS Cache allowlisted leaves
		if prof.CachePath != "" {
			for _, leaf := range []string{"cache2", "startupCache", "thumbnails"} {
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

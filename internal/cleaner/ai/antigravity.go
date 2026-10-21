package ai

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
)

// antigravityRootLeaves contains root-level cache and model store directories under ~/.gemini/antigravity-browser-profile.
var antigravityRootLeaves = []string{
	"GraphiteDawnCache",
	"ShaderCache",
	"GrShaderCache",
	"component_crx_cache",
	"extensions_crx_cache",
	"optimization_guide_model_store",
}

// antigravityProfileLeaves contains ephemeral cache directories inside each profile directory (e.g. Default/).
var antigravityProfileLeaves = []string{
	"Cache",
	"Code Cache",
	"GPUCache",
	"DawnGraphiteCache",
	"DawnWebGPUCache",
	filepath.Join("Service Worker", "CacheStorage"),
	filepath.Join("Service Worker", "ScriptCache"),
}

// scanAntigravityBrowserProfile discovers and aggregates Chromium-based profile ephemeral caches
// under ~/.gemini/antigravity-browser-profile into a single consolidated target.
func scanAntigravityBrowserProfile(ctx context.Context, exec cleaner.CommandExecutor, home string, rules domain.EcosystemRules) ([]domain.Target, error) {
	if home == "" {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	profileRoot := filepath.Join(home, ".gemini", "antigravity-browser-profile")
	if info, err := os.Stat(profileRoot); err != nil || !info.IsDir() {
		return nil, nil
	}

	var subPaths []string
	var totalSize int64
	var eligibleBytes int64
	var hasEligible bool
	var latestModTime time.Time

	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	// 1. Scan root-level leaves
	for _, leaf := range antigravityRootLeaves {
		p := filepath.Join(profileRoot, leaf)
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		var sz int64
		if info.IsDir() {
			sz, _ = devfs.CalcDirSize(p)
		} else {
			sz = info.Size()
		}
		if sz == 0 {
			continue
		}

		totalSize += sz
		subPaths = append(subPaths, p)
		if info.ModTime().After(latestModTime) {
			latestModTime = info.ModTime()
		}
		if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
			eligibleBytes += sz
			hasEligible = true
		}
	}

	// 2. Discover profile folders under profileRoot
	var profileDirs []string
	seen := make(map[string]bool)
	if entries, err := os.ReadDir(profileRoot); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if name == "Default" || name == "System Profile" || name == "Guest Profile" || strings.HasPrefix(name, "Profile ") {
				if !seen[name] {
					seen[name] = true
					profileDirs = append(profileDirs, filepath.Join(profileRoot, name))
				}
				continue
			}
			// Profile with Preferences file
			if _, err := os.Stat(filepath.Join(profileRoot, name, "Preferences")); err == nil {
				if !seen[name] {
					seen[name] = true
					profileDirs = append(profileDirs, filepath.Join(profileRoot, name))
				}
			}
		}
	}

	// 3. Scan profile-level leaves
	for _, profDir := range profileDirs {
		for _, leaf := range antigravityProfileLeaves {
			p := filepath.Join(profDir, leaf)
			info, err := os.Stat(p)
			if err != nil {
				continue
			}
			var sz int64
			if info.IsDir() {
				sz, _ = devfs.CalcDirSize(p)
			} else {
				sz = info.Size()
			}
			if sz == 0 {
				continue
			}

			totalSize += sz
			subPaths = append(subPaths, p)
			if info.ModTime().After(latestModTime) {
				latestModTime = info.ModTime()
			}
			if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
				eligibleBytes += sz
				hasEligible = true
			}
		}
	}

	if totalSize == 0 {
		return nil, nil
	}

	if latestModTime.IsZero() {
		latestModTime = time.Now()
	}

	running, uncertain := checkProcessStatus(ctx, exec, "Antigravity", "antigravity", "antigravity-browser")
	isBlocked := running || uncertain

	trg := domain.Target{
		ID:             hashPath("ai://antigravity-browser-profile/cache"),
		Name:           "Antigravity: Browser Profile Cache",
		Path:           "ai://antigravity-browser-profile/cache",
		SizeBytes:      totalSize,
		EligibleBytes:  eligibleBytes,
		TTLEligible:    hasEligible,
		Risk:           domain.RiskLow,
		LastAccessed:   latestModTime,
		Category:       "ai-antigravity-browser-cache",
		IsConsolidated: true,
		SubPaths:       subPaths,
	}

	if isBlocked {
		trg.ReviewOnly = true
		trg.ReviewReason = domain.ReviewReasonActiveProcess
		trg.EligibleBytes = 0
		trg.TTLEligible = false
	}

	return []domain.Target{trg}, nil
}

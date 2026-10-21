package macos

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/fs"
)

type daemonPathDef struct {
	pathRel   string // Path relative to ~/Library
	childOnly bool   // Purge child contents only, preserving parent folder (e.g. Data/tmp)
}

type canonicalDaemonDef struct {
	id       string
	name     string
	bundleID string
	paths    []daemonPathDef
	alwaysOn bool

	// Compatibility aliases for macos.go prior to Task 2 refactor
	ID       string
	BundleID string
}

// The canonical 9 Apple daemons supporting real multi-root paths (D-09, D-10, REQ-MAC-01).
var canonicalDaemons = []canonicalDaemonDef{
	{
		id:       "siri",
		name:     "Siri Suggestions & TTS",
		bundleID: "com.apple.siri.context",
		paths: []daemonPathDef{
			{pathRel: "Caches/com.apple.siri.context", childOnly: false},
			{pathRel: "Suggestions", childOnly: false},
		},
		alwaysOn: true,
		ID:       "siri",
		BundleID: "com.apple.siri.context",
	},
	{
		id:       "mediaanalysisd",
		name:     "Media Analysis Daemon",
		bundleID: "com.apple.mediaanalysisd",
		paths: []daemonPathDef{
			{pathRel: "Caches/com.apple.mediaanalysisd", childOnly: false},
			{pathRel: "Containers/com.apple.mediaanalysisd/Data/tmp", childOnly: true},
		},
		alwaysOn: true,
		ID:       "mediaanalysisd",
		BundleID: "com.apple.mediaanalysisd",
	},
	{
		id:       "geod",
		name:     "Location Services (geod)",
		bundleID: "com.apple.geod",
		paths: []daemonPathDef{
			{pathRel: "Caches/com.apple.geod", childOnly: false},
			{pathRel: "Containers/com.apple.geod/Data/tmp", childOnly: true},
		},
		alwaysOn: true,
		ID:       "geod",
		BundleID: "com.apple.geod",
	},
	{
		id:       "geoservices",
		name:     "macOS GeoServices",
		bundleID: "com.apple.geoservices",
		paths: []daemonPathDef{
			{pathRel: "Caches/GeoServices", childOnly: false},
		},
		alwaysOn: true,
		ID:       "geoservices",
		BundleID: "com.apple.geoservices",
	},
	{
		id:       "handoff",
		name:     "Handoff & Universal Clipboard",
		bundleID: "com.apple.coreservices.useractivityd",
		paths: []daemonPathDef{
			{pathRel: "Caches/com.apple.coreservices.useractivityd", childOnly: false},
			{pathRel: "Group Containers/group.com.apple.coreservices.useractivityd/shared-pasteboard", childOnly: true},
		},
		alwaysOn: true,
		ID:       "handoff",
		BundleID: "com.apple.coreservices.useractivityd",
	},
	{
		id:       "wallpaper",
		name:     "macOS Wallpaper Agent",
		bundleID: "com.apple.WallpaperAgent",
		paths: []daemonPathDef{
			{pathRel: "Caches/com.apple.WallpaperAgent", childOnly: false},
		},
		alwaysOn: true,
		ID:       "wallpaper",
		BundleID: "com.apple.WallpaperAgent",
	},
	{
		id:       "helpd",
		name:     "macOS Help Viewer",
		bundleID: "com.apple.helpd",
		paths: []daemonPathDef{
			{pathRel: "Caches/com.apple.helpd", childOnly: false},
		},
		alwaysOn: true,
		ID:       "helpd",
		BundleID: "com.apple.helpd",
	},
	{
		id:       "ams",
		name:     "Apple Media Services",
		bundleID: "com.apple.AppleMediaServices",
		paths: []daemonPathDef{
			{pathRel: "Caches/com.apple.AppleMediaServices", childOnly: false},
		},
		alwaysOn: true,
		ID:       "ams",
		BundleID: "com.apple.AppleMediaServices",
	},
	{
		id:       "parsecd",
		name:     "Core Parsec Search",
		bundleID: "com.apple.parsecd",
		paths: []daemonPathDef{
			{pathRel: "Caches/com.apple.parsecd", childOnly: false},
		},
		alwaysOn: true,
		ID:       "parsecd",
		BundleID: "com.apple.parsecd",
	},
}

// scanDaemons scans strictly the canonical 9 Apple daemons supporting real multi-root paths
// and unconditionally unblocks always-on Apple daemons managed by launchd (D-09, D-10, REQ-MAC-01, REQ-MAC-02).
func scanDaemons(ctx context.Context, exec cleaner.CommandExecutor, homeDir string) ([]domain.Target, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	libRoot := filepath.Join(homeDir, "Library")
	if _, err := os.Stat(libRoot); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var targets []domain.Target

	for _, d := range canonicalDaemons {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var subPaths []string
		var totalSize int64
		var latestModTime time.Time

		for _, pDef := range d.paths {
			fullPath := filepath.Join(libRoot, pDef.pathRel)
			info, err := os.Stat(fullPath)
			if err != nil || !info.IsDir() {
				continue
			}

			usage, _ := fs.CalcUsage(fullPath)
			size := usage.AllocatedBytes
			if size == 0 {
				size = usage.ApparentBytes
				if size == 0 {
					size = info.Size()
				}
			}
			if size > 0 {
				totalSize += size
				subPaths = append(subPaths, fullPath)
				if info.ModTime().After(latestModTime) {
					latestModTime = info.ModTime()
				}
			}
		}

		if totalSize == 0 || len(subPaths) == 0 {
			continue
		}

		primaryPath := subPaths[0]
		targets = append(targets, domain.Target{
			ID:             "macos-daemon-" + d.id,
			Name:           fmt.Sprintf("macOS Daemon: %s", d.name),
			Path:           primaryPath,
			SizeBytes:      totalSize,
			EligibleBytes:  totalSize, // Always-on: 100% eligible, managed by launchd (D-09, REQ-MAC-02)
			TTLEligible:    true,
			ReviewOnly:     false,
			ReviewReason:   "",
			Risk:           domain.RiskLow,
			Category:       "macos-daemon-caches",
			LastAccessed:   latestModTime,
			IsConsolidated: len(subPaths) > 1,
			SubPaths:       subPaths,
		})
	}

	return targets, nil
}

package macos

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/fs"
)

// The canonical 8 Apple daemons defined in D-13 and SPEC §Frente 3.
var canonicalDaemons = []struct {
	ID       string
	Name     string
	PathRel  string // Relative to ~/Library/Caches
	BundleID string
}{
	{"siri", "Siri Suggestions & TTS", "com.apple.siri.context", "com.apple.siri.context"},
	{"wallpaper", "macOS Wallpaper Agent", "com.apple.WallpaperAgent", "com.apple.WallpaperAgent"},
	{"mediaanalysisd", "Media Analysis Daemon", "com.apple.mediaanalysisd", "com.apple.mediaanalysisd"},
	{"geod", "Location Services (geod)", "com.apple.geod", "com.apple.geod"},
	{"helpd", "macOS Help Viewer", "com.apple.helpd", "com.apple.helpd"},
	{"ams", "Apple Media Services", "com.apple.AppleMediaServices", "com.apple.AppleMediaServices"},
	{"parsecd", "Core Parsec Search", "com.apple.parsecd", "com.apple.parsecd"},
	{"handoff", "Handoff & Universal Clipboard", "com.apple.coreservices.useractivityd", "com.apple.coreservices.useractivityd"},
}

// scanDaemons scans strictly the canonical 8 Apple daemons with process guard protection (D-13).
func scanDaemons(ctx context.Context, exec cleaner.CommandExecutor, homeDir string) ([]domain.Target, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cachesRoot := filepath.Join(homeDir, "Library", "Caches")
	if _, err := os.Stat(cachesRoot); err != nil {
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

		fullPath := filepath.Join(cachesRoot, d.PathRel)
		info, err := os.Stat(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
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

		running, uncertain := IsBundleProcessRunning(ctx, exec, d.BundleID)
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
			ID:            "macos-daemon-" + d.ID,
			Name:          fmt.Sprintf("macOS Daemon: %s", d.Name),
			Path:          fullPath,
			SizeBytes:     size,
			EligibleBytes: eligible,
			TTLEligible:   !reviewOnly,
			ReviewOnly:    reviewOnly,
			ReviewReason:  reason,
			Risk:          domain.RiskLow,
			Category:      "macos-daemon-caches",
		})
	}

	return targets, nil
}

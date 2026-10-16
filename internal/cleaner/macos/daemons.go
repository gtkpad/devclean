package macos

import (
	"context"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
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
	// Stub for TDD RED phase
	return nil, nil
}

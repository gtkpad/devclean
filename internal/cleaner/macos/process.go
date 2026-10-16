package macos

import (
	"context"

	"github.com/gtkpad/devclean/internal/cleaner"
)

// Known mapping from bundle IDs and daemon identifiers to executable process names (D-14).
var knownProcessMap = map[string][]string{
	"com.spotify.client":                   {"Spotify"},
	"com.apple.mediaanalysisd":             {"mediaanalysisd"},
	"com.apple.geod":                       {"geod"},
	"com.apple.helpd":                      {"helpd"},
	"com.apple.WallpaperAgent":             {"WallpaperAgent", "wallpaper"},
	"com.apple.parsecd":                    {"parsecd"},
	"com.apple.AppleMediaServices":         {"amsaccountsd", "appstoreagent"},
	"com.apple.coreservices.useractivityd": {"useractivityd"},
	"com.google.Chrome.code_sign_clone":    {"Google Chrome"},
	"org.chromium.Chromium.code_sign_clone": {"Chromium"},
}

// CheckProcessStatus returns running=true if pgrep finds matching processes.
// If pgrep fails with exit code 1, running=false, uncertain=false (cleanly stopped).
// Any other exit code or execution error returns uncertain=true (FAIL CLOSED!).
func CheckProcessStatus(ctx context.Context, exec cleaner.CommandExecutor, processNames ...string) (running bool, uncertain bool) {
	return false, false // stub for RED test phase
}

// IsBundleProcessRunning resolves mapped process names for a bundle/daemon ID and checks running status.
func IsBundleProcessRunning(ctx context.Context, exec cleaner.CommandExecutor, bundleOrDaemonID string) (running bool, uncertain bool) {
	return false, false // stub for RED test phase
}

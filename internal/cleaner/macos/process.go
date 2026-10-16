package macos

import (
	"context"
	"errors"
	"strings"

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
	if exec == nil {
		return false, true
	}
	for _, name := range processNames {
		out, err := exec.Output(ctx, "pgrep", "-x", name)
		if err == nil && len(strings.TrimSpace(string(out))) > 0 {
			return true, false
		}
		var exitErr interface{ ExitCode() int }
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			continue
		}
		// Unexpected error (pgrep missing, permission error, exit code != 1) -> FAIL CLOSED
		return false, true
	}
	return false, false
}

// IsBundleProcessRunning resolves mapped process names for a bundle/daemon ID and checks running status.
func IsBundleProcessRunning(ctx context.Context, exec cleaner.CommandExecutor, bundleOrDaemonID string) (running bool, uncertain bool) {
	if exec == nil {
		return false, true
	}
	names, ok := knownProcessMap[bundleOrDaemonID]
	if !ok {
		// If bundle has no explicit process mapping, trim prefixes to get base name
		base := bundleOrDaemonID
		if strings.HasPrefix(base, "com.apple.") {
			base = strings.TrimPrefix(base, "com.apple.")
		} else if strings.HasPrefix(base, "com.") {
			base = strings.TrimPrefix(base, "com.")
		}
		names = []string{base}
	}
	return CheckProcessStatus(ctx, exec, names...)
}

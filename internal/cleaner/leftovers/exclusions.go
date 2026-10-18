package leftovers

import (
	"strings"

	"github.com/gtkpad/devclean/internal/cleaner/macos"
	"github.com/gtkpad/devclean/internal/safety"
)

var appleSystemDirNames = map[string]bool{
	"addressbook":             true,
	"animoji":                 true,
	"app store":               true,
	"callhistorydb":           true,
	"callhistorytransactions": true,
	"categories":              true,
	"clouddocs":               true,
	"controlcenter":           true,
	"crashreporter":           true,
	"differentialprivacy":     true,
	"diskimages":              true,
	"facetime":                true,
	"fileprovider":            true,
	"knowledge":               true,
	"passes":                  true,
	"quicklook":               true,
	"safari":                  true,
	"screencontinuity":        true,
	"siri":                    true,
	"syncservices":            true,
	"apple":                   true,
	"apple push service":      true,
}

var dedicatedCleanerDirs = map[string]bool{
	"pnpm":                   true,
	"uv":                     true,
	"pip":                    true,
	"homebrew":               true,
	"cocoapods":              true,
	"vcpkg":                  true,
	"mise":                   true,
	"gh":                     true,
	"ms-playwright":          true,
	"cypress":                true,
	"node":                   true,
	"yarn":                   true,
	"dev.orbstack.orbstack":  true,
	"docker":                 true,
	"coresimulator":          true,
	"developer":              true,
	"google":                 true,
	"bravesoftware":          true,
	"company.thebrowser.arc": true,
	"microsoft edge":         true,
	"firefox":                true,
	"code":                   true,
	"cursor":                 true,
	"antigravity":            true,
	"windsurf":               true,
	"claude":                 true,
	"opencode":               true,
	"zed":                    true,
}

var commonCLITools = map[string]bool{
	"git":        true,
	"gnupg":      true,
	"gpg":        true,
	"ssh":        true,
	"zsh":        true,
	"oh-my-zsh":  true,
	"pre-commit": true,
	"cargo":      true,
	"rustup":     true,
	"go":         true,
	"bun":        true,
	"brew":       true,
	"lima":       true,
	"colima":     true,
	"podman":     true,
	"tmux":       true,
	"nvim":       true,
	"vim":        true,
	"subversion": true,
}

// isExcluded evaluates whether a directory name or path represents an Apple component,
// a dedicated DevClean cleaner directory, a common CLI tool, a blacklisted path, or a macOS whitelist entry (D-07).
func isExcluded(name, fullPath string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return true
	}
	if strings.HasPrefix(lower, "com.apple.") || strings.HasPrefix(lower, "apple") || appleSystemDirNames[lower] {
		return true
	}
	if dedicatedCleanerDirs[lower] || commonCLITools[lower] {
		return true
	}
	if fullPath != "" {
		if safety.IsCoreBlacklisted(fullPath) || macos.IsWhitelisted(fullPath) {
			return true
		}
	}
	return false
}

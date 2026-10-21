package leftovers

import (
	"os/exec"
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

	// Active macOS daemons and private background agents (D-01, REQ-LFT-01)
	"passkit":              true,
	"spotlight":            true,
	"icloud":               true,
	"icloudmailagent":      true,
	"familycircled":        true,
	"contactsd":            true,
	"homeenergyd":          true,
	"icdd":                 true,
	"identityservicesd":    true,
	"intelligenceflow":     true,
	"locationaccessstored": true,
	"networkserviceproxy":  true,
	"stickersd":            true,
	"tipsd":                true,

	// Telemetry and system logging tokens
	"cctclearcutlogger": true,
	"gippseudonymousid": true,
	"phtphenotype":      true,
	"ssu":               true,

	// OS and SDK debug symbol stores and persistent storage (D-03, REQ-LFT-01)
	"symbols":             true,
	"refsrcsymbols":       true,
	"symbolsourcesymbols": true,
	"sesstorage":          true,
	"isolatedstorage":     true,
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

	// Developer toolchains, telemetry frameworks, and SDK folders (D-06, REQ-LFT-02)
	"dotnet":                               true,
	"microsoft":                            true,
	"microsoft devdiv":                     true,
	"xamarin":                              true,
	"jetbrains":                            true,
	"ai-memory":                            true,
	"caramelo":                             true,
	"archon":                               true,
	"csdevkit":                             true,
	"tabnine":                              true,
	"iterative":                            true,
	"io.branch":                            true,
	"io.sentry":                            true,
	"sentrycrash":                          true,
	"com.github.domt4.homebrew-autoupdate": true,
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

	// Active command-line utilities lacking .app bundles (D-06, REQ-LFT-02)
	"gopls":             true,
	"golangci-lint":     true,
	"copilot":           true,
	"claude":            true,
	"claude-3p":         true,
	"claude-cli-nodejs": true,
	"codex":             true,
	"openai":            true,
	"fastmcp":           true,
	"firecrawl-cli":     true,
	"gitkrakencli":      true,
	"gk":                true,
	"k9s":               true,
	"rtk":               true,
	"turborepo":         true,
}

// isExcluded evaluates whether a directory name or path represents an Apple component,
// a dedicated DevClean cleaner directory, a common CLI tool, a blacklisted path, or a macOS whitelist entry (D-07).
func isExcluded(name, fullPath string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return true
	}
	if strings.HasPrefix(lower, "com.apple.") ||
		strings.HasPrefix(lower, "apple") ||
		strings.HasPrefix(lower, "group.com.apple.") ||
		appleSystemDirNames[lower] {
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
	// Dynamic LookPath fallback for dotless candidate names (D-06, REQ-LFT-02)
	if !strings.Contains(lower, ".") {
		if _, err := exec.LookPath(lower); err == nil {
			return true
		}
	}
	return false
}

package macos

import (
	"path/filepath"
	"strings"

	"github.com/gtkpad/devclean/internal/safety"
)

// Immutable list of sensitive patterns and bundles that must NEVER be cleaned.
// Inviolable in Go code; cannot be disabled by flags or config (D-15).
var immutableSensitivePatterns = []string{
	// Apple Sync, Cloud & Identity
	"*cloudkit*",
	"*CloudKit*",
	"*mobile documents*",
	"*Mobile Documents*",
	"*MobileDocuments*",
	"*mobile.documents*",
	"com.apple.keychain*",
	"*keychain*",
	"*Keychain*",
	"com.apple.Safari*",
	"Safari",

	// Password Managers & Vaults
	"*1password*",
	"com.1password.*",
	"com.agilebits.*",
	"*bitwarden*",
	"com.bitwarden.*",
	"*keepassxc*",
	"org.keepassxc.*",
	"com.lastpass.*",
	"com.dashlane.*",

	// Mesh & Corporate VPNs
	"*tailscale*",
	"io.tailscale.*",
	"*wireguard*",
	"com.wireguard.*",
	"*zerotier*",
	"com.zerotier.*",
	"*1dot1dot1dot1*", // Cloudflare WARP
	"*cloudflare*warp*",
}

// IsWhitelisted checks whether the path or bundle identifier matches any
// immutable protection pattern. Returns true if protected (must NOT be touched).
func IsWhitelisted(path string) bool {
	clean := filepath.Clean(path)
	base := filepath.Base(clean)
	lowerPath := strings.ToLower(clean)
	lowerBase := strings.ToLower(base)

	// Extract path components for granular matching
	components := strings.FieldsFunc(lowerPath, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	if len(components) == 0 {
		components = []string{lowerBase}
	}

	for _, pattern := range immutableSensitivePatterns {
		lowerPat := strings.ToLower(pattern)

		// 1. Direct match on base name
		if matched, _ := filepath.Match(lowerPat, lowerBase); matched {
			return true
		}

		// 2. Direct match on clean path
		if matched, _ := filepath.Match(lowerPat, lowerPath); matched {
			return true
		}

		// 3. Match against each individual path component
		for _, comp := range components {
			if matched, _ := filepath.Match(lowerPat, comp); matched {
				return true
			}
		}

		// 4. Substring matching for wildcard patterns
		sub := strings.Trim(lowerPat, "*")
		if sub != "" && !strings.Contains(sub, "*") {
			if strings.Contains(lowerPath, sub) {
				return true
			}
		}
	}

	return false
}

// isSafeSymlink verifies that a symlink resolves to a target within cacheRoot
// and does not point to a blacklisted system root (D-16).
func isSafeSymlink(cacheRoot, linkPath string) bool {
	resolvedLink, err := filepath.EvalSymlinks(linkPath)
	if err != nil {
		return false
	}
	resolvedRoot, err := filepath.EvalSymlinks(cacheRoot)
	if err != nil {
		resolvedRoot = filepath.Clean(cacheRoot)
	}

	cleanRoot := filepath.Clean(resolvedRoot)
	cleanResolved := filepath.Clean(resolvedLink)

	if safety.IsCoreBlacklisted(cleanResolved) {
		return false
	}

	rel, err := filepath.Rel(cleanRoot, cleanResolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

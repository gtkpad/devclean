package browsers

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gtkpad/devclean/internal/safety"
)

// Immutable session tokens and database patterns that must NEVER be measured or deleted.
var protectedSessionTokens = []string{
	"cookies",
	"cookies-journal",
	"history",
	"history-journal",
	"login data",
	"login data-journal",
	"web data",
	"web data-journal",
	"local storage",
	"indexeddb",
	"sessions",
	"session storage",
	"current session",
	"current tabs",
	"last session",
	"last tabs",
	"bookmarks",
	"preferences",
	"secure preferences",
	"places.sqlite",
	"places.sqlite-wal",
	"key4.db",
	"logins.json",
	"sessionstore",
	"favicons",
	"favicons.sqlite",
	"form history",
	"formhistory.sqlite",
	"lastsession.plist",
	"tabs",
	"cloudtabs",
}

// IsSessionOrProtectedData returns true if the candidate path matches any personal,
// authentication, browsing history, session database pattern, or the private Safari library.
func IsSessionOrProtectedData(path string) bool {
	clean := filepath.Clean(path)
	if safety.IsCoreBlacklisted(clean) {
		return true
	}

	lower := strings.ToLower(clean)

	// Inviolable Safari private state shield (D-13): ~/Library/Safari is NEVER touched!
	if strings.Contains(lower, "library/safari") || strings.Contains(lower, "library\\safari") {
		return true
	}

	base := strings.ToLower(filepath.Base(clean))
	for _, tok := range protectedSessionTokens {
		if base == tok || strings.HasPrefix(base, tok) {
			return true
		}
	}

	// Granular component inspection
	parts := strings.FieldsFunc(lower, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	for _, part := range parts {
		// Inviolable user-installed extensions directory (<Profile>/Extensions)
		if part == "extensions" {
			return true
		}
		for _, tok := range protectedSessionTokens {
			if part == tok || strings.HasPrefix(part, tok) {
				return true
			}
		}
	}

	return false
}

// VerifyCleanCandidate validates that a candidate path is safe to delete.
func VerifyCleanCandidate(path string) error {
	if IsSessionOrProtectedData(path) {
		return fmt.Errorf("security violation: path %s matched session invariant guard", path)
	}
	if safety.IsCoreBlacklisted(path) {
		return fmt.Errorf("security violation: path %s is core blacklisted", path)
	}
	return nil
}

//go:build !darwin

package macos

import (
	"os"
)

// isOwnedByCurrentUser fallback for non-Darwin environments (used in portable tests).
func isOwnedByCurrentUser(info os.FileInfo) bool {
	return true
}

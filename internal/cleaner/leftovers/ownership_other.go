//go:build !darwin

package leftovers

import (
	"os"
)

// isAllowedOwnership fallback for non-Darwin environments (used in portable tests).
func isAllowedOwnership(info os.FileInfo, isSystemDir bool) bool {
	return true
}

// isOwnedByCurrentUser fallback for non-Darwin environments (used in portable tests).
func isOwnedByCurrentUser(info os.FileInfo) bool {
	return true
}

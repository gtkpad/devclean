//go:build !darwin

package leftovers

import (
	"os"
)

// isOwnedByCurrentUser fallback for non-Darwin environments (used in portable tests).
func isOwnedByCurrentUser(info os.FileInfo) bool {
	return true
}

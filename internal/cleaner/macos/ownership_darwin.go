//go:build darwin

package macos

import (
	"os"
)

// isOwnedByCurrentUser verifies that the filesystem item is owned by the
// UID of the current executing process (D-16). Zero CGO.
func isOwnedByCurrentUser(info os.FileInfo) bool {
	return false // stub for RED test
}

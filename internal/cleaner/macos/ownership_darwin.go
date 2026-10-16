//go:build darwin

package macos

import (
	"os"
	"syscall"
)

// isOwnedByCurrentUser verifies that the filesystem item is owned by the
// UID of the current executing process (D-16). Compiles cleanly with CGO_ENABLED=0.
func isOwnedByCurrentUser(info os.FileInfo) bool {
	if info == nil {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return true // Fallback if stat cannot be extracted
	}
	return stat.Uid == uint32(os.Getuid())
}

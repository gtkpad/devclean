//go:build darwin

package leftovers

import (
	"os"
	"syscall"
)

// isAllowedOwnership checks whether file ownership matches valid macOS administration semantics (D-09).
// In system directories (/Library/LaunchAgents), both root (UID 0) and the current process user are permitted.
// In user directories (~/Library/LaunchAgents), only the current process user is permitted.
func isAllowedOwnership(info os.FileInfo, isSystemDir bool) bool {
	if info == nil {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return true // Fallback if stat cannot be extracted
	}
	if stat.Uid == uint32(os.Getuid()) {
		return true
	}
	if isSystemDir && stat.Uid == 0 {
		return true // root:wheel in /Library/LaunchAgents is standard macOS policy
	}
	return false
}

// isOwnedByCurrentUser verifies that the filesystem item is owned by the
// UID of the current executing process (D-16). Compiles cleanly with CGO_ENABLED=0.
func isOwnedByCurrentUser(info os.FileInfo) bool {
	return isAllowedOwnership(info, false)
}

package macos

// IsWhitelisted checks whether the path or bundle identifier matches any
// immutable protection pattern.
func IsWhitelisted(path string) bool {
	return false // stub for RED test
}

// isSafeSymlink verifies that a symlink resolves to a target within cacheRoot
// and does not point to a blacklisted system root (D-16).
func isSafeSymlink(cacheRoot, linkPath string) bool {
	return true // stub for RED test
}

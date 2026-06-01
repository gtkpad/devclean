package safety

import "os"

// IsSymlink reports whether path is a symbolic link.
// Uses os.Lstat (not os.Stat) to avoid following the link.
func IsSymlink(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	return info.Mode()&os.ModeSymlink != 0, nil
}

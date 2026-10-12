package fs

import (
	"io/fs"
	"os"
	"path/filepath"
)

// ChmodRecursive traverses root recursively applying write permissions.
// It applies dirPerm to directories and filePerm to regular files,
// unlocking read-only trees (e.g. from Conan or vcpkg) prior to removal or trash operations.
// If root does not exist, it returns nil without error.
func ChmodRecursive(root string, dirPerm, filePerm os.FileMode) error {
	info, err := os.Lstat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// If root is a symlink, do not follow or modify permissions
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}

	if !info.IsDir() {
		return os.Chmod(root, filePerm)
	}

	// Unlock root directory first so WalkDir can enter and read its entries
	_ = os.Chmod(root, dirPerm)

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Try to recover write permission on the entry if there was an access error
			_ = os.Chmod(path, dirPerm)
			return nil
		}

		// Skip modifying symlinks or their targets
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		if d.IsDir() {
			return os.Chmod(path, dirPerm)
		}
		return os.Chmod(path, filePerm)
	})
}

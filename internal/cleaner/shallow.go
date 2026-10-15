package cleaner

import (
	"os"
	"path/filepath"
	"strings"
)

// HasShallowManifest checks whether any of the specified manifest patterns or directory names
// exist directly in the provided scan roots or within their immediate level-1 child directories.
// Per D-15 and D-16, it only uses os.ReadDir limited to at most 50 entries per directory
// and never performs recursive traversal.
func HasShallowManifest(paths []string, manifests ...string) bool {
	if len(paths) == 0 || len(manifests) == 0 {
		return false
	}

	for _, root := range paths {
		if root == "" {
			continue
		}
		// 1. Direct check at the scan root
		if checkDirForManifests(root, manifests...) {
			return true
		}

		// 2. Check level-1 immediate child subdirectories
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}

		count := 0
		for _, entry := range entries {
			if count >= 50 {
				break
			}
			count++

			if !entry.IsDir() {
				continue
			}

			name := entry.Name()
			if strings.HasPrefix(name, ".") {
				continue // skip hidden directories like .git
			}

			childPath := filepath.Join(root, name)
			if checkDirForManifests(childPath, manifests...) {
				return true
			}
		}
	}

	return false
}

func checkDirForManifests(dir string, manifests ...string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	count := 0
	for _, entry := range entries {
		if count >= 50 {
			break
		}
		count++

		name := entry.Name()
		for _, m := range manifests {
			if matchManifest(name, m) {
				return true
			}
		}
	}
	return false
}

func matchManifest(fileName, pattern string) bool {
	if strings.HasPrefix(pattern, "*.") {
		return strings.HasSuffix(fileName, strings.TrimPrefix(pattern, "*"))
	}
	return fileName == pattern
}

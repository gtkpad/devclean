package safety

import (
	"path/filepath"
	"strings"
)

// coreBlacklist is immutable — cannot be disabled by config or flags.
var coreBlacklist = []string{".git", ".env", ".ssh", "secrets.json"}

// systemRoots protects OS and filesystem root directories.
var systemRoots = []string{
	"/", "/bin", "/boot", "/dev", "/etc", "/lib", "/proc", "/root", "/sys", "/usr", "/var",
	"/System", "/Library", "/Applications", "/private", "/private/etc", "/private/var",
}

// CoreBlacklist returns a copy of immutable system blacklist patterns.
func CoreBlacklist() []string {
	cp := make([]string, len(coreBlacklist))
	copy(cp, coreBlacklist)
	return cp
}

// SystemRoots returns a copy of protected system root directories.
func SystemRoots() []string {
	cp := make([]string, len(systemRoots))
	copy(cp, systemRoots)
	return cp
}

// IsCoreBlacklisted checks whether the path, any of its path components, or system root matches an immutable blacklist item.
func IsCoreBlacklisted(path string) bool {
	clean := filepath.Clean(path)

	for _, root := range systemRoots {
		if clean == filepath.Clean(root) {
			return true
		}
	}

	parts := strings.FieldsFunc(clean, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	for _, part := range parts {
		for _, blocked := range coreBlacklist {
			if part == blocked {
				return true
			}
		}
	}
	return false
}

type Blacklist struct {
	patterns []string
}

func NewBlacklist(extraPatterns []string) *Blacklist {
	return &Blacklist{patterns: extraPatterns}
}

func (b *Blacklist) IsBlacklisted(path string) bool {
	if IsCoreBlacklisted(path) {
		return true
	}
	base := filepath.Base(path)
	for _, pattern := range b.patterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}
	return false
}

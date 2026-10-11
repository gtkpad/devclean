package safety

import "path/filepath"

// coreBlacklist is immutable — cannot be disabled by config or flags.
var coreBlacklist = []string{".git", ".env", ".ssh", "secrets.json"}

// CoreBlacklist returns a copy of immutable system blacklist patterns.
func CoreBlacklist() []string {
	cp := make([]string, len(coreBlacklist))
	copy(cp, coreBlacklist)
	return cp
}

// IsCoreBlacklisted checks whether the path or its base name matches an immutable blacklist item.
func IsCoreBlacklisted(path string) bool {
	clean := filepath.Clean(path)
	base := filepath.Base(clean)
	for _, blocked := range coreBlacklist {
		if clean == blocked || base == blocked {
			return true
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

package safety

import "path/filepath"

// coreBlacklist is immutable — cannot be disabled by config or flags.
var coreBlacklist = []string{".git", ".env", ".ssh", "secrets.json"}

type Blacklist struct {
	patterns []string
}

func NewBlacklist(extraPatterns []string) *Blacklist {
	return &Blacklist{patterns: extraPatterns}
}

func (b *Blacklist) IsBlacklisted(path string) bool {
	base := filepath.Base(path)
	for _, blocked := range coreBlacklist {
		if base == blocked {
			return true
		}
	}
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

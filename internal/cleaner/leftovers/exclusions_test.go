package leftovers

import (
	"path/filepath"
	"testing"
)

func TestLeftovers_Exclusions_Shield(t *testing.T) {
	tempDir := t.TempDir()

	testCases := []struct {
		name         string
		fullPath     string
		wantExcluded bool
	}{
		// Apple system components & daemons
		{"com.apple.Safari", filepath.Join(tempDir, "com.apple.Safari"), true},
		{"com.apple.finder", filepath.Join(tempDir, "com.apple.finder"), true},
		{"apple", filepath.Join(tempDir, "apple"), true},
		{"Apple Push Service", filepath.Join(tempDir, "Apple Push Service"), true},
		{"Safari", filepath.Join(tempDir, "Safari"), true},
		{"AddressBook", filepath.Join(tempDir, "AddressBook"), true},
		{"QuickLook", filepath.Join(tempDir, "QuickLook"), true},
		{"ControlCenter", filepath.Join(tempDir, "ControlCenter"), true},

		// Dedicated DevClean cleaner directories
		{"docker", filepath.Join(tempDir, "docker"), true},
		{"pnpm", filepath.Join(tempDir, "pnpm"), true},
		{"uv", filepath.Join(tempDir, "uv"), true},
		{"google", filepath.Join(tempDir, "google"), true},
		{"Code", filepath.Join(tempDir, "Code"), true},
		{"Cursor", filepath.Join(tempDir, "Cursor"), true},
		{"firefox", filepath.Join(tempDir, "firefox"), true},
		{"mise", filepath.Join(tempDir, "mise"), true},
		{"homebrew", filepath.Join(tempDir, "homebrew"), true},
		{"antigravity", filepath.Join(tempDir, "antigravity"), true},

		// Common CLI tools without .app
		{"git", filepath.Join(tempDir, "git"), true},
		{"ssh", filepath.Join(tempDir, "ssh"), true},
		{"gnupg", filepath.Join(tempDir, "gnupg"), true},
		{"gpg", filepath.Join(tempDir, "gpg"), true},
		{"cargo", filepath.Join(tempDir, "cargo"), true},
		{"rustup", filepath.Join(tempDir, "rustup"), true},
		{"zsh", filepath.Join(tempDir, "zsh"), true},
		{"brew", filepath.Join(tempDir, "brew"), true},
		{"go", filepath.Join(tempDir, "go"), true},

		// System core blacklists
		{"System", "/System", true},
		{"usr", "/usr", true},

		// macOS sensitive whitelist
		{"1Password", filepath.Join(tempDir, "1Password"), true},
		{"tailscale", filepath.Join(tempDir, "tailscale"), true},

		// Arbitrary uninstalled application leftovers (NOT excluded)
		{"OldApp", filepath.Join(tempDir, "OldApp"), false},
		{"com.corp.retiredapp", filepath.Join(tempDir, "com.corp.retiredapp"), false},
		{"RandomLeftover", filepath.Join(tempDir, "RandomLeftover"), false},
	}

	for _, tc := range testCases {
		got := isExcluded(tc.name, tc.fullPath)
		if got != tc.wantExcluded {
			t.Errorf("isExcluded(%q, %q) = %v; want %v", tc.name, tc.fullPath, got, tc.wantExcluded)
		}
	}
}

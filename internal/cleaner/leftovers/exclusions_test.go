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

		// Apple private daemons & background agents (D-01, REQ-LFT-01)
		{"PassKit", filepath.Join(tempDir, "PassKit"), true},
		{"Spotlight", filepath.Join(tempDir, "Spotlight"), true},
		{"iCloud", filepath.Join(tempDir, "iCloud"), true},
		{"icloudmailagent", filepath.Join(tempDir, "icloudmailagent"), true},
		{"familycircled", filepath.Join(tempDir, "familycircled"), true},
		{"contactsd", filepath.Join(tempDir, "contactsd"), true},
		{"homeenergyd", filepath.Join(tempDir, "homeenergyd"), true},
		{"icdd", filepath.Join(tempDir, "icdd"), true},
		{"identityservicesd", filepath.Join(tempDir, "identityservicesd"), true},
		{"IntelligenceFlow", filepath.Join(tempDir, "IntelligenceFlow"), true},
		{"locationaccessstored", filepath.Join(tempDir, "locationaccessstored"), true},
		{"networkserviceproxy", filepath.Join(tempDir, "networkserviceproxy"), true},
		{"stickersd", filepath.Join(tempDir, "stickersd"), true},
		{"tipsd", filepath.Join(tempDir, "tipsd"), true},

		// Telemetry and system logging tokens
		{"cctclearcutlogger", filepath.Join(tempDir, "cctclearcutlogger"), true},
		{"gippseudonymousid", filepath.Join(tempDir, "gippseudonymousid"), true},
		{"phtphenotype", filepath.Join(tempDir, "phtphenotype"), true},
		{"ssu", filepath.Join(tempDir, "ssu"), true},

		// OS and SDK debug symbol stores and persistent storage (D-03, REQ-LFT-01)
		{"Symbols", filepath.Join(tempDir, "Symbols"), true},
		{"RefSrcSymbols", filepath.Join(tempDir, "RefSrcSymbols"), true},
		{"SymbolSourceSymbols", filepath.Join(tempDir, "SymbolSourceSymbols"), true},
		{"SESStorage", filepath.Join(tempDir, "SESStorage"), true},
		{"IsolatedStorage", filepath.Join(tempDir, "IsolatedStorage"), true},

		// Prefix variations
		{"group.com.apple.coreservices", filepath.Join(tempDir, "group.com.apple.coreservices"), true},
		{"apple-push-service", filepath.Join(tempDir, "apple-push-service"), true},

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

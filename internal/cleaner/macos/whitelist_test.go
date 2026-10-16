package macos

import (
	"testing"
)

func TestWhitelist_ImmutablePatterns(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		// Apple Sync, Cloud & Identity
		{"CloudKit lower base", "cloudkit", true},
		{"CloudKit mixed base", "CloudKit", true},
		{"CloudKit in path", "/Users/user/Library/Caches/CloudKit", true},
		{"CloudKit bundle subpath", "/Users/user/Library/Caches/com.apple.CloudKit.metadata", true},
		{"Mobile Documents space", "mobile documents", true},
		{"Mobile Documents title", "Mobile Documents", true},
		{"MobileDocuments joined", "MobileDocuments", true},
		{"mobile.documents dotted", "mobile.documents", true},
		{"Mobile Documents full path", "/Users/test/Library/Mobile Documents/com~apple~CloudDocs", true},
		{"Keychain bundle", "com.apple.keychainaccess", true},
		{"Keychain lower", "keychain", true},
		{"Keychain title", "Keychain", true},
		{"Keychain path", "/Users/test/Library/Keychains", true},
		{"Safari bundle", "com.apple.Safari", true},
		{"Safari cache bundle", "com.apple.Safari.Cache", true},
		{"Safari simple name", "Safari", true},
		{"Safari path", "/Users/test/Library/Caches/com.apple.Safari", true},

		// Password Managers & Vaults
		{"1Password modern", "com.1password.1password", true},
		{"1Password legacy", "com.agilebits.onepassword-osx", true},
		{"Bitwarden", "com.bitwarden.desktop", true},
		{"KeePassXC", "org.keepassxc.keepassxc", true},
		{"LastPass", "com.lastpass.LastPass", true},
		{"Dashlane", "com.dashlane.Dashlane", true},

		// Mesh & Corporate VPNs
		{"Tailscale bundle", "io.tailscale.ipn.macsys", true},
		{"Tailscale lower keyword", "tailscale", true},
		{"Tailscale path", "/Users/test/Library/Caches/io.tailscale.ipn.macsys", true},
		{"WireGuard bundle", "com.wireguard.macos", true},
		{"WireGuard keyword", "wireguard", true},
		{"ZeroTier bundle", "com.zerotier.one", true},
		{"ZeroTier keyword", "zerotier", true},
		{"Cloudflare 1dot1dot1dot1", "1dot1dot1dot1", true},
		{"Cloudflare WARP bundle", "com.cloudflare.1dot1dot1dot1.macos", true},
		{"Cloudflare WARP composite", "cloudflare-warp", true},

		// Legitimate non-whitelisted caches (MUST return false)
		{"Spotify client cache", "com.spotify.client", false},
		{"Spotify path", "/Users/test/Library/Caches/com.spotify.client", false},
		{"Apple Media Analysis Daemon", "com.apple.mediaanalysisd", false},
		{"Google Chrome cache", "com.google.Chrome", false},
		{"Docker Desktop", "com.docker.docker", false},
		{"Apple Help Daemon", "com.apple.helpd", false},
		{"Apple Geod Daemon", "com.apple.geod", false},
		{"Apple Wallpaper Agent", "com.apple.WallpaperAgent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := IsWhitelisted(tt.path)
			if actual != tt.expected {
				t.Errorf("IsWhitelisted(%q) = %v, expected %v", tt.path, actual, tt.expected)
			}
		})
	}
}

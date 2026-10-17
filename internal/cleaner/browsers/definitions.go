package browsers

import (
	"path/filepath"
)

// GetBrowserDefinitions returns the canonical definitions for all 6 supported browsers.
func GetBrowserDefinitions() []BrowserDef {
	return []BrowserDef{
		{
			ID:             "chrome",
			Name:           "Google Chrome",
			Engine:         EngineChromium,
			DarwinOnly:     false,
			HasScriptCache: true,
			ProcessNames: map[string][]string{
				"darwin": {"Google Chrome", "Google Chrome Helper"},
				"linux":  {"chrome", "google-chrome", "google-chrome-stable"},
			},
			AppSupportDir: func(home, goos string) string {
				if goos == "darwin" {
					return filepath.Join(home, "Library", "Application Support", "Google", "Chrome")
				}
				return filepath.Join(home, ".config", "google-chrome")
			},
			CacheDir: func(home, goos string) string {
				if goos == "darwin" {
					return filepath.Join(home, "Library", "Caches", "Google", "Chrome")
				}
				return filepath.Join(home, ".cache", "google-chrome")
			},
		},
		{
			ID:             "brave",
			Name:           "Brave",
			Engine:         EngineChromium,
			DarwinOnly:     false,
			HasScriptCache: true,
			ProcessNames: map[string][]string{
				"darwin": {"Brave Browser", "Brave Browser Helper"},
				"linux":  {"brave", "brave-browser"},
			},
			AppSupportDir: func(home, goos string) string {
				if goos == "darwin" {
					return filepath.Join(home, "Library", "Application Support", "BraveSoftware", "Brave-Browser")
				}
				return filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser")
			},
			CacheDir: func(home, goos string) string {
				if goos == "darwin" {
					return filepath.Join(home, "Library", "Caches", "BraveSoftware", "Brave-Browser")
				}
				return filepath.Join(home, ".cache", "BraveSoftware", "Brave-Browser")
			},
		},
		{
			ID:             "arc",
			Name:           "Arc",
			Engine:         EngineChromium,
			DarwinOnly:     true,
			HasScriptCache: true,
			ProcessNames: map[string][]string{
				"darwin": {"Arc", "Arc Helper"},
			},
			AppSupportDir: func(home, goos string) string {
				if goos != "darwin" {
					return ""
				}
				return filepath.Join(home, "Library", "Application Support", "Arc", "User Data")
			},
			CacheDir: func(home, goos string) string {
				if goos != "darwin" {
					return ""
				}
				return filepath.Join(home, "Library", "Caches", "company.thebrowser.Arc")
			},
		},
		{
			ID:             "edge",
			Name:           "Microsoft Edge",
			Engine:         EngineChromium,
			DarwinOnly:     false,
			HasScriptCache: true,
			ProcessNames: map[string][]string{
				"darwin": {"Microsoft Edge", "Microsoft Edge Helper"},
				"linux":  {"msedge", "microsoft-edge"},
			},
			AppSupportDir: func(home, goos string) string {
				if goos == "darwin" {
					return filepath.Join(home, "Library", "Application Support", "Microsoft Edge")
				}
				return filepath.Join(home, ".config", "microsoft-edge")
			},
			CacheDir: func(home, goos string) string {
				if goos == "darwin" {
					return filepath.Join(home, "Library", "Caches", "Microsoft Edge")
				}
				return filepath.Join(home, ".cache", "microsoft-edge")
			},
		},
		{
			ID:             "firefox",
			Name:           "Mozilla Firefox",
			Engine:         EngineGecko,
			DarwinOnly:     false,
			HasScriptCache: false,
			ProcessNames: map[string][]string{
				"darwin": {"firefox", "firefox-bin"},
				"linux":  {"firefox", "firefox-bin"},
			},
			AppSupportDir: func(home, goos string) string {
				if goos == "darwin" {
					return filepath.Join(home, "Library", "Application Support", "Firefox", "Profiles")
				}
				return filepath.Join(home, ".mozilla", "firefox")
			},
			CacheDir: func(home, goos string) string {
				if goos == "darwin" {
					return filepath.Join(home, "Library", "Caches", "Firefox", "Profiles")
				}
				return filepath.Join(home, ".cache", "mozilla", "firefox")
			},
		},
		{
			ID:             "safari",
			Name:           "Safari",
			Engine:         EngineWebKit,
			DarwinOnly:     true,
			HasScriptCache: false,
			ProcessNames: map[string][]string{
				"darwin": {"Safari", "SafariTechnologyPreview"},
			},
			AppSupportDir: func(home, goos string) string {
				return "" // Inviolable: Strictly NO AppSupport scanning for Safari! (D-13)
			},
			CacheDir: func(home, goos string) string {
				if goos != "darwin" {
					return ""
				}
				return filepath.Join(home, "Library", "Caches", "com.apple.Safari")
			},
		},
	}
}

// FindBrowserDef searches for a BrowserDef by ID.
func FindBrowserDef(id string) (BrowserDef, bool) {
	for _, bDef := range GetBrowserDefinitions() {
		if bDef.ID == id {
			return bDef, true
		}
	}
	return BrowserDef{}, false
}

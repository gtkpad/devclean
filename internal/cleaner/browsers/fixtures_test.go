package browsers

import (
	"os"
	"path/filepath"
	"testing"
)

// setupBrowserTestFixtures constructs mock directory structures for macOS browsers in homeDir
// with double sentinels: disposable cache files, protected session sentinels, and gated ScriptCache files.
func setupBrowserTestFixtures(t *testing.T, homeDir string) (disposableFiles []string, protectedFiles []string, scriptCacheFiles []string) {
	t.Helper()

	// 1. Disposable cache files
	disposableFiles = []string{
		// Chrome
		filepath.Join(homeDir, "Library", "Application Support", "Google", "Chrome", "Default", "GPUCache", "data_0"),
		filepath.Join(homeDir, "Library", "Application Support", "Google", "Chrome", "Default", "Code Cache", "js", "entry1"),
		filepath.Join(homeDir, "Library", "Application Support", "Google", "Chrome", "Default", "Service Worker", "CacheStorage", "index"),
		filepath.Join(homeDir, "Library", "Caches", "Google", "Chrome", "Default", "Cache_Data", "f_000001"),
		filepath.Join(homeDir, "Library", "Caches", "Google", "Chrome", "Profile 1", "Cache_Data", "f_000002"),
		// Brave
		filepath.Join(homeDir, "Library", "Application Support", "BraveSoftware", "Brave-Browser", "Default", "GPUCache", "data_0"),
		filepath.Join(homeDir, "Library", "Caches", "BraveSoftware", "Brave-Browser", "Default", "Cache_Data", "f_000001"),
		// Arc
		filepath.Join(homeDir, "Library", "Application Support", "Arc", "User Data", "Default", "GPUCache", "data_0"),
		filepath.Join(homeDir, "Library", "Caches", "company.thebrowser.Arc", "Default", "Cache_Data", "f_000001"),
		// Edge
		filepath.Join(homeDir, "Library", "Application Support", "Microsoft Edge", "Default", "GPUCache", "data_0"),
		filepath.Join(homeDir, "Library", "Caches", "Microsoft Edge", "Default", "Cache_Data", "f_000001"),
		// Firefox
		filepath.Join(homeDir, "Library", "Caches", "Firefox", "Profiles", "abcd1234.default-release", "cache2", "entries", "000001"),
		filepath.Join(homeDir, "Library", "Caches", "Firefox", "Profiles", "abcd1234.default-release", "startupCache", "script.bin"),
		// Safari
		filepath.Join(homeDir, "Library", "Caches", "com.apple.Safari", "WebKitCache", "v16"),
		filepath.Join(homeDir, "Library", "Caches", "com.apple.Safari", "Cache.db"),
	}

	// 2. Protected session sentinels (MUST NEVER BE DELETED!)
	protectedFiles = []string{
		// Chrome
		filepath.Join(homeDir, "Library", "Application Support", "Google", "Chrome", "Default", "Cookies"),
		filepath.Join(homeDir, "Library", "Application Support", "Google", "Chrome", "Default", "History"),
		filepath.Join(homeDir, "Library", "Application Support", "Google", "Chrome", "Default", "Login Data"),
		filepath.Join(homeDir, "Library", "Application Support", "Google", "Chrome", "Default", "Web Data"),
		filepath.Join(homeDir, "Library", "Application Support", "Google", "Chrome", "Default", "Local Storage", "leveldb", "000003.log"),
		filepath.Join(homeDir, "Library", "Application Support", "Google", "Chrome", "Default", "IndexedDB", "site.db"),
		filepath.Join(homeDir, "Library", "Application Support", "Google", "Chrome", "Default", "Sessions", "Session_123"),
		filepath.Join(homeDir, "Library", "Application Support", "Google", "Chrome", "Default", "Bookmarks"),
		filepath.Join(homeDir, "Library", "Application Support", "Google", "Chrome", "Default", "Preferences"),
		filepath.Join(homeDir, "Library", "Application Support", "Google", "Chrome", "Profile 1", "Preferences"),
		// Firefox
		filepath.Join(homeDir, "Library", "Application Support", "Firefox", "Profiles", "abcd1234.default-release", "places.sqlite"),
		filepath.Join(homeDir, "Library", "Application Support", "Firefox", "Profiles", "abcd1234.default-release", "key4.db"),
		filepath.Join(homeDir, "Library", "Application Support", "Firefox", "Profiles", "abcd1234.default-release", "logins.json"),
		filepath.Join(homeDir, "Library", "Application Support", "Firefox", "Profiles", "abcd1234.default-release", "sessionstore.jsonlz4"),
		// Safari
		filepath.Join(homeDir, "Library", "Safari", "History.db"),
		filepath.Join(homeDir, "Library", "Safari", "Bookmarks.plist"),
		filepath.Join(homeDir, "Library", "Safari", "LastSession.plist"),
		filepath.Join(homeDir, "Library", "Safari", "CloudTabs.db"),
	}

	// 3. Gated ScriptCache files (ReviewOnly: true, EligibleBytes: 0 in v2.3)
	scriptCacheFiles = []string{
		filepath.Join(homeDir, "Library", "Application Support", "Google", "Chrome", "Default", "Service Worker", "ScriptCache", "script_0"),
		filepath.Join(homeDir, "Library", "Application Support", "BraveSoftware", "Brave-Browser", "Default", "Service Worker", "ScriptCache", "script_0"),
		filepath.Join(homeDir, "Library", "Application Support", "Arc", "User Data", "Default", "Service Worker", "ScriptCache", "script_0"),
		filepath.Join(homeDir, "Library", "Application Support", "Microsoft Edge", "Default", "Service Worker", "ScriptCache", "script_0"),
	}

	for _, f := range disposableFiles {
		if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
			t.Fatalf("failed to create dir for %s: %v", f, err)
		}
		if err := os.WriteFile(f, []byte("disposable cache content"), 0644); err != nil {
			t.Fatalf("failed to create disposable file %s: %v", f, err)
		}
	}

	for _, f := range protectedFiles {
		if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
			t.Fatalf("failed to create dir for %s: %v", f, err)
		}
		if err := os.WriteFile(f, []byte("SENSITIVE SESSION SENTINEL - DO NOT DELETE"), 0644); err != nil {
			t.Fatalf("failed to create protected file %s: %v", f, err)
		}
	}

	for _, f := range scriptCacheFiles {
		if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
			t.Fatalf("failed to create dir for %s: %v", f, err)
		}
		if err := os.WriteFile(f, []byte("scriptcache bytecode data"), 0644); err != nil {
			t.Fatalf("failed to create scriptcache file %s: %v", f, err)
		}
	}

	return disposableFiles, protectedFiles, scriptCacheFiles
}

// setupBrowserLinuxTestFixtures constructs mock directory structures for Linux browsers in homeDir.
func setupBrowserLinuxTestFixtures(t *testing.T, homeDir string) (disposableFiles []string, protectedFiles []string, scriptCacheFiles []string) {
	t.Helper()

	disposableFiles = []string{
		// Chrome Linux
		filepath.Join(homeDir, ".cache", "google-chrome", "Default", "Cache_Data", "f_000001"),
		filepath.Join(homeDir, ".config", "google-chrome", "Default", "GPUCache", "data_0"),
		filepath.Join(homeDir, ".config", "google-chrome", "Default", "Service Worker", "CacheStorage", "index"),
		// Brave Linux
		filepath.Join(homeDir, ".cache", "BraveSoftware", "Brave-Browser", "Default", "Cache_Data", "f_000001"),
		filepath.Join(homeDir, ".config", "BraveSoftware", "Brave-Browser", "Default", "GPUCache", "data_0"),
		// Edge Linux
		filepath.Join(homeDir, ".cache", "microsoft-edge", "Default", "Cache_Data", "f_000001"),
		filepath.Join(homeDir, ".config", "microsoft-edge", "Default", "GPUCache", "data_0"),
		// Firefox Linux
		filepath.Join(homeDir, ".cache", "mozilla", "firefox", "abcd1234.default-release", "cache2", "entries", "000001"),
	}

	protectedFiles = []string{
		// Chrome Linux
		filepath.Join(homeDir, ".config", "google-chrome", "Default", "Cookies"),
		filepath.Join(homeDir, ".config", "google-chrome", "Default", "History"),
		filepath.Join(homeDir, ".config", "google-chrome", "Default", "Login Data"),
		filepath.Join(homeDir, ".config", "google-chrome", "Default", "Preferences"),
		// Brave Linux
		filepath.Join(homeDir, ".config", "BraveSoftware", "Brave-Browser", "Default", "Cookies"),
		// Edge Linux
		filepath.Join(homeDir, ".config", "microsoft-edge", "Default", "Cookies"),
		// Firefox Linux
		filepath.Join(homeDir, ".mozilla", "firefox", "abcd1234.default-release", "places.sqlite"),
		filepath.Join(homeDir, ".mozilla", "firefox", "abcd1234.default-release", "key4.db"),
		filepath.Join(homeDir, ".mozilla", "firefox", "abcd1234.default-release", "logins.json"),
	}

	scriptCacheFiles = []string{
		filepath.Join(homeDir, ".config", "google-chrome", "Default", "Service Worker", "ScriptCache", "script_0"),
		filepath.Join(homeDir, ".config", "BraveSoftware", "Brave-Browser", "Default", "Service Worker", "ScriptCache", "script_0"),
		filepath.Join(homeDir, ".config", "microsoft-edge", "Default", "Service Worker", "ScriptCache", "script_0"),
	}

	for _, f := range disposableFiles {
		if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
			t.Fatalf("failed to create dir for %s: %v", f, err)
		}
		if err := os.WriteFile(f, []byte("linux disposable cache"), 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", f, err)
		}
	}

	for _, f := range protectedFiles {
		if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
			t.Fatalf("failed to create dir for %s: %v", f, err)
		}
		if err := os.WriteFile(f, []byte("linux protected session sentinel"), 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", f, err)
		}
	}

	for _, f := range scriptCacheFiles {
		if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
			t.Fatalf("failed to create dir for %s: %v", f, err)
		}
		if err := os.WriteFile(f, []byte("linux scriptcache data"), 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", f, err)
		}
	}

	return disposableFiles, protectedFiles, scriptCacheFiles
}

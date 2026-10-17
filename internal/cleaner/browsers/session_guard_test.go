package browsers

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBrowsers_InviolableSessionShield(t *testing.T) {
	// Table of paths that MUST be rejected (IsSessionOrProtectedData == true)
	protectedTestCases := []struct {
		name string
		path string
	}{
		// Session & Authentication databases
		{"Chromium Cookies", "/Users/user/Library/Application Support/Google/Chrome/Default/Cookies"},
		{"Chromium Cookies Journal", "/Users/user/Library/Application Support/Google/Chrome/Default/Cookies-journal"},
		{"Chromium Cookies WAL", "/Users/user/Library/Application Support/Google/Chrome/Default/Cookies-wal"},
		{"Chromium Lowercase cookies", "/home/user/.config/google-chrome/Default/cookies"},
		{"Chromium History", "/Users/user/Library/Application Support/Google/Chrome/Default/History"},
		{"Chromium History Journal", "/Users/user/Library/Application Support/Google/Chrome/Default/History-journal"},
		{"Chromium Login Data", "/Users/user/Library/Application Support/Google/Chrome/Default/Login Data"},
		{"Chromium Login Data Journal", "/Users/user/Library/Application Support/Google/Chrome/Default/Login Data-journal"},
		{"Chromium Web Data", "/Users/user/Library/Application Support/Google/Chrome/Default/Web Data"},
		{"Chromium Web Data Journal", "/Users/user/Library/Application Support/Google/Chrome/Default/Web Data-journal"},
		{"Chromium Local Storage Root", "/Users/user/Library/Application Support/Google/Chrome/Default/Local Storage"},
		{"Chromium Local Storage File", "/Users/user/Library/Application Support/Google/Chrome/Default/Local Storage/leveldb/000003.log"},
		{"Chromium IndexedDB Root", "/Users/user/Library/Application Support/Google/Chrome/Default/IndexedDB"},
		{"Chromium IndexedDB File", "/Users/user/Library/Application Support/Google/Chrome/Default/IndexedDB/site.indexeddb.leveldb/000001.log"},
		{"Chromium Sessions Root", "/Users/user/Library/Application Support/Google/Chrome/Default/Sessions"},
		{"Chromium Sessions File", "/Users/user/Library/Application Support/Google/Chrome/Default/Sessions/Session_1337424242"},
		{"Chromium Session Storage", "/Users/user/Library/Application Support/Google/Chrome/Default/Session Storage"},
		{"Chromium Current Session", "/Users/user/Library/Application Support/Google/Chrome/Default/Current Session"},
		{"Chromium Current Tabs", "/Users/user/Library/Application Support/Google/Chrome/Default/Current Tabs"},
		{"Chromium Last Session", "/Users/user/Library/Application Support/Google/Chrome/Default/Last Session"},
		{"Chromium Last Tabs", "/Users/user/Library/Application Support/Google/Chrome/Default/Last Tabs"},
		{"Chromium Bookmarks", "/Users/user/Library/Application Support/Google/Chrome/Default/Bookmarks"},
		{"Chromium Preferences", "/Users/user/Library/Application Support/Google/Chrome/Default/Preferences"},
		{"Chromium Secure Preferences", "/Users/user/Library/Application Support/Google/Chrome/Default/Secure Preferences"},
		{"Chromium Favicons", "/Users/user/Library/Application Support/Google/Chrome/Default/Favicons"},
		{"Chromium Form History", "/Users/user/Library/Application Support/Google/Chrome/Default/Form History"},

		// Firefox SQLite & JSON databases
		{"Firefox Places SQLite", "/Users/user/Library/Application Support/Firefox/Profiles/x.default/places.sqlite"},
		{"Firefox Places WAL", "/Users/user/Library/Application Support/Firefox/Profiles/x.default/places.sqlite-wal"},
		{"Firefox Key4 DB", "/Users/user/Library/Application Support/Firefox/Profiles/x.default/key4.db"},
		{"Firefox Logins JSON", "/Users/user/Library/Application Support/Firefox/Profiles/x.default/logins.json"},
		{"Firefox Sessionstore", "/Users/user/Library/Application Support/Firefox/Profiles/x.default/sessionstore.jsonlz4"},
		{"Firefox Sessionstore Backups", "/Users/user/Library/Application Support/Firefox/Profiles/x.default/sessionstore-backups/recovery.jsonlz4"},
		{"Firefox Formhistory", "/Users/user/Library/Application Support/Firefox/Profiles/x.default/formhistory.sqlite"},
		{"Firefox Favicons", "/Users/user/Library/Application Support/Firefox/Profiles/x.default/favicons.sqlite"},

		// Safari Private State
		{"Safari Root", "/Users/user/Library/Safari"},
		{"Safari History", "/Users/user/Library/Safari/History.db"},
		{"Safari Bookmarks", "/Users/user/Library/Safari/Bookmarks.plist"},
		{"Safari LastSession", "/Users/user/Library/Safari/LastSession.plist"},
		{"Safari CloudTabs", "/Users/user/Library/Safari/CloudTabs.db"},
		{"Safari Subdir", "/Users/user/Library/Safari/Touch Icons Cache"},

		// System Blacklisted Paths (via safety.IsCoreBlacklisted)
		{"Root Directory", "/"},
		{"System Bin", "/bin"},
		{"System Lib", "/usr/lib"},
		{"System Etc", "/etc"},
		{"Hidden SSH", "/Users/user/.ssh/id_rsa"},
		{"Hidden Git", "/Users/user/project/.git/HEAD"},
		{"Secrets File", "/Users/user/secrets.json"},
		{"Environment File", "/Users/user/app/.env"},
	}

	for _, tc := range protectedTestCases {
		t.Run(tc.name, func(t *testing.T) {
			if !IsSessionOrProtectedData(tc.path) {
				t.Errorf("VIOLATION: expected IsSessionOrProtectedData(%q) = true, got false", tc.path)
			}
			if err := VerifyCleanCandidate(tc.path); err == nil {
				t.Errorf("VIOLATION: expected VerifyCleanCandidate(%q) to return error, got nil", tc.path)
			}
		})
	}

	// Table of paths that ARE safe regenerable cache leaves (IsSessionOrProtectedData == false)
	safeTestCases := []struct {
		name string
		path string
	}{
		{"Chrome GPUCache", "/Users/user/Library/Application Support/Google/Chrome/Default/GPUCache/data_0"},
		{"Chrome DawnCache", "/Users/user/Library/Application Support/Google/Chrome/Default/DawnCache/gpu_data"},
		{"Chrome Code Cache JS", "/Users/user/Library/Application Support/Google/Chrome/Default/Code Cache/js/entry1"},
		{"Chrome CacheStorage Index", "/Users/user/Library/Application Support/Google/Chrome/Default/Service Worker/CacheStorage/index"},
		{"Chrome Cache_Data Leaf", "/Users/user/Library/Caches/Google/Chrome/Default/Cache_Data/f_000001"},
		{"Brave GPUCache", "/Users/user/Library/Application Support/BraveSoftware/Brave-Browser/Default/GPUCache/data_0"},
		{"Edge Cache_Data", "/Users/user/Library/Caches/Microsoft Edge/Default/Cache_Data/f_000001"},
		{"Firefox Cache2 Entry", "/Users/user/Library/Caches/Firefox/Profiles/abcd1234.default-release/cache2/entries/000001"},
		{"Firefox StartupCache", "/Users/user/Library/Caches/Firefox/Profiles/abcd1234.default-release/startupCache/script.bin"},
		{"Safari WebKitCache", "/Users/user/Library/Caches/com.apple.Safari/WebKitCache/v16"},
		{"Safari Cache.db", "/Users/user/Library/Caches/com.apple.Safari/Cache.db"},
		{"Safari Cache.db-wal", "/Users/user/Library/Caches/com.apple.Safari/Cache.db-wal"},
	}

	for _, tc := range safeTestCases {
		t.Run(tc.name, func(t *testing.T) {
			if IsSessionOrProtectedData(tc.path) {
				t.Errorf("FALSE POSITIVE: expected IsSessionOrProtectedData(%q) = false, got true", tc.path)
			}
			if err := VerifyCleanCandidate(tc.path); err != nil {
				t.Errorf("FALSE POSITIVE: expected VerifyCleanCandidate(%q) = nil, got %v", tc.path)
			}
		})
	}
}

func TestBrowsers_SafariIsolation(t *testing.T) {
	// Inviolable rule (D-13): ~/Library/Safari is NEVER touched, scanned, or deleted.
	// Only ~/Library/Caches/com.apple.Safari is allowed.

	forbiddenSafariPaths := []string{
		"/Users/user/Library/Safari",
		"/Users/user/Library/Safari/",
		"/Users/user/Library/Safari/History.db",
		"/Users/user/Library/Safari/Bookmarks.plist",
		"/Users/user/Library/Safari/LastSession.plist",
		"/Users/user/Library/Safari/CloudTabs.db",
		"/Users/user/Library/Safari/Touch Icons Cache",
		"/Users/user/Library/Safari/LocalStorage",
		"/Users/user/Library/Safari/WebFeedSources.plist",
		"/Users/user/Library/Safari/SearchDescriptions.plist",
		"/Users/user/Library/Safari/PerSitePreferences.db",
		"/Users/user/Library/Safari/ReadingListArchives",
	}

	for _, p := range forbiddenSafariPaths {
		if !IsSessionOrProtectedData(p) {
			t.Errorf("INVIOLABLE BREACH: Safari private path %q was not detected as protected session data!", p)
		}
		if err := VerifyCleanCandidate(p); err == nil {
			t.Errorf("INVIOLABLE BREACH: VerifyCleanCandidate(%q) returned nil for Safari private directory!", p)
		}
	}

	// Allowed Safari cache paths
	allowedSafariPaths := []string{
		"/Users/user/Library/Caches/com.apple.Safari/WebKitCache/v16",
		"/Users/user/Library/Caches/com.apple.Safari/Cache.db",
		"/Users/user/Library/Caches/com.apple.Safari/Cache.db-wal",
		"/Users/user/Library/Caches/com.apple.Safari/Cache.db-shm",
		"/Users/user/Library/Caches/com.apple.Safari/fsCachedData",
	}

	for _, p := range allowedSafariPaths {
		if IsSessionOrProtectedData(p) {
			t.Errorf("FALSE POSITIVE: Allowed Safari cache path %q was marked as protected session data!", p)
		}
		if err := VerifyCleanCandidate(p); err != nil {
			t.Errorf("FALSE POSITIVE: VerifyCleanCandidate(%q) rejected valid Safari cache: %v", p, err)
		}
	}
}

func TestBrowsers_Definitions(t *testing.T) {
	defs := GetBrowserDefinitions()
	if len(defs) != 6 {
		t.Fatalf("expected 6 browser definitions, got %d", len(defs))
	}

	expectedIDs := map[string]bool{
		"chrome":  true,
		"brave":   true,
		"arc":     true,
		"edge":    true,
		"firefox": true,
		"safari":  true,
	}

	for _, def := range defs {
		if !expectedIDs[def.ID] {
			t.Errorf("unexpected browser ID: %s", def.ID)
		}
		foundDef, ok := FindBrowserDef(def.ID)
		if !ok {
			t.Errorf("FindBrowserDef(%q) failed to find registered browser", def.ID)
		}
		if foundDef.Name != def.Name {
			t.Errorf("FindBrowserDef(%q) name mismatch: expected %q, got %q", def.ID, def.Name, foundDef.Name)
		}

		// Platform checks
		if def.ID == "arc" || def.ID == "safari" {
			if !def.DarwinOnly {
				t.Errorf("expected %s to have DarwinOnly=true", def.ID)
			}
		} else {
			if def.DarwinOnly {
				t.Errorf("expected %s to have DarwinOnly=false", def.ID)
			}
		}

		// ScriptCache checks (Chromium only)
		if def.Engine == EngineChromium {
			if !def.HasScriptCache {
				t.Errorf("expected Chromium browser %s to have HasScriptCache=true", def.ID)
			}
		} else {
			if def.HasScriptCache {
				t.Errorf("expected non-Chromium browser %s to have HasScriptCache=false", def.ID)
			}
		}

		// Safari inviolable AppSupport rule
		if def.ID == "safari" {
			if def.AppSupportDir != nil && def.AppSupportDir("/Users/test", "darwin") != "" {
				t.Errorf("INVIOLABLE BREACH: Safari AppSupportDir returned non-empty string %q", def.AppSupportDir("/Users/test", "darwin"))
			}
		}
	}

	if _, ok := FindBrowserDef("nonexistent"); ok {
		t.Errorf("expected FindBrowserDef('nonexistent') to return false")
	}
}

func TestBrowsers_FixturesHelper(t *testing.T) {
	tmpDir := t.TempDir()
	disposable, protected, scriptCache := setupBrowserTestFixtures(t, tmpDir)

	if len(disposable) == 0 {
		t.Errorf("setupBrowserTestFixtures created 0 disposable files")
	}
	if len(protected) == 0 {
		t.Errorf("setupBrowserTestFixtures created 0 protected files")
	}
	if len(scriptCache) == 0 {
		t.Errorf("setupBrowserTestFixtures created 0 scriptcache files")
	}

	// Verify all protected files are indeed rejected by the session guard
	for _, f := range protected {
		if !IsSessionOrProtectedData(f) {
			t.Errorf("FIXTURE SANITY FAILURE: protected fixture %q was not blocked by IsSessionOrProtectedData", f)
		}
	}

	// Verify all disposable files are NOT blocked by the session guard
	for _, f := range disposable {
		if IsSessionOrProtectedData(f) {
			t.Errorf("FIXTURE SANITY FAILURE: disposable fixture %q was blocked by IsSessionOrProtectedData", f)
		}
	}

	// Linux fixtures sanity
	tmpLinuxDir := t.TempDir()
	linuxDisposable, linuxProtected, linuxScriptCache := setupBrowserLinuxTestFixtures(t, tmpLinuxDir)
	if len(linuxDisposable) == 0 || len(linuxProtected) == 0 || len(linuxScriptCache) == 0 {
		t.Errorf("setupBrowserLinuxTestFixtures failed to create files")
	}
	for _, f := range linuxProtected {
		if !IsSessionOrProtectedData(f) {
			t.Errorf("LINUX FIXTURE FAILURE: protected %q not blocked", f)
		}
	}
	for _, f := range linuxDisposable {
		if IsSessionOrProtectedData(f) {
			t.Errorf("LINUX FIXTURE FAILURE: disposable %q blocked", f)
		}
	}
}

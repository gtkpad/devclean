package browsers_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/cleaner/browsers"
	"github.com/gtkpad/devclean/internal/domain"
)

func mockStoppedExecutor() *cleaner.MockCommandExecutor {
	mock := cleaner.NewMockExecutor()
	for _, bDef := range browsers.GetBrowserDefinitions() {
		for _, names := range bDef.ProcessNames {
			for _, name := range names {
				mock.Errors["pgrep -x "+name] = &cleaner.MockExitError{Code: 1}
			}
		}
	}
	return mock
}

func TestBrowsers_Lifecycle(t *testing.T) {
	ctx := context.Background()

	t.Run("NameAndInstallation", func(t *testing.T) {
		c := browsers.New()
		if c.Name() != "browsers" {
			t.Errorf("Name() = %q, want 'browsers'", c.Name())
		}

		emptyDir := t.TempDir()
		emptyCleaner := browsers.NewWithHome(emptyDir)
		if emptyCleaner.IsInstalled() {
			t.Errorf("IsInstalled() on empty dir = true, want false")
		}

		popDir := t.TempDir()
		// Create minimal Chrome cache directory
		var appDir string
		if runtime.GOOS == "darwin" {
			appDir = filepath.Join(popDir, "Library", "Application Support", "Google", "Chrome")
		} else {
			appDir = filepath.Join(popDir, ".config", "google-chrome")
		}
		if err := os.MkdirAll(appDir, 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
		popCleaner := browsers.NewWithHome(popDir)
		if !popCleaner.IsInstalled() {
			t.Errorf("IsInstalled() on populated dir = false, want true")
		}
	})

	t.Run("ScanDisabled", func(t *testing.T) {
		tempDir := t.TempDir()
		c := browsers.NewWithHome(tempDir)
		targets, err := c.Scan(ctx, domain.EcosystemRules{Enabled: false})
		if err != nil {
			t.Fatalf("Scan() error = %v, want nil", err)
		}
		if len(targets) != 0 {
			t.Errorf("Scan() targets count = %d, want 0 when disabled", len(targets))
		}
	})

	t.Run("ScanFullDispatch", func(t *testing.T) {
		tempDir := t.TempDir()
		mockExec := mockStoppedExecutor()

		// Create mock directories with cache files
		chromeCache := filepath.Join(tempDir, "Library", "Application Support", "Google", "Chrome", "Default", "GPUCache")
		if runtime.GOOS != "darwin" {
			chromeCache = filepath.Join(tempDir, ".config", "google-chrome", "Default", "GPUCache")
		}
		if err := os.MkdirAll(chromeCache, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(chromeCache, "data_0"), []byte("payload"), 0644); err != nil {
			t.Fatal(err)
		}

		c := browsers.NewWithExecutorAndHome(mockExec, tempDir)
		targets, err := c.Scan(ctx, domain.EcosystemRules{Enabled: true, MaxAgeDays: 0})
		if err != nil {
			t.Fatalf("Scan() failed: %v", err)
		}
		if len(targets) == 0 {
			t.Fatalf("expected at least 1 target, got 0")
		}

		foundChrome := false
		for _, trg := range targets {
			if trg.Category != "browser-cache" && trg.Category != "browser-scriptcache" {
				t.Errorf("unexpected category: %s", trg.Category)
			}
			if trg.Name == "Google Chrome Caches" {
				foundChrome = true
				if trg.ReviewOnly {
					t.Errorf("expected Chrome Caches to be eligible when stopped, got ReviewOnly")
				}
			}
		}
		if !foundChrome {
			t.Errorf("Google Chrome Caches target not found in scan results")
		}
	})
}

func TestBrowsers_DefensiveRefusal(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	mockExec := mockStoppedExecutor()

	scDir := filepath.Join(tempDir, "Library", "Application Support", "Google", "Chrome", "Default", "Service Worker", "ScriptCache")
	if runtime.GOOS != "darwin" {
		scDir = filepath.Join(tempDir, ".config", "google-chrome", "Default", "Service Worker", "ScriptCache")
	}
	if err := os.MkdirAll(scDir, 0755); err != nil {
		t.Fatal(err)
	}
	scFile := filepath.Join(scDir, "script_0")
	if err := os.WriteFile(scFile, []byte("bytecode"), 0644); err != nil {
		t.Fatal(err)
	}

	c := browsers.NewWithExecutorAndHome(mockExec, tempDir)

	// Target 1: Category browser-scriptcache
	t1 := domain.Target{
		ID:           "test-sc-1",
		Name:         "Chrome ScriptCache",
		Path:         "browser://chrome/scriptcache",
		Category:     "browser-scriptcache",
		ReviewOnly:   true,
		ReviewReason: "scriptcache_requires_evidence",
	}

	// Target 2: ReviewOnly = true
	t2 := domain.Target{
		ID:           "test-sc-2",
		Name:         "ReviewOnly Target",
		Path:         "browser://chrome/caches",
		Category:     "browser-cache",
		ReviewOnly:   true,
		ReviewReason: domain.ReviewReasonActiveProcess,
	}

	err := c.Clean(ctx, []domain.Target{t1, t2})
	if err != nil {
		t.Fatalf("Clean() returned error on refused targets: %v", err)
	}

	// Assert scFile was NOT deleted
	if _, err := os.Stat(scFile); err != nil {
		t.Errorf("scriptcache file should have been protected by defensive refusal, got err: %v", err)
	}
}

func TestBrowsers_PreCleanRevalidationRace(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	mockExec := mockStoppedExecutor()

	cacheDir := filepath.Join(tempDir, "Library", "Application Support", "Google", "Chrome", "Default", "GPUCache")
	if runtime.GOOS != "darwin" {
		cacheDir = filepath.Join(tempDir, ".config", "google-chrome", "Default", "GPUCache")
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	cacheFile := filepath.Join(cacheDir, "data_0")
	if err := os.WriteFile(cacheFile, []byte("disposable data"), 0644); err != nil {
		t.Fatal(err)
	}

	c := browsers.NewWithExecutorAndHome(mockExec, tempDir)

	// Scan when stopped: target is eligible
	targets, err := c.Scan(ctx, domain.EcosystemRules{Enabled: true, MaxAgeDays: 0})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(targets) == 0 {
		t.Fatalf("expected targets from Scan")
	}

	// User opens Chrome between Scan and Clean!
	delete(mockExec.Errors, "pgrep -x Google Chrome")
	delete(mockExec.Errors, "pgrep -x chrome")
	mockExec.Outputs["pgrep -x Google Chrome"] = []byte("99999\n")
	mockExec.Outputs["pgrep -x chrome"] = []byte("99999\n")

	// Clean is called on previously eligible target
	err = c.Clean(ctx, targets)
	if err != nil {
		t.Fatalf("Clean returned error: %v", err)
	}

	// Cache file MUST survive because pre-clean process revalidation aborted fail-closed!
	if _, err := os.Stat(cacheFile); err != nil {
		t.Errorf("cacheFile should have been preserved due to pre-clean race abort, got err: %v", err)
	}
}

func TestBrowsers_InviolableSessionShield_Integration(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	mockExec := mockStoppedExecutor()

	// Create test fixtures with disposable cache, protected session, and gated scriptcache
	disposableFiles := []string{
		filepath.Join(tempDir, "Library", "Application Support", "Google", "Chrome", "Default", "GPUCache", "data_0"),
		filepath.Join(tempDir, "Library", "Caches", "Google", "Chrome", "Default", "Cache_Data", "f_000001"),
		filepath.Join(tempDir, "Library", "Caches", "Firefox", "Profiles", "test.default-release", "cache2", "entries", "001"),
	}
	protectedFiles := []string{
		filepath.Join(tempDir, "Library", "Application Support", "Google", "Chrome", "Default", "Cookies"),
		filepath.Join(tempDir, "Library", "Application Support", "Google", "Chrome", "Default", "History"),
		filepath.Join(tempDir, "Library", "Application Support", "Google", "Chrome", "Default", "Login Data"),
		filepath.Join(tempDir, "Library", "Application Support", "Firefox", "Profiles", "test.default-release", "places.sqlite"),
		filepath.Join(tempDir, "Library", "Application Support", "Firefox", "Profiles", "test.default-release", "key4.db"),
		filepath.Join(tempDir, "Library", "Safari", "History.db"),
		filepath.Join(tempDir, "Library", "Safari", "LastSession.plist"),
	}
	scriptCacheFiles := []string{
		filepath.Join(tempDir, "Library", "Application Support", "Google", "Chrome", "Default", "Service Worker", "ScriptCache", "script_0"),
	}

	if runtime.GOOS != "darwin" {
		disposableFiles = []string{
			filepath.Join(tempDir, ".config", "google-chrome", "Default", "GPUCache", "data_0"),
			filepath.Join(tempDir, ".cache", "google-chrome", "Default", "Cache_Data", "f_000001"),
			filepath.Join(tempDir, ".cache", "mozilla", "firefox", "test.default-release", "cache2", "entries", "001"),
		}
		protectedFiles = []string{
			filepath.Join(tempDir, ".config", "google-chrome", "Default", "Cookies"),
			filepath.Join(tempDir, ".config", "google-chrome", "Default", "History"),
			filepath.Join(tempDir, ".config", "google-chrome", "Default", "Login Data"),
			filepath.Join(tempDir, ".mozilla", "firefox", "test.default-release", "places.sqlite"),
			filepath.Join(tempDir, ".mozilla", "firefox", "test.default-release", "key4.db"),
		}
		scriptCacheFiles = []string{
			filepath.Join(tempDir, ".config", "google-chrome", "Default", "Service Worker", "ScriptCache", "script_0"),
		}
	}

	for _, f := range disposableFiles {
		if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("disposable"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range protectedFiles {
		if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("protected session data"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range scriptCacheFiles {
		if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("scriptcache data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	c := browsers.NewWithExecutorAndHome(mockExec, tempDir)

	targets, err := c.Scan(ctx, domain.EcosystemRules{Enabled: true, MaxAgeDays: 0})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Clean all returned targets
	err = c.Clean(ctx, targets)
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// 1. Assert disposable files are DELETED
	for _, f := range disposableFiles {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("disposable file should have been deleted: %s", f)
		}
	}

	// 2. Assert protected session files are 100% PRESERVED
	for _, f := range protectedFiles {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("protected session file was destroyed: %s (err: %v)", f, err)
		}
	}

	// 3. Assert ScriptCache files are 100% PRESERVED
	for _, f := range scriptCacheFiles {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("scriptcache file was destroyed: %s (err: %v)", f, err)
		}
	}
}

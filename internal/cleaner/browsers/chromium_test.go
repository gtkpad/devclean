package browsers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)

func TestBrowsers_OnDiskDetection(t *testing.T) {
	ctx := context.Background()

	t.Run("Darwin_ChromiumBrowsersDetected", func(t *testing.T) {
		tempDir := t.TempDir()
		setupBrowserTestFixtures(t, tempDir)

		defs := []string{"chrome", "brave", "arc", "edge"}
		for _, id := range defs {
			bDef, ok := FindBrowserDef(id)
			if !ok {
				t.Fatalf("browser definition not found: %s", id)
			}
			targets, err := scanChromiumBrowser(ctx, tempDir, "darwin", bDef, false, time.Time{}, 0)
			if err != nil {
				t.Fatalf("scanChromiumBrowser failed for %s: %v", id, err)
			}
			if len(targets) == 0 {
				t.Errorf("expected targets for browser %s on darwin, got 0", id)
			}
		}
	})

	t.Run("Linux_ArcSkipped_OthersDetected", func(t *testing.T) {
		tempDir := t.TempDir()
		setupBrowserLinuxTestFixtures(t, tempDir)

		// Arc is Darwin-only
		arcDef, ok := FindBrowserDef("arc")
		if !ok {
			t.Fatal("arc definition not found")
		}
		if !arcDef.DarwinOnly {
			t.Error("expected arc to be DarwinOnly")
		}
		arcTargets, err := scanChromiumBrowser(ctx, tempDir, "linux", arcDef, false, time.Time{}, 0)
		if err != nil {
			t.Fatalf("scanChromiumBrowser failed for arc on linux: %v", err)
		}
		if len(arcTargets) != 0 {
			t.Errorf("expected 0 targets for arc on linux, got %d", len(arcTargets))
		}

		// Chrome, Brave, Edge detected on Linux
		linuxChromium := []string{"chrome", "brave", "edge"}
		for _, id := range linuxChromium {
			bDef, ok := FindBrowserDef(id)
			if !ok {
				t.Fatalf("browser definition not found: %s", id)
			}
			targets, err := scanChromiumBrowser(ctx, tempDir, "linux", bDef, false, time.Time{}, 0)
			if err != nil {
				t.Fatalf("scanChromiumBrowser failed for %s on linux: %v", id, err)
			}
			if len(targets) == 0 {
				t.Errorf("expected targets for %s on linux, got 0", id)
			}
		}
	})
}

func TestBrowsers_MultiProfileTraversal(t *testing.T) {
	tempDir := t.TempDir()
	appSupportRoot := filepath.Join(tempDir, "Library", "Application Support", "Google", "Chrome")
	cacheRoot := filepath.Join(tempDir, "Library", "Caches", "Google", "Chrome")

	// 1. Profile discovery test
	profileNames := []string{"Default", "Profile 1", "Profile 2", "System Profile", "Guest Profile", "CustomProfile"}
	for _, name := range profileNames {
		pDir := filepath.Join(appSupportRoot, name)
		if err := os.MkdirAll(pDir, 0755); err != nil {
			t.Fatal(err)
		}
		if name == "CustomProfile" {
			// Recognized by Preferences file
			if err := os.WriteFile(filepath.Join(pDir, "Preferences"), []byte("{}"), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}

	discovered := discoverChromiumProfiles(appSupportRoot, cacheRoot)
	if len(discovered) != len(profileNames) {
		t.Fatalf("expected %d profiles discovered, got %d", len(profileNames), len(discovered))
	}

	foundMap := make(map[string]bool)
	for _, p := range discovered {
		foundMap[p.Name] = true
	}
	for _, name := range profileNames {
		if !foundMap[name] {
			t.Errorf("profile %q was not discovered", name)
		}
	}

	// 2. Scan multi-profile cache consolidation
	// Write allowlisted cache files across multiple profiles
	cachePayloads := []struct {
		path string
		size int64
	}{
		{filepath.Join(appSupportRoot, "Default", "GPUCache", "data_0"), 1000},
		{filepath.Join(appSupportRoot, "Profile 1", "DawnCache", "data_0"), 2000},
		{filepath.Join(appSupportRoot, "Profile 2", "Code Cache", "js", "entry1"), 3000},
		{filepath.Join(appSupportRoot, "System Profile", "GPUCache", "data_0"), 4000},
		{filepath.Join(cacheRoot, "Default", "Cache_Data", "f_000001"), 5000},
		{filepath.Join(cacheRoot, "Profile 1", "Media Cache", "f_000002"), 6000},
	}

	var expectedTotal int64
	for _, cp := range cachePayloads {
		expectedTotal += cp.size
		if err := os.MkdirAll(filepath.Dir(cp.path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(cp.path, make([]byte, cp.size), 0644); err != nil {
			t.Fatal(err)
		}
	}

	chromeDef, _ := FindBrowserDef("chrome")
	targets, err := scanChromiumBrowser(context.Background(), tempDir, "darwin", chromeDef, false, time.Time{}, 0)
	if err != nil {
		t.Fatalf("scanChromiumBrowser error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected exactly 1 consolidated target, got %d", len(targets))
	}

	mainTarget := targets[0]
	if mainTarget.Path != "browser://chrome/caches" {
		t.Errorf("expected virtual path browser://chrome/caches, got %s", mainTarget.Path)
	}
	if mainTarget.SizeBytes != expectedTotal {
		t.Errorf("expected SizeBytes %d, got %d", expectedTotal, mainTarget.SizeBytes)
	}
	if mainTarget.EligibleBytes != expectedTotal {
		t.Errorf("expected EligibleBytes %d, got %d", expectedTotal, mainTarget.EligibleBytes)
	}
	if !mainTarget.TTLEligible {
		t.Error("expected TTLEligible true when maxAgeDays is 0")
	}
	if mainTarget.ReviewOnly {
		t.Error("expected ReviewOnly false when browser is not running")
	}
}

func TestBrowsers_Chromium_ScriptCacheGated(t *testing.T) {
	tempDir := t.TempDir()
	appSupportRoot := filepath.Join(tempDir, "Library", "Application Support", "Google", "Chrome")

	// Disposable cache leaf
	cacheFile := filepath.Join(appSupportRoot, "Default", "GPUCache", "data_0")
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cacheFile, make([]byte, 1024), 0644); err != nil {
		t.Fatal(err)
	}

	// ScriptCache leaf in Default profile
	sc1 := filepath.Join(appSupportRoot, "Default", "Service Worker", "ScriptCache", "script_0")
	if err := os.MkdirAll(filepath.Dir(sc1), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sc1, make([]byte, 2048), 0644); err != nil {
		t.Fatal(err)
	}

	// ScriptCache leaf in Profile 1
	sc2 := filepath.Join(appSupportRoot, "Profile 1", "Service Worker", "ScriptCache", "script_1")
	if err := os.MkdirAll(filepath.Dir(sc2), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sc2, make([]byte, 4096), 0644); err != nil {
		t.Fatal(err)
	}

	chromeDef, _ := FindBrowserDef("chrome")
	targets, err := scanChromiumBrowser(context.Background(), tempDir, "darwin", chromeDef, false, time.Time{}, 0)
	if err != nil {
		t.Fatalf("scanChromiumBrowser error: %v", err)
	}

	if len(targets) != 2 {
		t.Fatalf("expected 2 targets (caches + scriptcache), got %d", len(targets))
	}

	var cacheTarget, scTarget *domain.Target
	for i := range targets {
		if targets[i].Path == "browser://chrome/caches" {
			cacheTarget = &targets[i]
		}
		if targets[i].Path == "browser://chrome/scriptcache" {
			scTarget = &targets[i]
		}
	}

	if cacheTarget == nil {
		t.Fatal("missing main cache target browser://chrome/caches")
	}
	if cacheTarget.SizeBytes != 1024 {
		t.Errorf("expected cache size 1024, got %d", cacheTarget.SizeBytes)
	}
	if cacheTarget.EligibleBytes != 1024 {
		t.Errorf("expected eligible bytes 1024, got %d", cacheTarget.EligibleBytes)
	}
	if cacheTarget.ReviewOnly {
		t.Error("expected cacheTarget ReviewOnly to be false")
	}

	if scTarget == nil {
		t.Fatal("missing scriptcache target browser://chrome/scriptcache")
	}
	if scTarget.SizeBytes != 6144 {
		t.Errorf("expected scriptcache size 6144 (2048+4096), got %d", scTarget.SizeBytes)
	}
	// INVIOLABLE: Gating requirements for ScriptCache in v2.3 (BROW-04, D-05, D-06)
	if scTarget.EligibleBytes != 0 {
		t.Errorf("INVIOLABLE VIOLATION: ScriptCache EligibleBytes must be 0, got %d", scTarget.EligibleBytes)
	}
	if scTarget.TTLEligible {
		t.Error("INVIOLABLE VIOLATION: ScriptCache TTLEligible must be false")
	}
	if !scTarget.ReviewOnly {
		t.Error("INVIOLABLE VIOLATION: ScriptCache ReviewOnly must be true")
	}
	if scTarget.ReviewReason != "scriptcache_requires_evidence" {
		t.Errorf("expected ReviewReason 'scriptcache_requires_evidence', got %q", scTarget.ReviewReason)
	}
	if scTarget.Category != "browser-scriptcache" {
		t.Errorf("expected Category 'browser-scriptcache', got %q", scTarget.Category)
	}
	if scTarget.Risk != domain.RiskMedium {
		t.Errorf("expected RiskMedium, got %s", scTarget.Risk)
	}
}

func TestBrowsers_Chromium_ProcessBlocked(t *testing.T) {
	tempDir := t.TempDir()
	appSupportRoot := filepath.Join(tempDir, "Library", "Application Support", "Google", "Chrome")

	cacheFile := filepath.Join(appSupportRoot, "Default", "GPUCache", "data_0")
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cacheFile, make([]byte, 1024), 0644); err != nil {
		t.Fatal(err)
	}

	chromeDef, _ := FindBrowserDef("chrome")
	targets, err := scanChromiumBrowser(context.Background(), tempDir, "darwin", chromeDef, true, time.Time{}, 0)
	if err != nil {
		t.Fatalf("scanChromiumBrowser error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}

	trg := targets[0]
	if !trg.ReviewOnly {
		t.Error("expected ReviewOnly true when process is blocked")
	}
	if trg.ReviewReason != domain.ReviewReasonActiveProcess {
		t.Errorf("expected ReviewReasonActiveProcess, got %s", trg.ReviewReason)
	}
	if trg.EligibleBytes != 0 {
		t.Errorf("expected EligibleBytes 0 when process blocked, got %d", trg.EligibleBytes)
	}
	if trg.TTLEligible {
		t.Error("expected TTLEligible false when process blocked")
	}
}

func TestBrowsers_Chromium_Cleaning(t *testing.T) {
	tempDir := t.TempDir()
	disposable, protected, scriptCache := setupBrowserTestFixtures(t, tempDir)

	chromiumIDs := []string{"chrome", "brave", "arc", "edge"}
	for _, id := range chromiumIDs {
		bDef, ok := FindBrowserDef(id)
		if !ok {
			t.Fatalf("browser %s not found", id)
		}
		if err := cleanChromiumBrowser(tempDir, "darwin", bDef); err != nil {
			t.Fatalf("cleanChromiumBrowser failed for %s: %v", id, err)
		}
	}

	// 1. Verify Chromium disposable files are deleted
	for _, f := range disposable {
		// Only check Chromium files in this test
		if strings.Contains(f, "Safari") || strings.Contains(f, "Firefox") {
			continue
		}
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("expected disposable file to be deleted, but it exists: %s", f)
		}
	}

	// 2. Verify all protected session sentinels are 100% INTACT
	for _, f := range protected {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("INVIOLABLE VIOLATION: protected session file was corrupted or removed: %s (%v)", f, err)
		}
	}

	// 3. Verify all ScriptCache files are 100% UNTOUCHED
	for _, f := range scriptCache {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("INVIOLABLE VIOLATION: ScriptCache file was corrupted or removed during clean: %s (%v)", f, err)
		}
	}
}

func TestBrowsers_Chromium_SymlinkDefense(t *testing.T) {
	tempDir := t.TempDir()
	appSupportRoot := filepath.Join(tempDir, "Library", "Application Support", "Google", "Chrome", "Default")
	outsideDir := filepath.Join(tempDir, "outside_target")

	if err := os.MkdirAll(appSupportRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outsideDir, 0755); err != nil {
		t.Fatal(err)
	}

	secretFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(secretFile, []byte("vital information"), 0644); err != nil {
		t.Fatal(err)
	}

	// Symlink GPUCache to outsideDir
	symlinkPath := filepath.Join(appSupportRoot, "GPUCache")
	if err := os.Symlink(outsideDir, symlinkPath); err != nil {
		t.Skipf("symlink creation not supported: %v", err)
	}

	chromeDef, _ := FindBrowserDef("chrome")
	if err := cleanChromiumBrowser(tempDir, "darwin", chromeDef); err != nil {
		t.Fatalf("cleanChromiumBrowser error: %v", err)
	}

	// Verify the symlink was NOT followed or deleted
	if _, err := os.Stat(secretFile); err != nil {
		t.Errorf("symlink target file was removed! %v", err)
	}
}

func TestBrowsers_Chromium_HashBrowserPath(t *testing.T) {
	h1 := hashBrowserPath("browser://chrome/caches")
	h2 := hashBrowserPath("browser://chrome/caches")
	h3 := hashBrowserPath("browser://chrome/scriptcache")

	if h1 != h2 {
		t.Errorf("hashBrowserPath not deterministic: %s != %s", h1, h2)
	}
	if h1 == h3 {
		t.Errorf("hashBrowserPath collision between caches and scriptcache: %s", h1)
	}
	if len(h1) != 16 {
		t.Errorf("expected 16 hex characters, got %d (%s)", len(h1), h1)
	}
}

func TestBrowsers_Chromium_RootAppSupportCaches(t *testing.T) {
	tempDir := t.TempDir()
	appSupportRoot := filepath.Join(tempDir, "Library", "Application Support", "Google", "Chrome")

	// Create dummy files for all 12 root AppSupport leaves
	rootLeaves := []struct {
		relPath string
		size    int64
	}{
		{"OptGuideOnDeviceModel/weights.bin", 1000},
		{"OptGuideOnDeviceClassifierModel/classifier.bin", 2000},
		{"optimization_guide_model_store/model.bin", 3000},
		{"OptimizationGuideModelsManifest/manifest.json", 400},
		{"OptimizationHints/hints.pb", 500},
		{"component_crx_cache/comp.crx", 600},
		{"extensions_crx_cache/ext.crx", 700},
		{"GraphiteDawnCache/data.bin", 800},
		{"GPUPersistentCache/cache.bin", 900},
		{"ShaderCache/shader.bin", 100},
		{"GrShaderCache/grshader.bin", 200},
		{filepath.Join("Crashpad", "completed", "minidump.dmp"), 300},
	}

	var expectedTotal int64
	for _, rl := range rootLeaves {
		expectedTotal += rl.size
		fullPath := filepath.Join(appSupportRoot, rl.relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, make([]byte, rl.size), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Also add a dummy profile with Preferences so profile discovery doesn't produce 0 profiles
	profDir := filepath.Join(appSupportRoot, "Default")
	_ = os.MkdirAll(profDir, 0755)
	_ = os.WriteFile(filepath.Join(profDir, "Preferences"), []byte("{}"), 0644)

	chromeDef, _ := FindBrowserDef("chrome")
	targets, err := scanChromiumBrowser(context.Background(), tempDir, "darwin", chromeDef, false, time.Time{}, 0)
	if err != nil {
		t.Fatalf("scanChromiumBrowser failed: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 consolidated target, got %d", len(targets))
	}
	trg := targets[0]
	if trg.SizeBytes != expectedTotal {
		t.Errorf("expected SizeBytes %d, got %d", expectedTotal, trg.SizeBytes)
	}
	if !trg.IsConsolidated {
		t.Errorf("expected IsConsolidated = true")
	}
	if len(trg.SubPaths) != len(rootLeaves) {
		t.Errorf("expected %d SubPaths, got %d", len(rootLeaves), len(trg.SubPaths))
	}

	// Clean via cleanChromiumBrowser with trg.SubPaths
	if err := cleanChromiumBrowser(tempDir, "darwin", chromeDef, trg.SubPaths); err != nil {
		t.Fatalf("cleanChromiumBrowser failed: %v", err)
	}

	// Verify all root leaves are deleted
	for _, rl := range rootLeaves {
		fullPath := filepath.Join(appSupportRoot, rl.relPath)
		if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
			t.Errorf("expected file %s to be deleted, but it exists", rl.relPath)
		}
	}

	// Verify root container directory still exists
	if _, err := os.Stat(appSupportRoot); os.IsNotExist(err) {
		t.Errorf("expected appSupportRoot %s to be preserved, but was deleted", appSupportRoot)
	}
}

func TestBrowsers_Chromium_NestedOSCaches(t *testing.T) {
	tempDir := t.TempDir()
	appSupportRoot := filepath.Join(tempDir, "Library", "Application Support", "Google", "Chrome")
	cacheRoot := filepath.Join(tempDir, "Library", "Caches", "Google", "Chrome")

	// Profile 1: Nested Cache layout (macOS)
	p1Cache := filepath.Join(cacheRoot, "Profile 1", "Cache")
	_ = os.MkdirAll(filepath.Join(p1Cache, "Cache_Data"), 0755)
	_ = os.MkdirAll(filepath.Join(p1Cache, "No_Vary_Search"), 0755)
	_ = os.WriteFile(filepath.Join(p1Cache, "index"), make([]byte, 100), 0644)
	_ = os.WriteFile(filepath.Join(p1Cache, "Cache_Data", "f_0001"), make([]byte, 500), 0644)
	_ = os.WriteFile(filepath.Join(p1Cache, "No_Vary_Search", "nv_001"), make([]byte, 200), 0644)
	// Expected P1 size = 800 bytes

	// Profile 2: Flat Cache_Data layout (fallback / Linux style)
	p2CacheData := filepath.Join(cacheRoot, "Profile 2", "Cache_Data")
	_ = os.MkdirAll(p2CacheData, 0755)
	_ = os.WriteFile(filepath.Join(p2CacheData, "f_0002"), make([]byte, 400), 0644)
	// Expected P2 size = 400 bytes

	// Add dummy appSupportRoot profiles with Preferences
	for _, prof := range []string{"Profile 1", "Profile 2"} {
		pDir := filepath.Join(appSupportRoot, prof)
		_ = os.MkdirAll(pDir, 0755)
		_ = os.WriteFile(filepath.Join(pDir, "Preferences"), []byte("{}"), 0644)
	}

	chromeDef, _ := FindBrowserDef("chrome")
	targets, err := scanChromiumBrowser(context.Background(), tempDir, "darwin", chromeDef, false, time.Time{}, 0)
	if err != nil {
		t.Fatalf("scanChromiumBrowser failed: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	trg := targets[0]
	expectedSize := int64(800 + 400)
	if trg.SizeBytes != expectedSize {
		t.Errorf("expected SizeBytes %d, got %d", expectedSize, trg.SizeBytes)
	}
}

func TestBrowsers_Chromium_ExtensionsProtection(t *testing.T) {
	tempDir := t.TempDir()
	appSupportRoot := filepath.Join(tempDir, "Library", "Application Support", "Google", "Chrome")

	// Root extension CRX updater cache (disposable)
	crxDir := filepath.Join(appSupportRoot, "extensions_crx_cache")
	_ = os.MkdirAll(crxDir, 0755)
	_ = os.WriteFile(filepath.Join(crxDir, "update.crx"), []byte("downloaded pkg"), 0644)

	// User-installed extensions directory (INVIOLABLE: MUST BE PRESERVED)
	userExtDir := filepath.Join(appSupportRoot, "Default", "Extensions", "cjpalhdlnbpafiamejdnhcphjbkeiagm", "1.52.0_0")
	_ = os.MkdirAll(userExtDir, 0755)
	manifestFile := filepath.Join(userExtDir, "manifest.json")
	_ = os.WriteFile(manifestFile, []byte(`{"name":"uBlock Origin"}`), 0644)

	// Preferences file so Default is recognized as a profile
	_ = os.WriteFile(filepath.Join(appSupportRoot, "Default", "Preferences"), []byte("{}"), 0644)

	chromeDef, _ := FindBrowserDef("chrome")

	// First scan and clean with SubPaths
	targets, err := scanChromiumBrowser(context.Background(), tempDir, "darwin", chromeDef, false, time.Time{}, 0)
	if err != nil {
		t.Fatalf("scanChromiumBrowser failed: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}

	if err := cleanChromiumBrowser(tempDir, "darwin", chromeDef, targets[0].SubPaths); err != nil {
		t.Fatalf("cleanChromiumBrowser failed: %v", err)
	}

	// Verify CRX update cache was cleaned
	if _, err := os.Stat(crxDir); !os.IsNotExist(err) {
		t.Errorf("expected extensions_crx_cache to be deleted, but exists")
	}

	// Verify installed extension is 100% intact
	if _, err := os.Stat(manifestFile); err != nil {
		t.Fatalf("CRITICAL REGRESSION: User installed extension manifest was deleted! %v", err)
	}

	// Also test fallback branch (calling without subPaths)
	_ = os.MkdirAll(crxDir, 0755)
	_ = os.WriteFile(filepath.Join(crxDir, "update2.crx"), []byte("downloaded pkg 2"), 0644)
	if err := cleanChromiumBrowser(tempDir, "darwin", chromeDef); err != nil {
		t.Fatalf("cleanChromiumBrowser fallback failed: %v", err)
	}
	if _, err := os.Stat(crxDir); !os.IsNotExist(err) {
		t.Errorf("expected extensions_crx_cache to be deleted in fallback branch, but exists")
	}
	if _, err := os.Stat(manifestFile); err != nil {
		t.Fatalf("CRITICAL REGRESSION: User installed extension manifest deleted in fallback branch! %v", err)
	}
}

func TestBrowsers_Chromium_MultiBrowserParity(t *testing.T) {
	ctx := context.Background()
	chromiumIDs := []string{"chrome", "brave", "arc", "edge"}

	t.Run("Darwin_Parity_ScanAndClean", func(t *testing.T) {
		tempDir := t.TempDir()

		for _, id := range chromiumIDs {
			bDef, ok := FindBrowserDef(id)
			if !ok {
				t.Fatalf("browser definition not found: %s", id)
			}

			appSupportRoot := bDef.AppSupportDir(tempDir, "darwin")
			cacheRoot := bDef.CacheDir(tempDir, "darwin")
			if appSupportRoot == "" || cacheRoot == "" {
				t.Fatalf("expected non-empty appSupportRoot and cacheRoot for %s on darwin", id)
			}

			// 1. Root model leaf in AppSupport
			modelFile := filepath.Join(appSupportRoot, "OptGuideOnDeviceModel", "weights.bin")
			if err := os.MkdirAll(filepath.Dir(modelFile), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(modelFile, make([]byte, 1024), 0644); err != nil {
				t.Fatal(err)
			}

			// Profile sentinel (Preferences) so profile is recognized
			profDir := filepath.Join(appSupportRoot, "Default")
			if err := os.MkdirAll(profDir, 0755); err != nil {
				t.Fatal(err)
			}
			prefFile := filepath.Join(profDir, "Preferences")
			if err := os.WriteFile(prefFile, []byte("{}"), 0644); err != nil {
				t.Fatal(err)
			}

			// 2. Nested OS cache leaf in CacheDir
			cacheFile := filepath.Join(cacheRoot, "Default", "Cache", "Cache_Data", "f_0001")
			if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(cacheFile, make([]byte, 2048), 0644); err != nil {
				t.Fatal(err)
			}

			// 3. Scan
			targets, err := scanChromiumBrowser(ctx, tempDir, "darwin", bDef, false, time.Time{}, 0)
			if err != nil {
				t.Fatalf("scanChromiumBrowser failed for %s: %v", id, err)
			}

			expectedURI := "browser://" + id + "/caches"
			if len(targets) != 1 {
				t.Fatalf("[%s] expected 1 target, got %d", id, len(targets))
			}
			trg := targets[0]
			if trg.Path != expectedURI {
				t.Errorf("[%s] expected path %s, got %s", id, expectedURI, trg.Path)
			}
			if trg.SizeBytes != 3072 {
				t.Errorf("[%s] expected SizeBytes 3072, got %d", id, trg.SizeBytes)
			}
			if trg.EligibleBytes != 3072 {
				t.Errorf("[%s] expected EligibleBytes 3072, got %d", id, trg.EligibleBytes)
			}
			if !trg.IsConsolidated {
				t.Errorf("[%s] expected IsConsolidated true", id)
			}
			if len(trg.SubPaths) != 2 {
				t.Errorf("[%s] expected 2 SubPaths, got %d: %v", id, len(trg.SubPaths), trg.SubPaths)
			}

			// 4. Clean with SubPaths
			if err := cleanChromiumBrowser(tempDir, "darwin", bDef, trg.SubPaths); err != nil {
				t.Fatalf("[%s] cleanChromiumBrowser failed: %v", id, err)
			}

			// Cache files and model files deleted
			if _, err := os.Stat(modelFile); !os.IsNotExist(err) {
				t.Errorf("[%s] expected modelFile to be deleted, but exists", id)
			}
			if _, err := os.Stat(cacheFile); !os.IsNotExist(err) {
				t.Errorf("[%s] expected cacheFile to be deleted, but exists", id)
			}

			// Parent directories and preferences remain intact
			if _, err := os.Stat(appSupportRoot); os.IsNotExist(err) {
				t.Errorf("[%s] appSupportRoot was deleted", id)
			}
			if _, err := os.Stat(cacheRoot); os.IsNotExist(err) {
				t.Errorf("[%s] cacheRoot was deleted", id)
			}
			if _, err := os.Stat(prefFile); err != nil {
				t.Errorf("[%s] user preferences file was deleted or corrupted: %v", id, err)
			}
		}
	})

	t.Run("EmptyBrowser_ReturnsZeroTargetsWithoutError", func(t *testing.T) {
		tempDir := t.TempDir()
		for _, id := range chromiumIDs {
			bDef, ok := FindBrowserDef(id)
			if !ok {
				t.Fatalf("browser definition not found: %s", id)
			}
			targets, err := scanChromiumBrowser(ctx, tempDir, "darwin", bDef, false, time.Time{}, 0)
			if err != nil {
				t.Fatalf("expected nil error for empty browser %s, got: %v", id, err)
			}
			if len(targets) != 0 {
				t.Errorf("expected 0 targets for empty browser %s, got %d", id, len(targets))
			}
		}
	})

	t.Run("SideBySideAdjacency_UniqueURIs", func(t *testing.T) {
		tempDir := t.TempDir()
		var uris []string
		for _, id := range chromiumIDs {
			bDef, ok := FindBrowserDef(id)
			if !ok {
				t.Fatalf("browser definition not found: %s", id)
			}
			appSupportRoot := bDef.AppSupportDir(tempDir, "darwin")
			modelFile := filepath.Join(appSupportRoot, "ShaderCache", "shader.bin")
			_ = os.MkdirAll(filepath.Dir(modelFile), 0755)
			_ = os.WriteFile(modelFile, make([]byte, 100), 0644)

			targets, err := scanChromiumBrowser(ctx, tempDir, "darwin", bDef, false, time.Time{}, 0)
			if err != nil {
				t.Fatalf("scan error for %s: %v", id, err)
			}
			if len(targets) != 1 {
				t.Fatalf("expected 1 target for %s, got %d", id, len(targets))
			}
			uris = append(uris, targets[0].Path)
		}

		uriSet := make(map[string]bool)
		for _, u := range uris {
			if uriSet[u] {
				t.Errorf("duplicate virtual URI found: %s", u)
			}
			uriSet[u] = true
		}
		if len(uriSet) != 4 {
			t.Errorf("expected 4 distinct URIs, got %d: %v", len(uriSet), uris)
		}
	})

	t.Run("Linux_PlatformConstraints", func(t *testing.T) {
		tempDir := t.TempDir()
		// Arc should return 0 targets on Linux
		arcDef, _ := FindBrowserDef("arc")
		arcTargets, err := scanChromiumBrowser(ctx, tempDir, "linux", arcDef, false, time.Time{}, 0)
		if err != nil {
			t.Fatalf("expected nil error on linux for arc, got: %v", err)
		}
		if len(arcTargets) != 0 {
			t.Errorf("expected 0 targets for arc on linux, got %d", len(arcTargets))
		}

		// Chrome, Brave, Edge can scan on Linux
		for _, id := range []string{"chrome", "brave", "edge"} {
			bDef, _ := FindBrowserDef(id)
			appSupportRoot := bDef.AppSupportDir(tempDir, "linux")
			cacheRoot := bDef.CacheDir(tempDir, "linux")

			// Write flat Linux Cache_Data
			cacheFile := filepath.Join(cacheRoot, "Default", "Cache_Data", "f_linux")
			_ = os.MkdirAll(filepath.Dir(cacheFile), 0755)
			_ = os.WriteFile(cacheFile, make([]byte, 500), 0644)

			// Write Preferences in AppSupport
			_ = os.MkdirAll(filepath.Join(appSupportRoot, "Default"), 0755)
			_ = os.WriteFile(filepath.Join(appSupportRoot, "Default", "Preferences"), []byte("{}"), 0644)

			targets, err := scanChromiumBrowser(ctx, tempDir, "linux", bDef, false, time.Time{}, 0)
			if err != nil {
				t.Fatalf("[%s linux] scan failed: %v", id, err)
			}
			if len(targets) != 1 {
				t.Fatalf("[%s linux] expected 1 target, got %d", id, len(targets))
			}
			if targets[0].SizeBytes != 500 {
				t.Errorf("[%s linux] expected 500 bytes, got %d", id, targets[0].SizeBytes)
			}
		}
	})
}

func TestBrowsers_Chromium_ProcessGuard_FullParity(t *testing.T) {
	ctx := context.Background()

	t.Run("Scan_ProcessBlocked_FullParity", func(t *testing.T) {
		tempDir := t.TempDir()
		appSupportRoot := filepath.Join(tempDir, "Library", "Application Support", "Google", "Chrome")

		// Root model: 3000 bytes
		modelFile := filepath.Join(appSupportRoot, "OptGuideOnDeviceModel", "weights.bin")
		if err := os.MkdirAll(filepath.Dir(modelFile), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(modelFile, make([]byte, 3000), 0644); err != nil {
			t.Fatal(err)
		}

		// Profile cache: 2000 bytes
		profileCache := filepath.Join(appSupportRoot, "Default", "GPUCache", "data_0")
		if err := os.MkdirAll(filepath.Dir(profileCache), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(profileCache, make([]byte, 2000), 0644); err != nil {
			t.Fatal(err)
		}

		// Preferences sentinel
		if err := os.WriteFile(filepath.Join(appSupportRoot, "Default", "Preferences"), []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}

		chromeDef, ok := FindBrowserDef("chrome")
		if !ok {
			t.Fatal("chrome definition not found")
		}

		// Scan with isProcessBlocked = true
		targets, err := scanChromiumBrowser(ctx, tempDir, "darwin", chromeDef, true, time.Time{}, 0)
		if err != nil {
			t.Fatalf("scanChromiumBrowser failed: %v", err)
		}
		if len(targets) != 1 {
			t.Fatalf("expected 1 target, got %d", len(targets))
		}

		trg := targets[0]
		// Invariant: full SizeBytes reported for complete inventory transparency
		if trg.SizeBytes != 5000 {
			t.Errorf("expected SizeBytes 5000, got %d", trg.SizeBytes)
		}
		// Invariant: EligibleBytes strictly 0 when process is blocked
		if trg.EligibleBytes != 0 {
			t.Errorf("expected EligibleBytes 0, got %d", trg.EligibleBytes)
		}
		// Invariant: TTLEligible false
		if trg.TTLEligible {
			t.Errorf("expected TTLEligible false")
		}
		// Invariant: ReviewOnly true
		if !trg.ReviewOnly {
			t.Errorf("expected ReviewOnly true")
		}
		// Invariant: ReviewReason active_process
		if trg.ReviewReason != domain.ReviewReasonActiveProcess {
			t.Errorf("expected ReviewReasonActiveProcess, got %s", trg.ReviewReason)
		}
		if !trg.IsConsolidated {
			t.Errorf("expected IsConsolidated true")
		}
		if len(trg.SubPaths) != 2 {
			t.Errorf("expected 2 SubPaths, got %d", len(trg.SubPaths))
		}
	})

	t.Run("Clean_PreCleanRace_AbortsUnlinking", func(t *testing.T) {
		tempDir := t.TempDir()
		appSupportRoot := filepath.Join(tempDir, "Library", "Application Support", "Google", "Chrome")

		// Create files totaling 5000 bytes
		modelPath := filepath.Join(appSupportRoot, "OptGuideOnDeviceModel")
		modelFile := filepath.Join(modelPath, "weights.bin")
		if err := os.MkdirAll(modelPath, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(modelFile, make([]byte, 3000), 0644); err != nil {
			t.Fatal(err)
		}

		profileCachePath := filepath.Join(appSupportRoot, "Default", "GPUCache")
		profileCacheFile := filepath.Join(profileCachePath, "data_0")
		if err := os.MkdirAll(profileCachePath, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(profileCacheFile, make([]byte, 2000), 0644); err != nil {
			t.Fatal(err)
		}

		// Target previously scanned and eligible
		vPath := "browser://chrome/caches"
		target := domain.Target{
			ID:             hashBrowserPath(vPath),
			Name:           "Google Chrome Caches",
			Path:           vPath,
			SizeBytes:      5000,
			EligibleBytes:  5000,
			TTLEligible:    true,
			ReviewOnly:     false,
			Risk:           domain.RiskLow,
			IsConsolidated: true,
			SubPaths:       []string{modelPath, profileCachePath},
		}

		// Mock executor reporting browser is running at Clean time (exit code 0 with PID output)
		mockExec := cleaner.NewMockExecutor()
		mockExec.Outputs["pgrep -x Google Chrome"] = []byte("77777\n")

		c := NewWithExecutorHomeAndOS(mockExec, tempDir, "darwin")
		err := c.Clean(ctx, []domain.Target{target})
		if err != nil {
			t.Fatalf("Clean returned unexpected error: %v", err)
		}

		// Stage 3 atomic pre-clean revalidation must have aborted unlinking
		// Both cache files must remain 100% intact on disk
		if info, err := os.Stat(modelFile); err != nil || info.Size() != 3000 {
			t.Errorf("modelFile was modified or deleted during pre-clean race! err: %v", err)
		}
		if info, err := os.Stat(profileCacheFile); err != nil || info.Size() != 2000 {
			t.Errorf("profileCacheFile was modified or deleted during pre-clean race! err: %v", err)
		}
	})

	t.Run("Clean_PreCleanUncertain_FailClosed", func(t *testing.T) {
		tempDir := t.TempDir()
		appSupportRoot := filepath.Join(tempDir, "Library", "Application Support", "Google", "Chrome")

		modelPath := filepath.Join(appSupportRoot, "OptGuideOnDeviceModel")
		modelFile := filepath.Join(modelPath, "weights.bin")
		if err := os.MkdirAll(modelPath, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(modelFile, make([]byte, 3000), 0644); err != nil {
			t.Fatal(err)
		}

		vPath := "browser://chrome/caches"
		target := domain.Target{
			ID:             hashBrowserPath(vPath),
			Name:           "Google Chrome Caches",
			Path:           vPath,
			SizeBytes:      3000,
			EligibleBytes:  3000,
			TTLEligible:    true,
			ReviewOnly:     false,
			Risk:           domain.RiskLow,
			IsConsolidated: true,
			SubPaths:       []string{modelPath},
		}

		// Mock executor returning unexpected error -> uncertain status -> fail-closed!
		mockExec := cleaner.NewMockExecutor()
		mockExec.Errors["pgrep -x Google Chrome"] = errors.New("pgrep: permission denied")

		c := NewWithExecutorHomeAndOS(mockExec, tempDir, "darwin")
		err := c.Clean(ctx, []domain.Target{target})
		if err != nil {
			t.Fatalf("Clean returned unexpected error: %v", err)
		}

		// Fail closed: must abort unlinking
		if info, err := os.Stat(modelFile); err != nil || info.Size() != 3000 {
			t.Errorf("modelFile was deleted on uncertain process status! err: %v", err)
		}
	})
}

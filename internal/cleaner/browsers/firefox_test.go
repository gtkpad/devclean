package browsers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestBrowsers_Firefox_ProfileDiscovery(t *testing.T) {
	tempDir := t.TempDir()
	appSupportRoot := filepath.Join(tempDir, "Library", "Application Support", "Firefox", "Profiles")
	cacheRoot := filepath.Join(tempDir, "Library", "Caches", "Firefox", "Profiles")

	profilesToCreate := []struct {
		name     string
		hasPrefs bool
	}{
		{"abc.default", false},
		{"def.default-release", false},
		{"xyz.custom", true},
		{"notaprofile", false},
	}

	for _, p := range profilesToCreate {
		dir := filepath.Join(appSupportRoot, p.name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if p.hasPrefs {
			if err := os.WriteFile(filepath.Join(dir, "prefs.js"), []byte("// Firefox prefs"), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}

	discovered := discoverFirefoxProfiles(appSupportRoot, cacheRoot)
	if len(discovered) != 3 {
		t.Fatalf("expected 3 profiles discovered, got %d", len(discovered))
	}

	discoveredMap := make(map[string]bool)
	for _, p := range discovered {
		discoveredMap[p.Name] = true
	}

	if !discoveredMap["abc.default"] || !discoveredMap["def.default-release"] || !discoveredMap["xyz.custom"] {
		t.Errorf("expected abc.default, def.default-release, and xyz.custom to be discovered, got: %+v", discovered)
	}
	if discoveredMap["notaprofile"] {
		t.Error("notaprofile should not be recognized as a Firefox profile")
	}
}

func TestBrowsers_Firefox_Scan(t *testing.T) {
	tempDir := t.TempDir()
	appSupportRoot := filepath.Join(tempDir, "Library", "Application Support", "Firefox", "Profiles")
	cacheRoot := filepath.Join(tempDir, "Library", "Caches", "Firefox", "Profiles")

	profName := "abcd1234.default-release"
	appProf := filepath.Join(appSupportRoot, profName)
	cacheProf := filepath.Join(cacheRoot, profName)

	files := []struct {
		path string
		size int64
	}{
		{filepath.Join(cacheProf, "cache2", "entries", "000001"), 1024},
		{filepath.Join(cacheProf, "startupCache", "script.bin"), 2048},
		{filepath.Join(cacheProf, "thumbnails", "t1.png"), 512},
		{filepath.Join(appProf, "jumpListCache", "j1"), 256},
	}

	var expectedTotal int64
	for _, f := range files {
		expectedTotal += f.size
		if err := os.MkdirAll(filepath.Dir(f.path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f.path, make([]byte, f.size), 0644); err != nil {
			t.Fatal(err)
		}
	}

	firefoxDef, ok := FindBrowserDef("firefox")
	if !ok {
		t.Fatal("firefox definition not found")
	}

	targets, err := scanFirefox(context.Background(), tempDir, "darwin", firefoxDef, false, time.Time{}, 0)
	if err != nil {
		t.Fatalf("scanFirefox error: %v", err)
	}

	// Strictly exactly 1 consolidated target, ZERO ScriptCache targets (BROW-01, BROW-02, D-01, D-08)
	if len(targets) != 1 {
		t.Fatalf("expected exactly 1 target for Firefox, got %d", len(targets))
	}

	trg := targets[0]
	if trg.Path != "browser://firefox/caches" {
		t.Errorf("expected virtual path browser://firefox/caches, got %s", trg.Path)
	}
	if trg.Name != "Mozilla Firefox Caches" {
		t.Errorf("expected name 'Mozilla Firefox Caches', got %s", trg.Name)
	}
	if trg.SizeBytes != expectedTotal {
		t.Errorf("expected size %d, got %d", expectedTotal, trg.SizeBytes)
	}
	if trg.EligibleBytes != expectedTotal {
		t.Errorf("expected eligible bytes %d, got %d", expectedTotal, trg.EligibleBytes)
	}
	if !trg.TTLEligible {
		t.Error("expected TTLEligible true when maxAgeDays is 0")
	}
	if trg.ReviewOnly {
		t.Error("expected ReviewOnly false when browser is stopped")
	}
	if trg.Category != "browser-cache" {
		t.Errorf("expected Category 'browser-cache', got %s", trg.Category)
	}
}

func TestBrowsers_Firefox_Scan_ProcessBlocked(t *testing.T) {
	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "Library", "Caches", "Firefox", "Profiles", "abc.default", "cache2", "entries", "001")
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cacheFile, make([]byte, 1024), 0644); err != nil {
		t.Fatal(err)
	}

	firefoxDef, _ := FindBrowserDef("firefox")
	targets, err := scanFirefox(context.Background(), tempDir, "darwin", firefoxDef, true, time.Time{}, 0)
	if err != nil {
		t.Fatalf("scanFirefox error: %v", err)
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
		t.Errorf("expected EligibleBytes 0 when process is blocked, got %d", trg.EligibleBytes)
	}
	if trg.TTLEligible {
		t.Error("expected TTLEligible false when process is blocked")
	}
}

func TestBrowsers_Firefox_CleanPreservesSession(t *testing.T) {
	tempDir := t.TempDir()
	disposable, protected, _ := setupBrowserTestFixtures(t, tempDir)

	firefoxDef, _ := FindBrowserDef("firefox")
	if err := cleanFirefox(tempDir, "darwin", firefoxDef); err != nil {
		t.Fatalf("cleanFirefox error: %v", err)
	}

	// 1. Verify Firefox disposable files are removed
	for _, f := range disposable {
		if strings.Contains(f, "Firefox") {
			if _, err := os.Stat(f); !os.IsNotExist(err) {
				t.Errorf("expected disposable Firefox file to be deleted: %s", f)
			}
		}
	}

	// 2. Verify Firefox SQLite databases and session sentinels are 100% INTACT (BROW-03, D-15)
	for _, f := range protected {
		if strings.Contains(f, "Firefox") {
			if _, err := os.Stat(f); err != nil {
				t.Errorf("INVIOLABLE VIOLATION: Firefox session sentinel was removed or corrupted: %s (%v)", f, err)
			}
		}
	}
}

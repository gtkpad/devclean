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

func TestBrowsers_Safari_Isolation(t *testing.T) {
	tempDir := t.TempDir()
	disposable, protected, _ := setupBrowserTestFixtures(t, tempDir)

	safariDef, ok := FindBrowserDef("safari")
	if !ok {
		t.Fatal("safari definition not found")
	}

	targets, err := scanSafari(context.Background(), tempDir, "darwin", safariDef, false, time.Time{}, 0)
	if err != nil {
		t.Fatalf("scanSafari error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected exactly 1 target for Safari, got %d", len(targets))
	}

	trg := targets[0]
	if trg.Path != "browser://safari/caches" {
		t.Errorf("expected virtual path browser://safari/caches, got %s", trg.Path)
	}
	if trg.Name != "Safari Caches" {
		t.Errorf("expected name 'Safari Caches', got %s", trg.Name)
	}
	if trg.SizeBytes == 0 {
		t.Error("expected non-zero Safari cache size")
	}

	// Clean Safari
	if err := cleanSafari(tempDir); err != nil {
		t.Fatalf("cleanSafari error: %v", err)
	}

	// 1. Verify allowlisted Safari cache items deleted
	for _, f := range disposable {
		if strings.Contains(f, "com.apple.Safari") {
			if _, err := os.Stat(f); !os.IsNotExist(err) {
				t.Errorf("expected Safari cache file to be deleted: %s", f)
			}
		}
	}

	// 2. Inviolable rule D-13: ~/Library/Safari is completely untouched!
	for _, f := range protected {
		if strings.Contains(f, "Library/Safari") || strings.Contains(f, "Library\\Safari") {
			if _, err := os.Stat(f); err != nil {
				t.Errorf("INVIOLABLE VIOLATION: ~/Library/Safari private file was corrupted or removed: %s (%v)", f, err)
			}
		}
	}
}

func TestBrowsers_Safari_TCCGracefulDegradation(t *testing.T) {
	tempDir := t.TempDir()
	safariCacheDir := filepath.Join(tempDir, "Library", "Caches", "com.apple.Safari")
	if err := os.MkdirAll(safariCacheDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Simulate TCC / permission denial by making the directory unreadable (0000)
	if err := os.Chmod(safariCacheDir, 0000); err != nil {
		t.Skipf("cannot set permissions 0000: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(safariCacheDir, 0755)
	})

	safariDef, _ := FindBrowserDef("safari")
	targets, err := scanSafari(context.Background(), tempDir, "darwin", safariDef, false, time.Time{}, 0)
	if err != nil {
		t.Fatalf("scanSafari should degrade gracefully without error on permission denial, got: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 degraded target, got %d", len(targets))
	}

	trg := targets[0]
	if !trg.ReviewOnly {
		t.Error("expected ReviewOnly true on TCC / permission denial")
	}
	if trg.ReviewReason != domain.ReviewReasonSystemProtected {
		t.Errorf("expected ReviewReasonSystemProtected, got %s", trg.ReviewReason)
	}
	if trg.EligibleBytes != 0 {
		t.Errorf("expected EligibleBytes 0 on TCC degradation, got %d", trg.EligibleBytes)
	}
	if trg.TTLEligible {
		t.Error("expected TTLEligible false on TCC degradation")
	}
	if trg.ProviderMetrics == nil || !trg.ProviderMetrics.Partial {
		t.Error("expected ProviderMetrics.Partial to be true on TCC degradation")
	}
	if trg.ProviderMetrics.StatusNote != "permission denied (TCC / Full Disk Access required)" {
		t.Errorf("unexpected StatusNote: %q", trg.ProviderMetrics.StatusNote)
	}
}

func TestBrowsers_Safari_ZeroScriptCache(t *testing.T) {
	tempDir := t.TempDir()
	setupBrowserTestFixtures(t, tempDir)

	safariDef, _ := FindBrowserDef("safari")
	targets, err := scanSafari(context.Background(), tempDir, "darwin", safariDef, false, time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, trg := range targets {
		if strings.Contains(trg.Path, "scriptcache") || trg.Category == "browser-scriptcache" {
			t.Errorf("INVIOLABLE VIOLATION: Safari emitted ScriptCache target: %+v", trg)
		}
	}
}

func TestBrowsers_Safari_ProcessBlocked(t *testing.T) {
	tempDir := t.TempDir()
	setupBrowserTestFixtures(t, tempDir)

	safariDef, _ := FindBrowserDef("safari")
	targets, err := scanSafari(context.Background(), tempDir, "darwin", safariDef, true, time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
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
}

func TestBrowsers_ConsolidatedTargets(t *testing.T) {
	tempDir := t.TempDir()
	setupBrowserTestFixtures(t, tempDir)
	ctx := context.Background()

	// 1. Chrome: 1 caches target + 1 scriptcache target
	chromeDef, _ := FindBrowserDef("chrome")
	chTargets, err := scanChromiumBrowser(ctx, tempDir, "darwin", chromeDef, false, time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(chTargets) != 2 {
		t.Errorf("expected Chrome to emit exactly 2 targets (caches + scriptcache), got %d", len(chTargets))
	}

	// 2. Firefox: exactly 1 target, ZERO scriptcache
	firefoxDef, _ := FindBrowserDef("firefox")
	ffTargets, err := scanFirefox(ctx, tempDir, "darwin", firefoxDef, false, time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(ffTargets) != 1 {
		t.Errorf("expected Firefox to emit exactly 1 target, got %d", len(ffTargets))
	}
	if ffTargets[0].Path != "browser://firefox/caches" {
		t.Errorf("expected browser://firefox/caches, got %s", ffTargets[0].Path)
	}

	// 3. Safari: exactly 1 target, ZERO scriptcache
	safariDef, _ := FindBrowserDef("safari")
	safTargets, err := scanSafari(ctx, tempDir, "darwin", safariDef, false, time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(safTargets) != 1 {
		t.Fatalf("expected Safari to emit exactly 1 target, got %d", len(safTargets))
	}
	if safTargets[0].Path != "browser://safari/caches" {
		t.Errorf("expected browser://safari/caches, got %s", safTargets[0].Path)
	}
}

package macos

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestDaemons_CanonicalAllowlist9Daemons(t *testing.T) {
	home := t.TempDir()
	libDir := filepath.Join(home, "Library")

	// Multi-root real paths for all 9 canonical daemons:
	// 1. siri: Caches/com.apple.siri.context, Suggestions
	// 2. mediaanalysisd: Caches/com.apple.mediaanalysisd, Containers/com.apple.mediaanalysisd/Data/tmp
	// 3. geod: Caches/com.apple.geod, Containers/com.apple.geod/Data/tmp
	// 4. geoservices: Caches/GeoServices
	// 5. handoff: Caches/com.apple.coreservices.useractivityd, Group Containers/group.com.apple.coreservices.useractivityd/shared-pasteboard
	// 6. wallpaper: Caches/com.apple.WallpaperAgent
	// 7. helpd: Caches/com.apple.helpd
	// 8. ams: Caches/com.apple.AppleMediaServices
	// 9. parsecd: Caches/com.apple.parsecd
	daemonSubPaths := []string{
		"Caches/com.apple.siri.context",
		"Suggestions",
		"Caches/com.apple.mediaanalysisd",
		"Containers/com.apple.mediaanalysisd/Data/tmp",
		"Caches/com.apple.geod",
		"Containers/com.apple.geod/Data/tmp",
		"Caches/GeoServices",
		"Caches/com.apple.coreservices.useractivityd",
		"Group Containers/group.com.apple.coreservices.useractivityd/shared-pasteboard",
		"Caches/com.apple.WallpaperAgent",
		"Caches/com.apple.helpd",
		"Caches/com.apple.AppleMediaServices",
		"Caches/com.apple.parsecd",
	}

	for _, rel := range daemonSubPaths {
		dir := filepath.Join(libDir, rel)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create daemon dir %s: %v", rel, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "cache.db"), []byte("sample cache content"), 0644); err != nil {
			t.Fatalf("failed to create cache file in %s: %v", rel, err)
		}
	}

	// Also create a non-daemon folder that must NOT be picked up by scanDaemons
	nonDaemon := filepath.Join(libDir, "Caches", "com.example.non-daemon")
	if err := os.MkdirAll(nonDaemon, 0755); err != nil {
		t.Fatalf("failed to create non-daemon dir: %v", err)
	}
	_ = os.WriteFile(filepath.Join(nonDaemon, "file.bin"), []byte("other data"), 0644)

	// Mock executor
	mockExec := newMockProcessExecutor()

	targets, err := scanDaemons(context.Background(), mockExec, home)
	if err != nil {
		t.Fatalf("scanDaemons failed: %v", err)
	}

	if len(targets) != 9 {
		t.Fatalf("expected exactly 9 targets for canonical daemons, got %d", len(targets))
	}

	foundIDs := make(map[string]domain.Target)
	for _, target := range targets {
		foundIDs[target.ID] = target

		if target.Category != "macos-daemon-caches" {
			t.Errorf("target %s category = %s, want macos-daemon-caches", target.ID, target.Category)
		}
		if target.Risk != domain.RiskLow {
			t.Errorf("target %s risk = %s, want %s", target.ID, target.Risk, domain.RiskLow)
		}
		if target.ReviewOnly {
			t.Errorf("target %s ReviewOnly = true, want false (always-on unblocked)", target.ID)
		}
		if target.EligibleBytes == 0 || target.EligibleBytes != target.SizeBytes {
			t.Errorf("target %s EligibleBytes = %d, want %d", target.ID, target.EligibleBytes, target.SizeBytes)
		}
		if !target.TTLEligible {
			t.Errorf("target %s TTLEligible = false, want true", target.ID)
		}
	}

	expectedIDs := []string{
		"macos-daemon-siri",
		"macos-daemon-wallpaper",
		"macos-daemon-mediaanalysisd",
		"macos-daemon-geod",
		"macos-daemon-geoservices",
		"macos-daemon-helpd",
		"macos-daemon-ams",
		"macos-daemon-parsecd",
		"macos-daemon-handoff",
	}

	for _, expectedID := range expectedIDs {
		trg, ok := foundIDs[expectedID]
		if !ok {
			t.Errorf("expected daemon target ID %s was not found in targets", expectedID)
			continue
		}

		// Verify multi-root aggregation
		switch expectedID {
		case "macos-daemon-siri", "macos-daemon-mediaanalysisd", "macos-daemon-geod", "macos-daemon-handoff":
			if !trg.IsConsolidated {
				t.Errorf("target %s should be consolidated (multi-root), got IsConsolidated=false", expectedID)
			}
			if len(trg.SubPaths) != 2 {
				t.Errorf("target %s SubPaths count = %d, want 2", expectedID, len(trg.SubPaths))
			}
		case "macos-daemon-geoservices", "macos-daemon-wallpaper", "macos-daemon-helpd", "macos-daemon-ams", "macos-daemon-parsecd":
			if trg.IsConsolidated {
				t.Errorf("target %s should not be consolidated (single root), got IsConsolidated=true", expectedID)
			}
			if len(trg.SubPaths) != 1 {
				t.Errorf("target %s SubPaths count = %d, want 1", expectedID, len(trg.SubPaths))
			}
		}
	}
}

func TestDaemons_AlwaysOnUnblocked_EvenIfProcessActive(t *testing.T) {
	home := t.TempDir()
	libDir := filepath.Join(home, "Library")

	daemonsToTest := []struct {
		relPath string
		id      string
		proc    string
	}{
		{"Caches/com.apple.mediaanalysisd", "macos-daemon-mediaanalysisd", "mediaanalysisd"},
		{"Caches/com.apple.geod", "macos-daemon-geod", "geod"},
		{"Caches/com.apple.parsecd", "macos-daemon-parsecd", "parsecd"},
	}

	for _, d := range daemonsToTest {
		dir := filepath.Join(libDir, d.relPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create daemon dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "cache.db"), []byte("sample db data"), 0644); err != nil {
			t.Fatalf("failed to create cache file: %v", err)
		}
	}

	// Mock executor where all these processes are actively running (exit code 0) or uncertain (exit code 2)
	mockExec := newMockProcessExecutor()
	mockExec.runningProcesses["mediaanalysisd"] = true
	mockExec.runningProcesses["geod"] = true
	mockExec.runningProcesses["parsecd"] = true
	mockExec.uncertainProcesses["geod"] = 2

	targets, err := scanDaemons(context.Background(), mockExec, home)
	if err != nil {
		t.Fatalf("scanDaemons failed: %v", err)
	}

	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(targets))
	}

	for _, target := range targets {
		if target.ReviewOnly {
			t.Errorf("target %s has ReviewOnly == true, expected false (always-on Apple daemons are unblocked)", target.ID)
		}
		if target.ReviewReason != "" {
			t.Errorf("target %s has ReviewReason == %q, expected empty string", target.ID, target.ReviewReason)
		}
		if target.EligibleBytes == 0 || target.EligibleBytes != target.SizeBytes {
			t.Errorf("target %s has EligibleBytes == %d, expected %d", target.ID, target.EligibleBytes, target.SizeBytes)
		}
		if !target.TTLEligible {
			t.Errorf("target %s has TTLEligible == false, expected true", target.ID)
		}
	}
}

func TestDaemons_MissingCachesDir(t *testing.T) {
	home := t.TempDir()
	targets, err := scanDaemons(context.Background(), newMockProcessExecutor(), home)
	if err != nil {
		t.Fatalf("expected nil error on missing caches dir, got: %v", err)
	}
	if len(targets) != 0 {
		t.Fatalf("expected 0 targets, got %d", len(targets))
	}
}

func TestDaemons_ContextCancelled(t *testing.T) {
	home := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := scanDaemons(ctx, newMockProcessExecutor(), home)
	if err == nil {
		t.Fatalf("expected error on canceled context, got nil")
	}
}

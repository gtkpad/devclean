package macos

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestDaemons_CanonicalAllowlist8Daemons(t *testing.T) {
	home := t.TempDir()
	cachesDir := filepath.Join(home, "Library", "Caches")
	if err := os.MkdirAll(cachesDir, 0755); err != nil {
		t.Fatalf("failed to create caches dir: %v", err)
	}

	// Create folders for all 8 canonical daemons with dummy data
	daemonPaths := []string{
		"com.apple.siri.context",
		"com.apple.WallpaperAgent",
		"com.apple.mediaanalysisd",
		"com.apple.geod",
		"com.apple.helpd",
		"com.apple.AppleMediaServices",
		"com.apple.parsecd",
		"com.apple.coreservices.useractivityd",
	}

	for _, dPath := range daemonPaths {
		dir := filepath.Join(cachesDir, dPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create daemon dir %s: %v", dPath, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "cache.db"), []byte("sample cache content"), 0644); err != nil {
			t.Fatalf("failed to create cache file: %v", err)
		}
	}

	// Also create a non-daemon folder that must NOT be picked up by scanDaemons
	nonDaemon := filepath.Join(cachesDir, "com.example.non-daemon")
	if err := os.MkdirAll(nonDaemon, 0755); err != nil {
		t.Fatalf("failed to create non-daemon dir: %v", err)
	}
	_ = os.WriteFile(filepath.Join(nonDaemon, "file.bin"), []byte("other data"), 0644)

	// Mock executor: all processes stopped (exit code 1)
	mockExec := newMockProcessExecutor()

	targets, err := scanDaemons(context.Background(), mockExec, home)
	if err != nil {
		t.Fatalf("scanDaemons failed: %v", err)
	}

	if len(targets) != 8 {
		t.Fatalf("expected exactly 8 targets for canonical daemons, got %d", len(targets))
	}

	foundIDs := make(map[string]bool)
	for _, target := range targets {
		foundIDs[target.ID] = true

		if target.Category != "macos-daemon-caches" {
			t.Errorf("target %s category = %s, want macos-daemon-caches", target.ID, target.Category)
		}
		if target.Risk != domain.RiskLow {
			t.Errorf("target %s risk = %s, want %s", target.ID, target.Risk, domain.RiskLow)
		}
		if target.ReviewOnly {
			t.Errorf("target %s ReviewOnly = true, want false when stopped", target.ID)
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
		"macos-daemon-helpd",
		"macos-daemon-ams",
		"macos-daemon-parsecd",
		"macos-daemon-handoff",
	}

	for _, expectedID := range expectedIDs {
		if !foundIDs[expectedID] {
			t.Errorf("expected daemon target ID %s was not found in targets", expectedID)
		}
	}
}

func TestDaemons_ActiveProcessGuard(t *testing.T) {
	home := t.TempDir()
	cachesDir := filepath.Join(home, "Library", "Caches")

	// Create mediaanalysisd cache folder
	mediaDir := filepath.Join(cachesDir, "com.apple.mediaanalysisd")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		t.Fatalf("failed to create mediaanalysisd dir: %v", err)
	}
	_ = os.WriteFile(filepath.Join(mediaDir, "analysis.db"), []byte("sample db data"), 0644)

	// Mock executor returns exit 0 (process running)
	mockExec := newMockProcessExecutor()
	mockExec.runningProcesses["mediaanalysisd"] = true

	targets, err := scanDaemons(context.Background(), mockExec, home)
	if err != nil {
		t.Fatalf("scanDaemons failed: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}

	target := targets[0]
	if target.ID != "macos-daemon-mediaanalysisd" {
		t.Fatalf("expected ID macos-daemon-mediaanalysisd, got %s", target.ID)
	}
	if !target.ReviewOnly {
		t.Errorf("expected ReviewOnly == true for active daemon process")
	}
	if target.ReviewReason != domain.ReviewReasonActiveProcess {
		t.Errorf("expected ReviewReason == %s, got %s", domain.ReviewReasonActiveProcess, target.ReviewReason)
	}
	if target.EligibleBytes != 0 {
		t.Errorf("expected EligibleBytes == 0 for active daemon, got %d", target.EligibleBytes)
	}
	if target.TTLEligible {
		t.Errorf("expected TTLEligible == false for active daemon, got true")
	}
}

func TestDaemons_UncertainProcessGuard(t *testing.T) {
	home := t.TempDir()
	cachesDir := filepath.Join(home, "Library", "Caches")

	geodDir := filepath.Join(cachesDir, "com.apple.geod")
	if err := os.MkdirAll(geodDir, 0755); err != nil {
		t.Fatalf("failed to create geod dir: %v", err)
	}
	_ = os.WriteFile(filepath.Join(geodDir, "geo.cache"), []byte("geo content"), 0644)

	// Mock executor with exit code 2 (unexpected error -> fail-closed uncertain)
	mockExec := newMockProcessExecutor()
	mockExec.uncertainProcesses["geod"] = 2

	targets, err := scanDaemons(context.Background(), mockExec, home)
	if err != nil {
		t.Fatalf("scanDaemons failed: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}

	target := targets[0]
	if !target.ReviewOnly {
		t.Errorf("expected ReviewOnly == true when process check is uncertain (fail-closed)")
	}
	if target.ReviewReason != domain.ReviewReasonActiveProcess {
		t.Errorf("expected ReviewReason == %s, got %s", domain.ReviewReasonActiveProcess, target.ReviewReason)
	}
	if target.EligibleBytes != 0 {
		t.Errorf("expected EligibleBytes == 0 when uncertain, got %d", target.EligibleBytes)
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

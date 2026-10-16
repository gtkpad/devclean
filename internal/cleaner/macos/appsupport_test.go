package macos

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/fs"
)

func TestAppSupport_SpotifyPersistentCacheDedicated(t *testing.T) {
	home := t.TempDir()
	appSupportDir := filepath.Join(home, "Library", "Application Support")

	// Create Spotify PersistentCache
	spotifyCacheDir := filepath.Join(appSupportDir, "Spotify", "PersistentCache")
	if err := os.MkdirAll(spotifyCacheDir, 0755); err != nil {
		t.Fatalf("failed to create spotify cache dir: %v", err)
	}
	cacheFile := filepath.Join(spotifyCacheDir, "data.cache")
	if err := os.WriteFile(cacheFile, make([]byte, 10*1024), 0644); err != nil {
		t.Fatalf("failed to write spotify cache file: %v", err)
	}

	// Create Spotify user profile database which must NEVER be touched or inventoried
	spotifyUsersDir := filepath.Join(appSupportDir, "Spotify", "Users")
	if err := os.MkdirAll(spotifyUsersDir, 0755); err != nil {
		t.Fatalf("failed to create spotify users dir: %v", err)
	}
	_ = os.WriteFile(filepath.Join(spotifyUsersDir, "user.db"), make([]byte, 50*1024), 0644)

	// Mock executor with Spotify stopped
	mockExec := newMockProcessExecutor()

	targets, err := scanApplicationSupport(context.Background(), mockExec, home)
	if err != nil {
		t.Fatalf("scanApplicationSupport failed: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target for Spotify PersistentCache, got %d", len(targets))
	}

	target := targets[0]
	if target.ID != "macos-spotify-persistent-cache" {
		t.Errorf("expected ID macos-spotify-persistent-cache, got %s", target.ID)
	}
	if target.Name != "Spotify: PersistentCache" {
		t.Errorf("expected Name 'Spotify: PersistentCache', got %s", target.Name)
	}
	if target.Category != "macos-spotify-cache" {
		t.Errorf("expected category macos-spotify-cache, got %s", target.Category)
	}
	if target.Risk != domain.RiskLow {
		t.Errorf("expected risk RiskLow, got %s", target.Risk)
	}
	if target.ReviewOnly {
		t.Errorf("expected ReviewOnly == false when Spotify is not running")
	}
	if !target.TTLEligible {
		t.Errorf("expected TTLEligible == true")
	}
	if target.Path != spotifyCacheDir {
		t.Errorf("expected target path strictly %s, got %s", spotifyCacheDir, target.Path)
	}

	// Verify size matches cache directory, strictly excluding Spotify/Users (50KB)
	expectedUsage, _ := fs.CalcUsage(spotifyCacheDir)
	if target.SizeBytes != expectedUsage.AllocatedBytes {
		t.Errorf("target SizeBytes = %d, want %d", target.SizeBytes, expectedUsage.AllocatedBytes)
	}
	if target.SizeBytes >= 40*1024 {
		t.Errorf("target SizeBytes = %d unexpectedly counted Spotify/Users directory", target.SizeBytes)
	}
}

func TestAppSupport_SpotifyActiveProcessGuard(t *testing.T) {
	home := t.TempDir()
	appSupportDir := filepath.Join(home, "Library", "Application Support")

	spotifyCacheDir := filepath.Join(appSupportDir, "Spotify", "PersistentCache")
	if err := os.MkdirAll(spotifyCacheDir, 0755); err != nil {
		t.Fatalf("failed to create spotify cache dir: %v", err)
	}
	_ = os.WriteFile(filepath.Join(spotifyCacheDir, "data.cache"), make([]byte, 1024), 0644)

	// Mock executor with Spotify actively running
	mockExec := newMockProcessExecutor()
	mockExec.runningProcesses["Spotify"] = true

	targets, err := scanApplicationSupport(context.Background(), mockExec, home)
	if err != nil {
		t.Fatalf("scanApplicationSupport failed: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}

	target := targets[0]
	if !target.ReviewOnly {
		t.Errorf("expected ReviewOnly == true for active Spotify process")
	}
	if target.ReviewReason != domain.ReviewReasonActiveProcess {
		t.Errorf("expected ReviewReason == %s, got %s", domain.ReviewReasonActiveProcess, target.ReviewReason)
	}
	if target.EligibleBytes != 0 {
		t.Errorf("expected EligibleBytes == 0 for active Spotify, got %d", target.EligibleBytes)
	}
	if target.TTLEligible {
		t.Errorf("expected TTLEligible == false for active Spotify, got true")
	}
}

func TestAppSupport_SpotifyUncertainProcessGuard(t *testing.T) {
	home := t.TempDir()
	appSupportDir := filepath.Join(home, "Library", "Application Support")

	spotifyCacheDir := filepath.Join(appSupportDir, "Spotify", "PersistentCache")
	if err := os.MkdirAll(spotifyCacheDir, 0755); err != nil {
		t.Fatalf("failed to create spotify cache dir: %v", err)
	}
	_ = os.WriteFile(filepath.Join(spotifyCacheDir, "data.cache"), make([]byte, 1024), 0644)

	// Mock executor returning unexpected exit code (uncertain -> fail-closed)
	mockExec := newMockProcessExecutor()
	mockExec.uncertainProcesses["Spotify"] = 2

	targets, err := scanApplicationSupport(context.Background(), mockExec, home)
	if err != nil {
		t.Fatalf("scanApplicationSupport failed: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}

	target := targets[0]
	if !target.ReviewOnly {
		t.Errorf("expected ReviewOnly == true for uncertain process state")
	}
	if target.ReviewReason != domain.ReviewReasonActiveProcess {
		t.Errorf("expected ReviewReason == %s, got %s", domain.ReviewReasonActiveProcess, target.ReviewReason)
	}
	if target.EligibleBytes != 0 {
		t.Errorf("expected EligibleBytes == 0, got %d", target.EligibleBytes)
	}
}

func TestAppSupport_RestrictedLeavesOnly(t *testing.T) {
	home := t.TempDir()
	appSupportDir := filepath.Join(home, "Library", "Application Support")

	// App 1: SomeApp with safe leaves and sensitive databases/preferences
	app1Dir := filepath.Join(appSupportDir, "SomeApp")
	cacheDir := filepath.Join(app1Dir, "Caches")
	logDir := filepath.Join(app1Dir, "Logs")
	crashpadCompletedDir := filepath.Join(app1Dir, "Crashpad", "completed")
	crashpadPendingDir := filepath.Join(app1Dir, "Crashpad", "pending")

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("failed to create log dir: %v", err)
	}
	if err := os.MkdirAll(crashpadCompletedDir, 0755); err != nil {
		t.Fatalf("failed to create crashpad completed dir: %v", err)
	}
	if err := os.MkdirAll(crashpadPendingDir, 0755); err != nil {
		t.Fatalf("failed to create crashpad pending dir: %v", err)
	}

	_ = os.WriteFile(filepath.Join(cacheDir, "data.bin"), make([]byte, 2048), 0644)
	_ = os.WriteFile(filepath.Join(logDir, "debug.log"), make([]byte, 1024), 0644)
	_ = os.WriteFile(filepath.Join(crashpadCompletedDir, "crash.dmp"), make([]byte, 1024), 0644)

	// Sensitive files that MUST NOT be counted or touched
	_ = os.WriteFile(filepath.Join(crashpadPendingDir, "pending.dmp"), make([]byte, 5000), 0644)
	_ = os.WriteFile(filepath.Join(app1Dir, "app.db"), make([]byte, 50*1024), 0644)
	_ = os.WriteFile(filepath.Join(app1Dir, "settings.plist"), make([]byte, 20*1024), 0644)

	// App 2: AnotherApp with lowercase "cache"
	app2Dir := filepath.Join(appSupportDir, "AnotherApp")
	cache2Dir := filepath.Join(app2Dir, "cache")
	if err := os.MkdirAll(cache2Dir, 0755); err != nil {
		t.Fatalf("failed to create cache2 dir: %v", err)
	}
	_ = os.WriteFile(filepath.Join(cache2Dir, "img.cache"), make([]byte, 4096), 0644)
	_ = os.WriteFile(filepath.Join(app2Dir, "accounts.sqlite"), make([]byte, 30*1024), 0644)

	mockExec := newMockProcessExecutor()

	targets, err := scanApplicationSupport(context.Background(), mockExec, home)
	if err != nil {
		t.Fatalf("scanApplicationSupport failed: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 consolidated target, got %d", len(targets))
	}

	target := targets[0]
	if target.ID != "macos-appsupport-leaves-consolidated" {
		t.Errorf("expected ID macos-appsupport-leaves-consolidated, got %s", target.ID)
	}
	if target.Name != "macOS Application Support Caches & Logs" {
		t.Errorf("expected Name 'macOS Application Support Caches & Logs', got %s", target.Name)
	}
	if target.Category != "macos-appsupport-caches" {
		t.Errorf("expected Category macos-appsupport-caches, got %s", target.Category)
	}
	if target.Risk != domain.RiskLow {
		t.Errorf("expected Risk RiskLow, got %s", target.Risk)
	}
	if target.ReviewOnly {
		t.Errorf("expected ReviewOnly == false")
	}

	// Calculate expected usage of strictly the allowed leaves
	u1, _ := fs.CalcUsage(cacheDir)
	u2, _ := fs.CalcUsage(logDir)
	u3, _ := fs.CalcUsage(crashpadCompletedDir)
	u4, _ := fs.CalcUsage(cache2Dir)
	expectedBytes := u1.AllocatedBytes + u2.AllocatedBytes + u3.AllocatedBytes + u4.AllocatedBytes

	if target.SizeBytes != expectedBytes {
		t.Errorf("target SizeBytes = %d, want %d", target.SizeBytes, expectedBytes)
	}
	if target.EligibleBytes != expectedBytes {
		t.Errorf("target EligibleBytes = %d, want %d", target.EligibleBytes, expectedBytes)
	}

	// Ensure the 100KB+ of databases, plists, and pending crashpads were never counted
	if target.SizeBytes >= 50*1024 {
		t.Errorf("target SizeBytes = %d unexpectedly counted database/plist files", target.SizeBytes)
	}
}

func TestAppSupport_WhitelistedSkipped(t *testing.T) {
	home := t.TempDir()
	appSupportDir := filepath.Join(home, "Library", "Application Support")

	// Whitelisted password manager app support directory
	pwDir := filepath.Join(appSupportDir, "1Password", "Caches")
	if err := os.MkdirAll(pwDir, 0755); err != nil {
		t.Fatalf("failed to create 1password dir: %v", err)
	}
	_ = os.WriteFile(filepath.Join(pwDir, "cache.bin"), make([]byte, 1024), 0644)

	mockExec := newMockProcessExecutor()
	targets, err := scanApplicationSupport(context.Background(), mockExec, home)
	if err != nil {
		t.Fatalf("scanApplicationSupport failed: %v", err)
	}

	if len(targets) != 0 {
		t.Fatalf("expected 0 targets because 1Password is whitelisted, got %d", len(targets))
	}
}

func TestAppSupport_NonExistentDir(t *testing.T) {
	home := t.TempDir()
	targets, err := scanApplicationSupport(context.Background(), newMockProcessExecutor(), home)
	if err != nil {
		t.Fatalf("expected nil error on missing app support dir, got %v", err)
	}
	if len(targets) != 0 {
		t.Fatalf("expected 0 targets, got %d", len(targets))
	}
}

func TestAppSupport_ContextCancelled(t *testing.T) {
	home := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := scanApplicationSupport(ctx, newMockProcessExecutor(), home)
	if err == nil {
		t.Fatalf("expected error on cancelled context, got nil")
	}
}

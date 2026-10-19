package macos

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/fs"
)

func TestGroupContainers_AllowlistedCachesOnly(t *testing.T) {
	home := t.TempDir()
	groupsDir := filepath.Join(home, "Library", "Group Containers")
	containerDir := filepath.Join(groupsDir, "group.com.example.app")

	// Allowlisted cache and log directories
	cacheDir := filepath.Join(containerDir, "Library", "Caches")
	logDir := filepath.Join(containerDir, "Library", "Logs")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("failed to create log dir: %v", err)
	}

	cacheFile := filepath.Join(cacheDir, "cache.bin")
	logFile := filepath.Join(logDir, "debug.log")
	if err := os.WriteFile(cacheFile, make([]byte, 1024), 0644); err != nil {
		t.Fatalf("failed to write cache file: %v", err)
	}
	if err := os.WriteFile(logFile, make([]byte, 512), 0644); err != nil {
		t.Fatalf("failed to write log file: %v", err)
	}

	// Non-allowlisted files and directories that must NEVER be counted or touched
	dbFile := filepath.Join(containerDir, "Database.sqlite")
	plistFile := filepath.Join(containerDir, "settings.plist")
	docDir := filepath.Join(containerDir, "Documents")
	if err := os.MkdirAll(docDir, 0755); err != nil {
		t.Fatalf("failed to create doc dir: %v", err)
	}
	_ = os.WriteFile(dbFile, make([]byte, 100*1024), 0644)
	_ = os.WriteFile(plistFile, make([]byte, 50*1024), 0644)
	_ = os.WriteFile(filepath.Join(docDir, "important.doc"), make([]byte, 20*1024), 0644)

	targets, err := scanGroupContainers(context.Background(), home)
	if err != nil {
		t.Fatalf("scanGroupContainers failed: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 consolidated target, got %d", len(targets))
	}

	target := targets[0]
	if target.ID != "macos-group-containers-consolidated" {
		t.Errorf("expected ID macos-group-containers-consolidated, got %s", target.ID)
	}
	if target.Category != "macos-group-caches" {
		t.Errorf("expected category macos-group-caches, got %s", target.Category)
	}
	if target.Risk != domain.RiskLow {
		t.Errorf("expected risk RiskLow, got %s", target.Risk)
	}

	// Target size must account strictly for cache + log, never the 170KB database/plist/doc files
	cacheUsage, _ := fs.CalcUsage(cacheDir)
	logUsage, _ := fs.CalcUsage(logDir)
	expectedCacheBytes := cacheUsage.AllocatedBytes + logUsage.AllocatedBytes

	if target.SizeBytes != expectedCacheBytes {
		t.Errorf("target SizeBytes = %d, want exactly %d (only Library/Caches and Library/Logs)", target.SizeBytes, expectedCacheBytes)
	}
	if target.EligibleBytes != expectedCacheBytes {
		t.Errorf("target EligibleBytes = %d, want %d", target.EligibleBytes, expectedCacheBytes)
	}
	if target.SizeBytes >= 50*1024 {
		t.Errorf("target SizeBytes = %d unexpectedly counted root database/plist files", target.SizeBytes)
	}
	if target.ReviewOnly {
		t.Errorf("expected ReviewOnly == false")
	}

	// Verify target path does not point to the root container directly for wholesale deletion
	if target.Path == containerDir {
		t.Errorf("target path must not be root container folder %s", containerDir)
	}
}

func TestGroupContainers_ExcludesOrbStack(t *testing.T) {
	home := t.TempDir()
	groupsDir := filepath.Join(home, "Library", "Group Containers")

	// Create group containers matching OrbStack patterns
	orbDirs := []string{
		filepath.Join(groupsDir, "u12345.dev.orbstack", "Library", "Caches"),
		filepath.Join(groupsDir, "dev.orbstack.data", "Caches"),
	}

	for _, d := range orbDirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("failed to create orbstack dir: %v", err)
		}
		_ = os.WriteFile(filepath.Join(d, "vm.img"), make([]byte, 1024*1024), 0644)
	}

	targets, err := scanGroupContainers(context.Background(), home)
	if err != nil {
		t.Fatalf("scanGroupContainers failed: %v", err)
	}

	if len(targets) != 0 {
		t.Fatalf("expected 0 targets because OrbStack is strictly excluded, got %d", len(targets))
	}
}

func TestGroupContainers_LargeBundleBreakout(t *testing.T) {
	home := t.TempDir()
	groupsDir := filepath.Join(home, "Library", "Group Containers")
	largeContainer := filepath.Join(groupsDir, "group.com.large.app", "Library", "Caches")
	if err := os.MkdirAll(largeContainer, 0755); err != nil {
		t.Fatalf("failed to create large container: %v", err)
	}

	// 52MB file (> 50MB threshold)
	largeFile := filepath.Join(largeContainer, "large.cache")
	f, err := os.Create(largeFile)
	if err != nil {
		t.Fatalf("failed to create large file: %v", err)
	}
	if err := f.Truncate(52 * 1024 * 1024); err != nil {
		_ = f.Close()
		t.Fatalf("failed to truncate large file: %v", err)
	}
	_ = f.Close()

	targets, err := scanGroupContainers(context.Background(), home)
	if err != nil {
		t.Fatalf("scanGroupContainers failed: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 broken out target, got %d", len(targets))
	}

	target := targets[0]
	if !strings.Contains(target.ID, "group.com.large.app") {
		t.Errorf("expected target ID to contain group name, got %s", target.ID)
	}
	if target.Category != "macos-group-caches" {
		t.Errorf("expected category macos-group-caches, got %s", target.Category)
	}
	if target.SizeBytes < 50*1024*1024 {
		t.Errorf("expected SizeBytes >= 50MB, got %d", target.SizeBytes)
	}
}

func TestGroupContainers_WhitelistedSkipped(t *testing.T) {
	home := t.TempDir()
	groupsDir := filepath.Join(home, "Library", "Group Containers")

	// Whitelisted sensitive containers
	sensitiveDirs := []string{
		filepath.Join(groupsDir, "group.com.apple.keychain", "Library", "Caches"),
		filepath.Join(groupsDir, "group.com.agilebits.1password", "Library", "Caches"),
	}

	for _, d := range sensitiveDirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("failed to create sensitive dir: %v", err)
		}
		_ = os.WriteFile(filepath.Join(d, "cache.bin"), make([]byte, 1024), 0644)
	}

	targets, err := scanGroupContainers(context.Background(), home)
	if err != nil {
		t.Fatalf("scanGroupContainers failed: %v", err)
	}

	if len(targets) != 0 {
		t.Fatalf("expected 0 targets because sensitive containers are whitelisted, got %d", len(targets))
	}
}

func TestGroupContainers_NonExistentDir(t *testing.T) {
	home := t.TempDir()
	targets, err := scanGroupContainers(context.Background(), home)
	if err != nil {
		t.Fatalf("expected nil error on missing group containers dir, got %v", err)
	}
	if len(targets) != 0 {
		t.Fatalf("expected 0 targets, got %d", len(targets))
	}
}

func TestGroupContainers_ContextCancelled(t *testing.T) {
	home := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := scanGroupContainers(ctx, home)
	if err == nil {
		t.Fatalf("expected error on canceled context, got nil")
	}
}

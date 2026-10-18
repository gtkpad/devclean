package leftovers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestLeftovers_AppLeftovers_DiscoveryAndMatching(t *testing.T) {
	tempDir := t.TempDir()
	libraryDir := filepath.Join(tempDir, "Library")
	appSupportDir := filepath.Join(libraryDir, "Application Support")
	cachesDir := filepath.Join(libraryDir, "Caches")
	containersDir := filepath.Join(libraryDir, "Containers")

	for _, d := range []string{appSupportDir, cachesDir, containersDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
	}

	// 1. Installed application directories: should NOT be surfaced as leftovers
	createTestDirWithSentinels(t, appSupportDir, "InstalledApp")
	createTestDirWithSentinels(t, cachesDir, "com.corp.installedapp")

	// 2. Uninstalled application directories: SHOULD be surfaced as leftovers
	createTestDirWithSentinels(t, appSupportDir, "OldApp")
	createTestDirWithSentinels(t, cachesDir, "com.example.oldapp")

	index := newAppIndex()
	index.addAppName("InstalledApp")
	index.addBundleID("com.corp.installedapp")

	targets, err := scanAppLeftovers(context.Background(), libraryDir, index)
	if err != nil {
		t.Fatalf("scanAppLeftovers failed: %v", err)
	}

	for _, target := range targets {
		if strings.Contains(strings.ToLower(target.Name), "installedapp") {
			t.Errorf("installed application was unexpectedly surfaced: %s", target.Name)
		}
	}

	foundOldApp := false
	for _, target := range targets {
		if target.Name == "App Leftover: oldapp" {
			foundOldApp = true
			if !target.ReviewOnly || target.EligibleBytes != 0 {
				t.Errorf("target %s must be ReviewOnly with 0 EligibleBytes", target.ID)
			}
			if target.ReviewReason != domain.ReviewReasonAppLeftover {
				t.Errorf("target %s ReviewReason = %q; want %q", target.ID, target.ReviewReason, domain.ReviewReasonAppLeftover)
			}
			if target.Category != "leftovers-app" {
				t.Errorf("target %s Category = %q; want %q", target.ID, target.Category, "leftovers-app")
			}
		}
	}

	if !foundOldApp {
		t.Errorf("expected leftover target for OldApp, got: %+v", targets)
	}
}

func TestLeftovers_AppLeftovers_Exclusions(t *testing.T) {
	tempDir := t.TempDir()
	libraryDir := filepath.Join(tempDir, "Library")
	appSupportDir := filepath.Join(libraryDir, "Application Support")

	for _, name := range []string{"com.apple.Safari", "docker", "pnpm", "git", "ssh"} {
		createTestDirWithSentinels(t, appSupportDir, name)
	}

	emptyIndex := newAppIndex()
	targets, err := scanAppLeftovers(context.Background(), libraryDir, emptyIndex)
	if err != nil {
		t.Fatalf("scanAppLeftovers failed: %v", err)
	}

	if len(targets) != 0 {
		t.Fatalf("expected 0 targets due to exclusion shield, got %d: %+v", len(targets), targets)
	}
}

func TestLeftovers_AppLeftovers_Consolidation(t *testing.T) {
	tempDir := t.TempDir()
	libraryDir := filepath.Join(tempDir, "Library")
	appSupportDir := filepath.Join(libraryDir, "Application Support")
	cachesDir := filepath.Join(libraryDir, "Caches")
	containersDir := filepath.Join(libraryDir, "Containers")

	// Create residual directories for the same uninstalled app across all 3 locations
	dir1, sentinels1 := createTestDirWithSentinels(t, appSupportDir, "RetiredApp")
	dir2, sentinels2 := createTestDirWithSentinels(t, cachesDir, "com.corp.retiredapp")
	dir3, sentinels3 := createTestDirWithSentinels(t, containersDir, "com.corp.retiredapp")

	emptyIndex := newAppIndex()
	targets, err := scanAppLeftovers(context.Background(), libraryDir, emptyIndex)
	if err != nil {
		t.Fatalf("scanAppLeftovers failed: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected exactly 1 consolidated target, got %d: %+v", len(targets), targets)
	}

	tgt := targets[0]
	if tgt.Name != "App Leftover: retiredapp" {
		t.Errorf("unexpected target Name: %q, want 'App Leftover: retiredapp'", tgt.Name)
	}
	if tgt.Category != "leftovers-app" {
		t.Errorf("unexpected Category: %q, want 'leftovers-app'", tgt.Category)
	}
	if tgt.ReviewReason != domain.ReviewReasonAppLeftover {
		t.Errorf("unexpected ReviewReason: %q, want %q", tgt.ReviewReason, domain.ReviewReasonAppLeftover)
	}
	if !tgt.ReviewOnly || tgt.EligibleBytes != 0 {
		t.Errorf("target must be ReviewOnly with 0 EligibleBytes")
	}

	if tgt.ProviderMetrics == nil {
		t.Fatalf("expected non-nil ProviderMetrics")
	}
	if tgt.ProviderMetrics.TotalCount != 3 {
		t.Errorf("ProviderMetrics.TotalCount = %d, want 3", tgt.ProviderMetrics.TotalCount)
	}

	// Verify all 3 paths are included in StatusNote
	note := tgt.ProviderMetrics.StatusNote
	for _, p := range []string{dir1, dir2, dir3} {
		if !strings.Contains(note, p) {
			t.Errorf("StatusNote %q does not contain path %q", note, p)
		}
	}

	// Verify disk usage was computed
	totalExpectedFiles := len(sentinels1) + len(sentinels2) + len(sentinels3)
	if totalExpectedFiles == 0 || tgt.SizeBytes == 0 {
		t.Errorf("expected non-zero SizeBytes, got %d", tgt.SizeBytes)
	}
}

func TestLeftovers_AppLeftovers_SymlinkGuard(t *testing.T) {
	tempDir := t.TempDir()
	libraryDir := filepath.Join(tempDir, "Library")
	appSupportDir := filepath.Join(libraryDir, "Application Support")
	if err := os.MkdirAll(appSupportDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// Outside directory target
	outsideDir := filepath.Join(tempDir, "outside_dir")
	if err := os.MkdirAll(outsideDir, 0755); err != nil {
		t.Fatalf("mkdir outside failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outsideDir, "secret.txt"), []byte("secret"), 0644); err != nil {
		t.Fatalf("write secret failed: %v", err)
	}

	// Symlink inside Application Support pointing to outside directory
	maliciousLink := filepath.Join(appSupportDir, "MaliciousApp")
	if err := os.Symlink(outsideDir, maliciousLink); err != nil {
		t.Fatalf("symlink failed: %v", err)
	}

	emptyIndex := newAppIndex()
	targets, err := scanAppLeftovers(context.Background(), libraryDir, emptyIndex)
	if err != nil {
		t.Fatalf("scanAppLeftovers failed: %v", err)
	}

	if len(targets) != 0 {
		t.Fatalf("expected 0 targets because unsafe symlink should be ignored, got %d: %+v", len(targets), targets)
	}
}

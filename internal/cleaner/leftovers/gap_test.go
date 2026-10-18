package leftovers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)

func TestLeftovers_Constructors_Defaults(t *testing.T) {
	c1, ok := New().(*leftoversCleaner)
	if !ok {
		t.Fatalf("New() did not return *leftoversCleaner")
	}
	if c1.Name() != "leftovers" {
		t.Errorf("New().Name() = %q, want 'leftovers'", c1.Name())
	}
	if len(c1.appRoots) == 0 {
		t.Errorf("expected default appRoots, got empty")
	}
	if len(c1.launchAgentsDirs) == 0 {
		t.Errorf("expected default launchAgentsDirs, got empty")
	}
	if c1.libraryDir == "" {
		t.Errorf("expected default libraryDir, got empty")
	}

	mockExec := cleaner.NewMockExecutor()
	c2, ok := NewWithExecutor(mockExec).(*leftoversCleaner)
	if !ok {
		t.Fatalf("NewWithExecutor() did not return *leftoversCleaner")
	}
	if c2.Name() != "leftovers" {
		t.Errorf("NewWithExecutor().Name() = %q, want 'leftovers'", c2.Name())
	}

	tempDir := t.TempDir()
	c3, ok := NewWithConfig(mockExec, tempDir, "darwin", nil, nil, "").(*leftoversCleaner)
	if !ok {
		t.Fatalf("NewWithConfig() did not return *leftoversCleaner")
	}
	if c3.libraryDir != filepath.Join(tempDir, "Library") {
		t.Errorf("c3.libraryDir = %q, want %q", c3.libraryDir, filepath.Join(tempDir, "Library"))
	}
	if !c3.IsInstalled() {
		t.Errorf("c3.IsInstalled() = false, want true on darwin with homeDir")
	}

	// Test non-nil explicit parameters
	customRoots := []string{filepath.Join(tempDir, "Apps")}
	customAgents := []string{filepath.Join(tempDir, "Agents")}
	customLib := filepath.Join(tempDir, "Lib")
	c4, ok := NewWithConfig(mockExec, tempDir, "darwin", customRoots, customAgents, customLib).(*leftoversCleaner)
	if !ok || len(c4.appRoots) != 1 || len(c4.launchAgentsDirs) != 1 || c4.libraryDir != customLib {
		t.Errorf("NewWithConfig did not preserve custom slices")
	}
}

func TestLeftovers_IsSafeSymlink_Traversals(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, "root")
	outside := filepath.Join(tempDir, "outside")
	_ = os.MkdirAll(root, 0755)
	_ = os.MkdirAll(outside, 0755)

	// 1. Inside child directory
	child := filepath.Join(root, "child")
	_ = os.MkdirAll(child, 0755)

	// Symlink pointing outside root -> MUST be rejected
	badLink := filepath.Join(root, "bad_link")
	_ = os.Symlink(outside, badLink)

	// Symlink pointing inside root -> ALLOWED
	goodLink := filepath.Join(root, "good_link")
	_ = os.Symlink(child, goodLink)

	// Symlink pointing to non-existent target -> REJECTED
	brokenLink := filepath.Join(root, "broken_link")
	_ = os.Symlink(filepath.Join(outside, "missing"), brokenLink)

	if isSafeSymlink(root, badLink) {
		t.Errorf("isSafeSymlink should return false for link pointing outside root")
	}
	if !isSafeSymlink(root, goodLink) {
		t.Errorf("isSafeSymlink should return true for link pointing inside root")
	}
	if isSafeSymlink(root, brokenLink) {
		t.Errorf("isSafeSymlink should return false for broken link")
	}

	// 2. Symlink pointing to blacklisted directory (e.g. .git)
	gitDir := filepath.Join(root, ".git")
	_ = os.MkdirAll(gitDir, 0755)
	gitLink := filepath.Join(root, "git_link")
	_ = os.Symlink(gitDir, gitLink)

	if isSafeSymlink(root, gitLink) {
		t.Errorf("isSafeSymlink should return false for link resolving to blacklisted path")
	}
}

func TestLeftovers_ParseCorruptPlist(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Nonexistent file error
	_, err := parseLaunchAgentFile(filepath.Join(tempDir, "missing.plist"))
	if err == nil {
		t.Errorf("parseLaunchAgentFile on missing file should return error, got nil")
	}
	_, err = parseAppBundleInfoFile(filepath.Join(tempDir, "missing.plist"))
	if err == nil {
		t.Errorf("parseAppBundleInfoFile on missing file should return error, got nil")
	}

	// 2. Corrupt non-plist file
	corruptFile := filepath.Join(tempDir, "corrupt.plist")
	if err := os.WriteFile(corruptFile, []byte("NOT_A_VALID_PLIST_DATA_STREAM"), 0644); err != nil {
		t.Fatalf("failed to write corrupt file: %v", err)
	}

	_, err = parseLaunchAgentFile(corruptFile)
	if err == nil {
		t.Errorf("expected error decoding corrupt launch agent plist, got nil")
	}

	_, err = parseAppBundleInfoFile(corruptFile)
	if err == nil {
		t.Errorf("expected error decoding corrupt app bundle info plist, got nil")
	}
}

func TestLeftovers_Scan_ContextCancellation(t *testing.T) {
	tempDir := t.TempDir()
	mockExec := cleaner.NewMockExecutor()
	c := NewWithConfig(mockExec, tempDir, "darwin", nil, nil, "")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Scan(ctx, domain.EcosystemRules{Enabled: true})
	if err == nil {
		t.Errorf("expected error from Scan on canceled context, got nil")
	}
}

func TestLeftovers_Clean_UnconditionalDefensiveRefusal(t *testing.T) {
	tempDir := t.TempDir()
	mockExec := cleaner.NewMockExecutor()
	c := NewWithConfig(mockExec, tempDir, "darwin", nil, nil, "")

	testFile := filepath.Join(tempDir, "leftover_sentinel.txt")
	_ = os.WriteFile(testFile, []byte("INVIOLABLE_SENTINEL"), 0644)

	targets := []domain.Target{
		{
			ID:         "app-leftover-test",
			Path:       tempDir,
			ReviewOnly: true,
		},
		{
			ID:         "launchagent-stale-test",
			Path:       testFile,
			ReviewOnly: false, // Attempting to force deletion!
		},
	}

	err := c.Clean(context.Background(), targets)
	if err != nil {
		t.Fatalf("Clean() returned error: %v, want nil", err)
	}

	// File MUST still exist completely unmodified
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("sentinel file was deleted! err: %v", err)
	}
	if string(data) != "INVIOLABLE_SENTINEL" {
		t.Fatalf("sentinel content was corrupted! got %q", string(data))
	}
}

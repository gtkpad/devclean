package homebrew

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)

func TestHomebrewCleaner_Name(t *testing.T) {
	mockExec := cleaner.NewMockExecutor()
	c := NewWithExecutor(mockExec)
	if c.Name() != "homebrew" {
		t.Errorf("expected Name() = 'homebrew', got %q", c.Name())
	}
}

func TestHomebrewCleaner_IsInstalled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	mockExec := cleaner.NewMockExecutor()
	c := NewWithExecutor(mockExec)

	// Present
	if !c.IsInstalled() {
		t.Errorf("expected IsInstalled() = true when brew is in PATH")
	}

	// Missing
	mockExec.Errors["lookpath:brew"] = errors.New("brew not found in PATH")
	if c.IsInstalled() {
		t.Errorf("expected IsInstalled() = false when brew is missing")
	}
}

func TestHomebrewCleaner_Scan_Success(t *testing.T) {
	tempCacheDir := t.TempDir()
	bottleFile := filepath.Join(tempCacheDir, "sample-formula--1.0.tar.gz")
	dummyData := []byte("fake-bottle-binary-data-1234567890")
	if err := os.WriteFile(bottleFile, dummyData, 0o644); err != nil {
		t.Fatalf("failed to write bottle file: %v", err)
	}

	mockExec := cleaner.NewMockExecutor()
	// Output with newline to test strings.TrimSpace
	mockExec.Outputs["brew --cache"] = []byte(tempCacheDir + "\n")

	c := NewWithExecutor(mockExec)
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{})
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}

	target := targets[0]
	if target.Path != "brew://cleanup" {
		t.Errorf("expected Path 'brew://cleanup', got %q", target.Path)
	}
	if target.Category != "virtual-homebrew-cache" {
		t.Errorf("expected Category 'virtual-homebrew-cache', got %q", target.Category)
	}
	if target.Risk != domain.RiskLow {
		t.Errorf("expected RiskLow, got %v", target.Risk)
	}
	if target.SizeBytes != int64(len(dummyData)) {
		t.Errorf("expected SizeBytes %d, got %d", len(dummyData), target.SizeBytes)
	}
	if target.ID != "homebrew-cleanup-cache" {
		t.Errorf("expected ID 'homebrew-cleanup-cache', got %q", target.ID)
	}
	if !target.IsVirtual() {
		t.Errorf("expected IsVirtual() = true for brew://cleanup")
	}
}

func TestHomebrewCleaner_Scan_EmptyCache(t *testing.T) {
	tempCacheDir := t.TempDir()

	mockExec := cleaner.NewMockExecutor()
	mockExec.Outputs["brew --cache"] = []byte(tempCacheDir)

	c := NewWithExecutor(mockExec)
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{})
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}

	if len(targets) != 0 {
		t.Errorf("expected 0 targets for empty cache, got %d", len(targets))
	}
}

func TestHomebrewCleaner_Scan_CommandError(t *testing.T) {
	mockExec := cleaner.NewMockExecutor()
	mockExec.Errors["brew --cache"] = errors.New("command not found")

	c := NewWithExecutor(mockExec)
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{})
	if err != nil {
		t.Fatalf("expected nil error on command error, got %v", err)
	}
	if targets != nil {
		t.Errorf("expected nil targets on command error, got %+v", targets)
	}
}

func TestHomebrewCleaner_Scan_ExcludePaths(t *testing.T) {
	tempCacheDir := t.TempDir()
	bottleFile := filepath.Join(tempCacheDir, "sample.tar.gz")
	_ = os.WriteFile(bottleFile, []byte("data"), 0o644)

	mockExec := cleaner.NewMockExecutor()
	mockExec.Outputs["brew --cache"] = []byte(tempCacheDir)

	c := NewWithExecutor(mockExec)
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{
		ExcludePaths: []string{"brew://*"},
	})
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}

	if len(targets) != 0 {
		t.Errorf("expected target to be excluded, got %d targets", len(targets))
	}
}

func TestHomebrewCleaner_Clean_Success(t *testing.T) {
	mockExec := cleaner.NewMockExecutor()
	c := NewWithExecutor(mockExec)

	targets := []domain.Target{
		{
			ID:       "homebrew-cleanup-cache",
			Name:     "Homebrew cache",
			Path:     "brew://cleanup",
			Category: "virtual-homebrew-cache",
		},
	}

	err := c.Clean(context.Background(), targets)
	if err != nil {
		t.Fatalf("unexpected clean error: %v", err)
	}

	if len(mockExec.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d: %+v", len(mockExec.Calls), mockExec.Calls)
	}
	expected := []string{"brew", "cleanup", "-s"}
	for i, arg := range expected {
		if mockExec.Calls[0][i] != arg {
			t.Errorf("expected arg %d = %q, got %q", i, arg, mockExec.Calls[0][i])
		}
	}
}

func TestHomebrewCleaner_Clean_Fallback(t *testing.T) {
	tempCacheDir := t.TempDir()
	bottleFile := filepath.Join(tempCacheDir, "old-bottle.tar.gz")
	if err := os.WriteFile(bottleFile, []byte("garbage data"), 0o644); err != nil {
		t.Fatalf("failed to write test bottle: %v", err)
	}

	mockExec := cleaner.NewMockExecutor()
	mockExec.Errors["brew cleanup -s"] = errors.New("brew cleanup failed with lock error")
	mockExec.Outputs["brew --cache"] = []byte(tempCacheDir)

	c := NewWithExecutor(mockExec)

	targets := []domain.Target{
		{
			ID:       "homebrew-cleanup-cache",
			Name:     "Homebrew cache",
			Path:     "brew://cleanup",
			Category: "virtual-homebrew-cache",
		},
	}

	err := c.Clean(context.Background(), targets)
	if err != nil {
		t.Fatalf("expected clean to succeed via fallback, got %v", err)
	}

	// Verify both commands were called
	if len(mockExec.Calls) != 2 {
		t.Fatalf("expected 2 calls, got %d: %+v", len(mockExec.Calls), mockExec.Calls)
	}
	if mockExec.Calls[0][0] != "brew" || mockExec.Calls[0][1] != "cleanup" || mockExec.Calls[0][2] != "-s" {
		t.Errorf("expected call 0 to be 'brew cleanup -s', got %+v", mockExec.Calls[0])
	}
	if mockExec.Calls[1][0] != "brew" || mockExec.Calls[1][1] != "--cache" {
		t.Errorf("expected call 1 to be 'brew --cache', got %+v", mockExec.Calls[1])
	}

	// Verify physical file was deleted from disk
	if _, err := os.Stat(bottleFile); !os.IsNotExist(err) {
		t.Errorf("expected physical bottle file to be removed by fallback, but it still exists")
	}
}

func TestHomebrewCleaner_Clean_FallbackFails(t *testing.T) {
	mockExec := cleaner.NewMockExecutor()
	mockExec.Errors["brew cleanup -s"] = errors.New("cleanup failed")
	mockExec.Errors["brew --cache"] = errors.New("cache lookup failed")

	c := NewWithExecutor(mockExec)

	targets := []domain.Target{
		{
			Path: "brew://cleanup",
		},
	}

	err := c.Clean(context.Background(), targets)
	if err == nil {
		t.Fatalf("expected clean error when both cleanup and fallback fail, got nil")
	}
}

func TestHomebrewCleaner_New(t *testing.T) {
	c := New()
	if c == nil || c.Name() != "homebrew" {
		t.Fatalf("expected New() to create homebrew cleaner, got %+v", c)
	}
}

func TestHomebrewCleaner_Clean_PhysicalTarget(t *testing.T) {
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "file.txt")
	_ = os.WriteFile(targetFile, []byte("content"), 0o644)

	mockExec := cleaner.NewMockExecutor()
	c := NewWithExecutor(mockExec)

	targets := []domain.Target{
		{
			Path: targetFile,
		},
	}

	if err := c.Clean(context.Background(), targets); err != nil {
		t.Fatalf("unexpected clean error: %v", err)
	}

	if _, err := os.Stat(targetFile); !os.IsNotExist(err) {
		t.Errorf("expected physical target to be deleted")
	}
}

func TestHomebrewCleaner_Clean_FallbackEmptyCache(t *testing.T) {
	mockExec := cleaner.NewMockExecutor()
	mockExec.Errors["brew cleanup -s"] = errors.New("cleanup failed")
	mockExec.Outputs["brew --cache"] = []byte("   \n")

	c := NewWithExecutor(mockExec)
	targets := []domain.Target{{Path: "brew://cleanup"}}

	if err := c.Clean(context.Background(), targets); err == nil {
		t.Fatalf("expected error when fallback cache path is empty")
	}
}

func TestHomebrewCleaner_Scan_EdgeCases(t *testing.T) {
	mockExec := cleaner.NewMockExecutor()
	c := NewWithExecutor(mockExec)

	// Empty output
	mockExec.Outputs["brew --cache"] = []byte("  \t\n")
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{})
	if err != nil || len(targets) != 0 {
		t.Fatalf("expected empty scan for empty brew --cache output, got %v, %v", targets, err)
	}

	// Cache path is a file, not a directory
	tmpFile := filepath.Join(t.TempDir(), "not-a-dir")
	_ = os.WriteFile(tmpFile, []byte("data"), 0o644)
	mockExec.Outputs["brew --cache"] = []byte(tmpFile)
	targets, err = c.Scan(context.Background(), domain.EcosystemRules{})
	if err != nil || len(targets) != 0 {
		t.Fatalf("expected empty scan for non-dir cache, got %v, %v", targets, err)
	}
}


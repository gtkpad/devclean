package xcode

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)

func TestXcodeCleaner_IsInstalled(t *testing.T) {
	mockExec := cleaner.NewMockExecutor()

	// On Darwin with xcrun available
	cDarwin := NewWithDirExecutorAndOS(t.TempDir(), mockExec, "darwin")
	if !cDarwin.IsInstalled() {
		t.Errorf("expected IsInstalled() = true on darwin when xcrun is present")
	}

	// On Darwin without xcrun
	mockExec.Errors["lookpath:xcrun"] = errors.New("executable file not found in $PATH")
	if cDarwin.IsInstalled() {
		t.Errorf("expected IsInstalled() = false when xcrun is missing")
	}

	// On Linux
	mockExec2 := cleaner.NewMockExecutor()
	cLinux := NewWithDirExecutorAndOS(t.TempDir(), mockExec2, "linux")
	if cLinux.IsInstalled() {
		t.Errorf("expected IsInstalled() = false on linux")
	}
}

func TestXcodeCleaner_Name(t *testing.T) {
	mockExec := cleaner.NewMockExecutor()
	c := NewWithExecutor(mockExec)
	if c.Name() != "xcode" {
		t.Errorf("expected Name() = 'xcode', got %q", c.Name())
	}
}

func TestXcodeCleaner_Scan_GranularDerivedDataAndSimulators(t *testing.T) {
	tmpDir := t.TempDir()
	derivedDataDir := filepath.Join(tmpDir, "DerivedData")
	if err := os.MkdirAll(derivedDataDir, 0o755); err != nil {
		t.Fatalf("failed to create DerivedData: %v", err)
	}

	// ProjectA: old (30 days ago)
	projA := filepath.Join(derivedDataDir, "ProjectA-abc123")
	if err := os.MkdirAll(projA, 0o755); err != nil {
		t.Fatalf("failed to create ProjectA: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projA, "build.log"), []byte("build output"), 0o644); err != nil {
		t.Fatalf("failed to write build file: %v", err)
	}
	oldTime := time.Now().AddDate(0, 0, -30)
	if err := os.Chtimes(projA, oldTime, oldTime); err != nil {
		t.Fatalf("failed to set mtime on ProjectA: %v", err)
	}

	// ProjectB: recent (2 days ago)
	projB := filepath.Join(derivedDataDir, "ProjectB-xyz789")
	if err := os.MkdirAll(projB, 0o755); err != nil {
		t.Fatalf("failed to create ProjectB: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projB, "build.log"), []byte("recent build"), 0o644); err != nil {
		t.Fatalf("failed to write build file: %v", err)
	}
	recentTime := time.Now().AddDate(0, 0, -2)
	if err := os.Chtimes(projB, recentTime, recentTime); err != nil {
		t.Fatalf("failed to set mtime on ProjectB: %v", err)
	}

	// ModuleCache.noindex: should be ignored
	moduleCache := filepath.Join(derivedDataDir, "ModuleCache.noindex")
	if err := os.MkdirAll(moduleCache, 0o755); err != nil {
		t.Fatalf("failed to create ModuleCache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleCache, "cache.dat"), []byte("cache"), 0o644); err != nil {
		t.Fatalf("failed to write cache file: %v", err)
	}

	mockExec := cleaner.NewMockExecutor()
	c := NewWithDirExecutorAndOS(tmpDir, mockExec, "darwin")

	targets, err := c.Scan(context.Background(), domain.EcosystemRules{
		MaxAgeDays: 14,
	})
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}

	// Expect 3 targets: ProjectA (filesystem, eligible), ProjectB (filesystem, not eligible), and simulators (virtual)
	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %d: %+v", len(targets), targets)
	}

	var foundProjA bool
	var foundProjB bool
	var foundSimulators bool

	for _, target := range targets {
		if target.Path == projA {
			foundProjA = true
			if target.Category != "xcode-derived-data" {
				t.Errorf("expected Category 'xcode-derived-data', got %q", target.Category)
			}
			if target.Risk != domain.RiskLow {
				t.Errorf("expected RiskLow, got %v", target.Risk)
			}
			if target.SizeBytes <= 0 {
				t.Errorf("expected SizeBytes > 0, got %d", target.SizeBytes)
			}
			if !target.TTLEligible {
				t.Errorf("expected TTLEligible = true for ProjectA")
			}
			if target.EligibleBytes != target.SizeBytes {
				t.Errorf("expected EligibleBytes == SizeBytes for ProjectA, got %d vs %d", target.EligibleBytes, target.SizeBytes)
			}
			if len(target.ID) != 12 {
				t.Errorf("expected ID length 12 (hex of sha256[:6]), got %q (%d)", target.ID, len(target.ID))
			}
			if target.Name != "Xcode DerivedData: ProjectA-abc123" {
				t.Errorf("expected Name 'Xcode DerivedData: ProjectA-abc123', got %q", target.Name)
			}
		}

		if target.Path == projB {
			foundProjB = true
			if target.TTLEligible {
				t.Errorf("ProjectB is within TTL (recent) and should have TTLEligible = false")
			}
			if target.EligibleBytes != 0 {
				t.Errorf("ProjectB should have EligibleBytes = 0, got %d", target.EligibleBytes)
			}
			if target.SizeBytes <= 0 {
				t.Errorf("ProjectB should have SizeBytes > 0, got %d", target.SizeBytes)
			}
		}

		if target.Path == "xcode://simulators/unavailable" {
			foundSimulators = true
			if target.Category != "virtual-xcode-simulators" {
				t.Errorf("expected Category 'virtual-xcode-simulators', got %q", target.Category)
			}
			if target.Risk != domain.RiskLow {
				t.Errorf("expected RiskLow, got %v", target.Risk)
			}
			if !target.IsVirtual() {
				t.Errorf("expected IsVirtual() to be true for virtual simulator target")
			}
		}

		if target.Path == moduleCache {
			t.Errorf("ModuleCache.noindex should be skipped")
		}
	}

	if !foundProjA {
		t.Errorf("expected ProjectA to be found in targets")
	}
	if !foundProjB {
		t.Errorf("expected ProjectB to be found in targets")
	}
	if !foundSimulators {
		t.Errorf("expected unavailable simulators virtual target to be found")
	}
}

func TestXcodeCleaner_Scan_LinuxSafety(t *testing.T) {
	tmpDir := t.TempDir()
	derivedDataDir := filepath.Join(tmpDir, "DerivedData")
	if err := os.MkdirAll(derivedDataDir, 0o755); err != nil {
		t.Fatalf("failed to create DerivedData: %v", err)
	}

	mockExec := cleaner.NewMockExecutor()
	cLinux := NewWithDirExecutorAndOS(tmpDir, mockExec, "linux")

	targets, err := cLinux.Scan(context.Background(), domain.EcosystemRules{MaxAgeDays: 0})
	if err != nil {
		t.Fatalf("expected Scan() to return nil error on Linux, got %v", err)
	}
	if targets != nil {
		t.Fatalf("expected Scan() to return nil targets on Linux, got %+v", targets)
	}
}

func TestXcodeCleaner_Scan_ExcludePaths(t *testing.T) {
	tmpDir := t.TempDir()
	derivedDataDir := filepath.Join(tmpDir, "DerivedData")
	projA := filepath.Join(derivedDataDir, "ProjectA-abc123")
	if err := os.MkdirAll(projA, 0o755); err != nil {
		t.Fatalf("failed to create ProjectA: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projA, "test.dat"), []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to write data: %v", err)
	}
	oldTime := time.Now().AddDate(0, 0, -30)
	_ = os.Chtimes(projA, oldTime, oldTime)

	mockExec := cleaner.NewMockExecutor()
	c := NewWithDirExecutorAndOS(tmpDir, mockExec, "darwin")

	targets, err := c.Scan(context.Background(), domain.EcosystemRules{
		MaxAgeDays:   0,
		ExcludePaths: []string{"*ProjectA*"},
	})
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}

	for _, target := range targets {
		if target.Path == projA {
			t.Errorf("ProjectA should have been excluded by ExcludePaths")
		}
	}
}

func TestXcodeCleaner_Clean(t *testing.T) {
	tmpDir := t.TempDir()
	derivedDataDir := filepath.Join(tmpDir, "DerivedData")
	projA := filepath.Join(derivedDataDir, "ProjectA-abc123")
	if err := os.MkdirAll(projA, 0o755); err != nil {
		t.Fatalf("failed to create ProjectA: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projA, "dummy.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("failed to write dummy file: %v", err)
	}

	mockExec := cleaner.NewMockExecutor()
	c := NewWithDirExecutorAndOS(tmpDir, mockExec, "darwin")

	targets := []domain.Target{
		{
			ID:       "projA-id",
			Name:     "Project A",
			Path:     projA,
			Category: "xcode-derived-data",
		},
		{
			ID:       "xcode-simulators-unavailable",
			Name:     "iOS Simulators",
			Path:     "xcode://simulators/unavailable",
			Category: "virtual-xcode-simulators",
		},
	}

	err := c.Clean(context.Background(), targets)
	if err != nil {
		t.Fatalf("unexpected clean error: %v", err)
	}

	// Verify ProjectA was deleted
	if _, err := os.Stat(projA); !os.IsNotExist(err) {
		t.Errorf("expected ProjectA to be deleted, but it still exists")
	}

	// Verify simctl command was executed
	if len(mockExec.Calls) != 1 {
		t.Fatalf("expected 1 command call, got %d: %+v", len(mockExec.Calls), mockExec.Calls)
	}
	expectedCall := []string{"xcrun", "simctl", "delete", "unavailable"}
	for i, arg := range expectedCall {
		if mockExec.Calls[0][i] != arg {
			t.Errorf("expected arg %d to be %q, got %q", i, arg, mockExec.Calls[0][i])
		}
	}
}

func TestXcodeCleaner_Clean_SimctlError(t *testing.T) {
	mockExec := cleaner.NewMockExecutor()
	mockExec.Errors["xcrun simctl delete unavailable"] = errors.New("simctl failed")

	c := NewWithDirExecutorAndOS(t.TempDir(), mockExec, "darwin")

	targets := []domain.Target{
		{
			Path: "xcode://simulators/unavailable",
		},
	}

	err := c.Clean(context.Background(), targets)
	if err == nil {
		t.Fatalf("expected clean error when simctl fails, got nil")
	}
}

package docker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)

func TestOrbStack_Scan_CacheAndDataSeparation(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	// Create mock regenerable cache directory
	cacheDir := filepath.Join(fakeHome, "Library", "Caches", "dev.orbstack.OrbStack")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "cache.dat"), make([]byte, 1024), 0644); err != nil {
		t.Fatal(err)
	}

	// Create mock persistent VM/Data directory with sparse file
	dataDir := filepath.Join(fakeHome, "Library", "Group Containers", "group.dev.orbstack", "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "data.img.raw"), make([]byte, 2048), 0644); err != nil {
		t.Fatal(err)
	}

	mockExec := cleaner.NewMockExecutor()
	// Process is stopped (exit code 1)
	mockExec.Errors["pgrep -x OrbStack"] = &cleaner.MockExitError{Code: 1, Msg: "exit status 1"}

	targets, err := scanOrbStack(context.Background(), mockExec, domain.EcosystemRules{})
	if err != nil {
		t.Fatalf("scanOrbStack returned unexpected error: %v", err)
	}

	if len(targets) != 2 {
		t.Fatalf("expected 2 targets (1 cache, 1 data), got %d", len(targets))
	}

	var cacheTarget, dataTarget *domain.Target
	for i := range targets {
		if targets[i].Category == "orbstack-cache" {
			cacheTarget = &targets[i]
		}
		if targets[i].Category == "orbstack-data" {
			dataTarget = &targets[i]
		}
	}

	if cacheTarget == nil {
		t.Fatal("expected orbstack-cache target to be discovered")
	}
	if cacheTarget.Risk != domain.RiskLow {
		t.Errorf("expected cache risk low, got %v", cacheTarget.Risk)
	}
	if cacheTarget.ReviewOnly {
		t.Error("expected stopped cache to have ReviewOnly=false")
	}
	if cacheTarget.EligibleBytes == 0 {
		t.Error("expected stopped cache to have EligibleBytes > 0")
	}
	if !cacheTarget.TTLEligible {
		t.Error("expected stopped cache to have TTLEligible=true")
	}

	if dataTarget == nil {
		t.Fatal("expected orbstack-data target to be discovered")
	}
	if !dataTarget.ReviewOnly {
		t.Error("expected orbstack-data to have ReviewOnly=true")
	}
	if dataTarget.ReviewReason != domain.ReviewReasonSharedStore {
		t.Errorf("expected ReviewReason=%q, got %q", domain.ReviewReasonSharedStore, dataTarget.ReviewReason)
	}
	if dataTarget.EligibleBytes != 0 {
		t.Errorf("expected orbstack-data to have EligibleBytes=0, got %d", dataTarget.EligibleBytes)
	}
	if dataTarget.TTLEligible {
		t.Error("expected orbstack-data to have TTLEligible=false")
	}
	if dataTarget.ProviderMetrics == nil {
		t.Fatal("expected orbstack-data ProviderMetrics to be populated")
	}
	if dataTarget.ProviderMetrics.ApparentBytes < 2048 {
		t.Errorf("expected ApparentBytes >= 2048, got %d", dataTarget.ProviderMetrics.ApparentBytes)
	}
	if dataTarget.ProviderMetrics.AllocatedBytes < 0 {
		t.Errorf("expected non-negative AllocatedBytes, got %d", dataTarget.ProviderMetrics.AllocatedBytes)
	}
	if !strings.Contains(dataTarget.Name, "OrbStack VM/Data (allocated:") || !strings.Contains(dataTarget.Name, "apparent:") {
		t.Errorf("expected Name to contain allocated vs apparent label, got %q", dataTarget.Name)
	}

	// Verify c.Scan() also includes both targets
	c := NewWithExecutor(mockExec)
	allTargets, err := c.Scan(context.Background(), domain.EcosystemRules{})
	if err != nil {
		t.Fatalf("c.Scan() returned error: %v", err)
	}
	var foundCacheInAll, foundDataInAll bool
	for _, trg := range allTargets {
		if trg.Category == "orbstack-cache" {
			foundCacheInAll = true
		}
		if trg.Category == "orbstack-data" {
			foundDataInAll = true
		}
	}
	if !foundCacheInAll {
		t.Error("expected c.Scan() to include orbstack-cache target")
	}
	if !foundDataInAll {
		t.Error("expected c.Scan() to include orbstack-data target")
	}
}

func TestOrbStack_VMData_StrictlyReviewOnly(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	dataDir := filepath.Join(fakeHome, ".orbstack", "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(dataDir, "data.img.raw")
	if err := os.WriteFile(filePath, make([]byte, 4096), 0644); err != nil {
		t.Fatal(err)
	}

	// Set file mod time to 60 days ago
	oldTime := time.Now().AddDate(0, 0, -60)
	_ = os.Chtimes(filePath, oldTime, oldTime)
	_ = os.Chtimes(dataDir, oldTime, oldTime)

	mockExec := cleaner.NewMockExecutor()
	mockExec.Errors["pgrep -x OrbStack"] = &cleaner.MockExitError{Code: 1, Msg: "exit status 1"}

	// Scan with MaxAgeDays = 7 (old file would normally be eligible if not ReviewOnly)
	targets, err := scanOrbStack(context.Background(), mockExec, domain.EcosystemRules{MaxAgeDays: 7})
	if err != nil {
		t.Fatalf("scanOrbStack returned unexpected error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 data target, got %d", len(targets))
	}

	tgt := targets[0]
	if tgt.Category != "orbstack-data" {
		t.Errorf("expected Category 'orbstack-data', got %q", tgt.Category)
	}
	if !tgt.ReviewOnly {
		t.Error("expected ReviewOnly == true")
	}
	if tgt.ReviewReason != domain.ReviewReasonSharedStore {
		t.Errorf("expected ReviewReason %q, got %q", domain.ReviewReasonSharedStore, tgt.ReviewReason)
	}
	if tgt.EligibleBytes != 0 {
		t.Errorf("expected EligibleBytes == 0 even for old files, got %d", tgt.EligibleBytes)
	}
	if tgt.TTLEligible {
		t.Error("expected TTLEligible == false")
	}
}

func TestOrbStack_ProcessGuard_Running(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	cacheDir := filepath.Join(fakeHome, "Library", "Caches", "dev.orbstack.OrbStack")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "cache.dat"), make([]byte, 1024), 0644); err != nil {
		t.Fatal(err)
	}

	mockExec := cleaner.NewMockExecutor()
	mockExec.Outputs["pgrep -x OrbStack"] = []byte("12345\n")

	running, uncertain := checkOrbStackProcess(context.Background(), mockExec)
	if !running {
		t.Error("expected running == true")
	}
	if uncertain {
		t.Error("expected uncertain == false")
	}

	targets, err := scanOrbStack(context.Background(), mockExec, domain.EcosystemRules{})
	if err != nil {
		t.Fatalf("scanOrbStack returned unexpected error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 cache target, got %d", len(targets))
	}

	tgt := targets[0]
	if !tgt.ReviewOnly {
		t.Error("expected ReviewOnly == true when process is running")
	}
	if tgt.ReviewReason != domain.ReviewReasonActiveProcess {
		t.Errorf("expected ReviewReason %q, got %q", domain.ReviewReasonActiveProcess, tgt.ReviewReason)
	}
	if tgt.EligibleBytes != 0 {
		t.Errorf("expected EligibleBytes == 0 when running, got %d", tgt.EligibleBytes)
	}
	if tgt.TTLEligible {
		t.Error("expected TTLEligible == false when running")
	}
}

func TestOrbStack_ProcessGuard_Stopped_Eligible(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	cacheDir := filepath.Join(fakeHome, ".cache", "orbstack")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "cache.dat"), make([]byte, 2048), 0644); err != nil {
		t.Fatal(err)
	}

	mockExec := cleaner.NewMockExecutor()
	mockExec.Errors["pgrep -x OrbStack"] = &cleaner.MockExitError{Code: 1, Msg: "exit status 1"}

	running, uncertain := checkOrbStackProcess(context.Background(), mockExec)
	if running {
		t.Error("expected running == false")
	}
	if uncertain {
		t.Error("expected uncertain == false")
	}

	targets, err := scanOrbStack(context.Background(), mockExec, domain.EcosystemRules{})
	if err != nil {
		t.Fatalf("scanOrbStack returned unexpected error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 cache target, got %d", len(targets))
	}

	tgt := targets[0]
	if tgt.ReviewOnly {
		t.Error("expected ReviewOnly == false when stopped")
	}
	if tgt.EligibleBytes == 0 {
		t.Error("expected EligibleBytes > 0 when stopped")
	}
	if !tgt.TTLEligible {
		t.Error("expected TTLEligible == true when stopped")
	}
}

func TestOrbStack_ProcessGuard_Uncertain_FailClosed(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	cacheDir := filepath.Join(fakeHome, "Library", "Caches", "dev.orbstack.OrbStack")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "cache.dat"), make([]byte, 1024), 0644); err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name     string
		mockExec func() cleaner.CommandExecutor
	}{
		{
			name: "exit code 2 error",
			mockExec: func() cleaner.CommandExecutor {
				m := cleaner.NewMockExecutor()
				m.Errors["pgrep -x OrbStack"] = &cleaner.MockExitError{Code: 2, Msg: "syntax error"}
				return m
			},
		},
		{
			name: "missing pgrep binary error",
			mockExec: func() cleaner.CommandExecutor {
				m := cleaner.NewMockExecutor()
				m.Errors["pgrep -x OrbStack"] = errors.New("exec: command not found")
				return m
			},
		},
		{
			name: "exit code 0 but empty output",
			mockExec: func() cleaner.CommandExecutor {
				m := cleaner.NewMockExecutor()
				m.Outputs["pgrep -x OrbStack"] = []byte("")
				return m
			},
		},
		{
			name: "nil executor",
			mockExec: func() cleaner.CommandExecutor {
				return nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			exec := tc.mockExec()
			running, uncertain := checkOrbStackProcess(context.Background(), exec)
			if running {
				t.Errorf("%s: expected running == false", tc.name)
			}
			if !uncertain {
				t.Errorf("%s: expected uncertain == true (fail closed)", tc.name)
			}

			targets, err := scanOrbStack(context.Background(), exec, domain.EcosystemRules{})
			if err != nil {
				t.Fatalf("%s: scanOrbStack returned unexpected error: %v", tc.name, err)
			}

			if len(targets) != 1 {
				t.Fatalf("%s: expected 1 cache target, got %d", tc.name, len(targets))
			}

			tgt := targets[0]
			if !tgt.ReviewOnly {
				t.Errorf("%s: expected ReviewOnly == true", tc.name)
			}
			if tgt.ReviewReason != domain.ReviewReasonAmbiguous {
				t.Errorf("%s: expected ReviewReason %q, got %q", tc.name, domain.ReviewReasonAmbiguous, tgt.ReviewReason)
			}
			if tgt.EligibleBytes != 0 {
				t.Errorf("%s: expected EligibleBytes == 0, got %d", tc.name, tgt.EligibleBytes)
			}
			if tgt.TTLEligible {
				t.Errorf("%s: expected TTLEligible == false", tc.name)
			}
		})
	}
}

func TestOrbStack_Clean_PreDeletionRevalidation(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	cacheDir := filepath.Join(fakeHome, "Library", "Caches", "dev.orbstack.OrbStack")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	canaryPath := filepath.Join(cacheDir, "canary.bin")
	canaryContent := []byte("canary-cache-must-be-preserved-if-running")
	if err := os.WriteFile(canaryPath, canaryContent, 0644); err != nil {
		t.Fatal(err)
	}

	mockExec := cleaner.NewMockExecutor()
	// Initial scan: cleanly stopped
	mockExec.Errors["pgrep -x OrbStack"] = &cleaner.MockExitError{Code: 1, Msg: "exit status 1"}

	c := NewWithExecutor(mockExec)
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{})
	if err != nil {
		t.Fatalf("c.Scan() error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Category != "orbstack-cache" {
		t.Fatalf("expected orbstack-cache target, got %q", targets[0].Category)
	}

	// Simulate race condition: OrbStack launched after Scan() but before Clean()!
	delete(mockExec.Errors, "pgrep -x OrbStack")
	mockExec.Outputs["pgrep -x OrbStack"] = []byte("43210\n")

	// Call Clean(): pre-deletion revalidation MUST abort cache deletion
	err = c.Clean(context.Background(), targets)
	if err != nil {
		t.Fatalf("Clean() returned unexpected error: %v", err)
	}

	// Verify canary still exists and content is intact
	if _, err := os.Stat(canaryPath); err != nil {
		t.Fatalf("canary file was unexpectedly deleted during Clean(): %v", err)
	}
	readBytes, err := os.ReadFile(canaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(readBytes) != string(canaryContent) {
		t.Errorf("canary content altered: got %q, want %q", string(readBytes), string(canaryContent))
	}

	// Now stop OrbStack again and verify Clean() removes cache
	delete(mockExec.Outputs, "pgrep -x OrbStack")
	mockExec.Errors["pgrep -x OrbStack"] = &cleaner.MockExitError{Code: 1, Msg: "exit status 1"}

	err = c.Clean(context.Background(), targets)
	if err != nil {
		t.Fatalf("Clean() returned unexpected error: %v", err)
	}

	if _, err := os.Stat(canaryPath); !os.IsNotExist(err) {
		t.Errorf("expected canary file to be deleted when stopped, stat err: %v", err)
	}
}

func TestOrbStack_Clean_PreservesVMDataSentinel(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	dataDir := filepath.Join(fakeHome, "Library", "Group Containers", "group.dev.orbstack", "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	sentinelPath := filepath.Join(dataDir, "data.img.raw")
	sentinelContent := []byte("orbstack-vm-data-sentinel-inviolable")
	if err := os.WriteFile(sentinelPath, sentinelContent, 0644); err != nil {
		t.Fatal(err)
	}

	mockExec := cleaner.NewMockExecutor()
	mockExec.Errors["pgrep -x OrbStack"] = &cleaner.MockExitError{Code: 1, Msg: "exit status 1"}

	c := NewWithExecutor(mockExec)
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	var dataTarget *domain.Target
	for i := range targets {
		if targets[i].Category == "orbstack-data" {
			dataTarget = &targets[i]
		}
	}
	if dataTarget == nil {
		t.Fatal("expected orbstack-data target to be discovered")
	}

	// Include both the discovered target and a malicious bypass target
	cleanTargets := []domain.Target{
		*dataTarget,
		{
			ID:         "bypass-orbstack-data",
			Path:       dataDir,
			Category:   "orbstack-data",
			ReviewOnly: false, // Bypass attempt!
		},
		{
			ID:         "bypass-raw-file",
			Path:       sentinelPath,
			Category:   "orbstack-data",
			ReviewOnly: false, // Direct file bypass attempt!
		},
	}

	err = c.Clean(context.Background(), cleanTargets)
	if err != nil {
		t.Fatalf("Clean() returned error: %v", err)
	}

	// Sentinel file must remain completely intact
	fi, err := os.Stat(sentinelPath)
	if err != nil {
		t.Fatalf("sentinel file was deleted! stat err: %v", err)
	}
	if fi.Size() != int64(len(sentinelContent)) {
		t.Errorf("sentinel file size altered: got %d, want %d", fi.Size(), len(sentinelContent))
	}
	readBytes, err := os.ReadFile(sentinelPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(readBytes) != string(sentinelContent) {
		t.Errorf("sentinel file content altered: got %q, want %q", string(readBytes), string(sentinelContent))
	}
}


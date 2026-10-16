package docker

import (
	"context"
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

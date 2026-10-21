package ai

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestAntigravityProfile_Scan_ConsolidatedTarget(t *testing.T) {
	home := t.TempDir()
	profileRoot := filepath.Join(home, ".gemini", "antigravity-browser-profile")

	// Create root caches
	rootLeaves := []string{
		"GraphiteDawnCache",
		"ShaderCache",
		"optimization_guide_model_store",
	}
	for _, leaf := range rootLeaves {
		dir := filepath.Join(profileRoot, leaf)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "cache_data.bin"), []byte("mock binary data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Create Default/ profile caches
	defaultLeaves := []string{
		"Cache",
		"Code Cache",
		"GPUCache",
		filepath.Join("Service Worker", "CacheStorage"),
	}
	defaultProfileDir := filepath.Join(profileRoot, "Default")
	for _, leaf := range defaultLeaves {
		dir := filepath.Join(defaultProfileDir, leaf)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "entry.bin"), []byte("profile cache payload"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mockExec := newMockExecutor()
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}

	targets, err := scanAntigravityBrowserProfile(context.Background(), mockExec, home, rules)
	if err != nil {
		t.Fatalf("scanAntigravityBrowserProfile returned unexpected error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected exactly 1 consolidated target, got %d", len(targets))
	}

	trg := targets[0]
	if trg.Path != "ai://antigravity-browser-profile/cache" {
		t.Errorf("expected Path 'ai://antigravity-browser-profile/cache', got %q", trg.Path)
	}
	if !trg.IsConsolidated {
		t.Errorf("expected IsConsolidated=true, got false")
	}
	if trg.Category != "ai-antigravity-browser-cache" {
		t.Errorf("expected Category 'ai-antigravity-browser-cache', got %q", trg.Category)
	}
	if trg.Risk != domain.RiskLow {
		t.Errorf("expected RiskLow, got %v", trg.Risk)
	}
	if trg.SizeBytes <= 0 {
		t.Errorf("expected positive SizeBytes, got %d", trg.SizeBytes)
	}
	if trg.EligibleBytes != trg.SizeBytes {
		t.Errorf("expected EligibleBytes == SizeBytes (%d), got %d", trg.SizeBytes, trg.EligibleBytes)
	}
	if !trg.TTLEligible {
		t.Errorf("expected TTLEligible=true")
	}

	// Verify all created constituent leaves are in SubPaths
	subPathMap := make(map[string]bool)
	for _, sp := range trg.SubPaths {
		subPathMap[sp] = true
	}

	for _, leaf := range rootLeaves {
		expectedPath := filepath.Join(profileRoot, leaf)
		if !subPathMap[expectedPath] {
			t.Errorf("missing expected root leaf in SubPaths: %s", expectedPath)
		}
	}
	for _, leaf := range defaultLeaves {
		expectedPath := filepath.Join(defaultProfileDir, leaf)
		if !subPathMap[expectedPath] {
			t.Errorf("missing expected profile leaf in SubPaths: %s", expectedPath)
		}
	}
}

func TestAntigravityProfile_SessionShielding(t *testing.T) {
	home := t.TempDir()
	profileRoot := filepath.Join(home, ".gemini", "antigravity-browser-profile")
	defaultDir := filepath.Join(profileRoot, "Default")
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Sensitive user session data that must NEVER be touched
	sessionFiles := []string{
		"Cookies",
		"Login Data",
		"Preferences",
		"History",
		"Bookmarks",
	}
	for _, file := range sessionFiles {
		filePath := filepath.Join(defaultDir, file)
		if err := os.WriteFile(filePath, []byte("sensitive-session-state"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	sessionDirs := []string{
		"Local Storage",
		"Sessions",
		"Extensions",
	}
	for _, dir := range sessionDirs {
		dirPath := filepath.Join(defaultDir, dir)
		if err := os.MkdirAll(dirPath, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dirPath, "data.sqlite"), []byte("sqlite-data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// Ephemeral caches that SHOULD be deleted
	cacheLeaves := []string{
		"Cache",
		"Code Cache",
		"GPUCache",
		filepath.Join("Service Worker", "CacheStorage"),
	}
	for _, leaf := range cacheLeaves {
		dir := filepath.Join(defaultDir, leaf)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "blob.bin"), []byte("ephemeral-cache"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mockExec := newMockExecutor()
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}
	cleanerInstance := NewWithExecutorAndHome(mockExec, nil, home)

	targets, err := cleanerInstance.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var agTarget *domain.Target
	for i := range targets {
		if targets[i].Path == "ai://antigravity-browser-profile/cache" {
			agTarget = &targets[i]
			break
		}
	}
	if agTarget == nil {
		t.Fatalf("expected target ai://antigravity-browser-profile/cache not found in Scan")
	}

	// Verify session files/dirs are not in SubPaths
	for _, sp := range agTarget.SubPaths {
		for _, sf := range sessionFiles {
			if filepath.Base(sp) == sf {
				t.Fatalf("SECURITY VIOLATION: session file %s present in SubPaths: %s", sf, sp)
			}
		}
		for _, sd := range sessionDirs {
			if filepath.Base(sp) == sd {
				t.Fatalf("SECURITY VIOLATION: session dir %s present in SubPaths: %s", sd, sp)
			}
		}
	}

	// Execute Clean
	if err := cleanerInstance.Clean(context.Background(), []domain.Target{*agTarget}); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// Verify all session files still exist intact
	for _, file := range sessionFiles {
		filePath := filepath.Join(defaultDir, file)
		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Errorf("session file was removed or corrupted: %s, err: %v", filePath, err)
		} else if string(data) != "sensitive-session-state" {
			t.Errorf("session file content modified: %s", filePath)
		}
	}

	// Verify all session dirs still exist intact
	for _, dir := range sessionDirs {
		dirPath := filepath.Join(defaultDir, dir)
		if info, err := os.Stat(dirPath); err != nil || !info.IsDir() {
			t.Errorf("session directory was removed: %s, err: %v", dirPath, err)
		}
		sqliteFile := filepath.Join(dirPath, "data.sqlite")
		if _, err := os.Stat(sqliteFile); err != nil {
			t.Errorf("session dir content was removed: %s", sqliteFile)
		}
	}

	// Verify cache leaves are removed
	for _, leaf := range cacheLeaves {
		dir := filepath.Join(defaultDir, leaf)
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("expected cache leaf %s to be deleted, but it still exists", dir)
		}
	}
}

func TestAntigravityProfile_ProcessGuard_FailClosed(t *testing.T) {
	home := t.TempDir()
	profileRoot := filepath.Join(home, ".gemini", "antigravity-browser-profile")
	cacheDir := filepath.Join(profileRoot, "Default", "Cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cacheFile := filepath.Join(cacheDir, "entry.bin")
	if err := os.WriteFile(cacheFile, []byte("cached-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}

	// 1. Running executor (exit 0)
	runningExec := newMockExecutor()
	runningExec.runningProcesses["antigravity-browser"] = true

	targetsRunning, err := scanAntigravityBrowserProfile(context.Background(), runningExec, home, rules)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(targetsRunning) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targetsRunning))
	}
	tRun := targetsRunning[0]
	if !tRun.ReviewOnly {
		t.Errorf("expected ReviewOnly=true when process is running")
	}
	if tRun.ReviewReason != domain.ReviewReasonActiveProcess {
		t.Errorf("expected ReviewReason=%q, got %q", domain.ReviewReasonActiveProcess, tRun.ReviewReason)
	}
	if tRun.EligibleBytes != 0 {
		t.Errorf("expected EligibleBytes=0 when running, got %d", tRun.EligibleBytes)
	}
	if tRun.TTLEligible {
		t.Errorf("expected TTLEligible=false when running")
	}
	if tRun.SizeBytes <= 0 {
		t.Errorf("expected SizeBytes to reflect non-zero size, got %d", tRun.SizeBytes)
	}

	// Attempt Clean with running process -> must abort unlinking
	cleanerRunning := NewWithExecutorAndHome(runningExec, nil, home)
	if err := cleanerRunning.Clean(context.Background(), targetsRunning); err != nil {
		t.Fatalf("Clean returned unexpected error: %v", err)
	}
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		t.Errorf("FAIL CLOSED VIOLATION: cache was deleted while process was running!")
	}

	// 2. Uncertain executor (pgrep returns error / unexpected exit)
	uncertainExec := newMockExecutor()
	uncertainExec.uncertainProcesses["Antigravity"] = true

	targetsUncertain, err := scanAntigravityBrowserProfile(context.Background(), uncertainExec, home, rules)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(targetsUncertain) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targetsUncertain))
	}
	tUncert := targetsUncertain[0]
	if !tUncert.ReviewOnly {
		t.Errorf("expected ReviewOnly=true when process is uncertain")
	}
	if tUncert.EligibleBytes != 0 {
		t.Errorf("expected EligibleBytes=0 when uncertain, got %d", tUncert.EligibleBytes)
	}

	cleanerUncertain := NewWithExecutorAndHome(uncertainExec, nil, home)
	if err := cleanerUncertain.Clean(context.Background(), targetsUncertain); err != nil {
		t.Fatalf("Clean returned unexpected error: %v", err)
	}
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		t.Errorf("FAIL CLOSED VIOLATION: cache was deleted while process status was uncertain!")
	}
}

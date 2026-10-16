package ai

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)

func TestOllama(t *testing.T) {
	tmpDir := t.TempDir()
	modelsDir := filepath.Join(tmpDir, "models")
	manifestsDir := filepath.Join(modelsDir, "manifests")
	blobsDir := filepath.Join(modelsDir, "blobs")

	// 1. Create valid model manifest
	validModelDir := filepath.Join(manifestsDir, "registry.ollama.ai", "library", "llama3")
	if err := os.MkdirAll(validModelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	validManifestPath := filepath.Join(validModelDir, "latest")
	validManifestContent := `{
		"schemaVersion": 2,
		"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
		"config": {
			"digest": "sha256:111111111111"
		},
		"layers": [
			{"digest": "sha256:222222222222"}
		]
	}`
	if err := os.WriteFile(validManifestPath, []byte(validManifestContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. Create corrupted manifest
	corruptDir := filepath.Join(manifestsDir, "corrupted")
	if err := os.MkdirAll(corruptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	corruptManifestPath := filepath.Join(corruptDir, "manifest")
	if err := os.WriteFile(corruptManifestPath, []byte("NOT-VALID-JSON"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 3. Create blobs
	if err := os.MkdirAll(blobsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Valid blobs referenced by manifest
	if err := os.WriteFile(filepath.Join(blobsDir, "sha256-111111111111"), []byte("config-data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blobsDir, "sha256-222222222222"), []byte("layer-data"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Orphan blob (not in any manifest)
	orphanBlobPath := filepath.Join(blobsDir, "sha256-999999999999")
	if err := os.WriteFile(orphanBlobPath, []byte("orphan-blob-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Test 1: Scan with MaxAgeDays = 0 (recent models should NOT be returned)
	rules := domain.EcosystemRules{
		Enabled:    true,
		MaxAgeDays: 0,
	}

	targets, err := scanOllama(modelsDir, rules)
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}

	// Expect 2 targets: orphan blob (RiskLow) and corrupted manifest (RiskMedium)
	var foundOrphan, foundCorrupt, foundRecentModel bool
	for _, target := range targets {
		if target.Category == "ai-ollama-orphan" {
			foundOrphan = true
			if target.Risk != domain.RiskLow {
				t.Errorf("expected RiskLow for orphan blob, got %v", target.Risk)
			}
			if target.Path != orphanBlobPath {
				t.Errorf("expected orphan blob path %s, got %s", orphanBlobPath, target.Path)
			}
		}
		if target.Category == "ai-ollama-corrupted" {
			foundCorrupt = true
			if target.Risk != domain.RiskMedium {
				t.Errorf("expected RiskMedium for corrupted manifest, got %v", target.Risk)
			}
		}
		if target.Category == "ai-ollama-model-stale" {
			foundRecentModel = true
		}
	}

	if !foundOrphan {
		t.Error("expected orphan blob to be detected")
	}
	if !foundCorrupt {
		t.Error("expected corrupted manifest to be detected")
	}
	if foundRecentModel {
		t.Error("recent intact model should NOT be returned as target")
	}

	// Test 2: Scan with MaxAgeDays = 30 and an old model
	oldModelDir := filepath.Join(manifestsDir, "registry.ollama.ai", "library", "oldmodel")
	if err := os.MkdirAll(oldModelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldManifestPath := filepath.Join(oldModelDir, "latest")
	if err := os.WriteFile(oldManifestPath, []byte(validManifestContent), 0o644); err != nil {
		t.Fatal(err)
	}
	// Set ModTime to 60 days ago
	oldTime := time.Now().AddDate(0, 0, -60)
	if err := os.Chtimes(oldManifestPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	rulesWithAge := domain.EcosystemRules{
		Enabled:    true,
		MaxAgeDays: 30,
	}
	targetsWithAge, err := scanOllama(modelsDir, rulesWithAge)
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}

	var foundStaleModel bool
	for _, target := range targetsWithAge {
		if target.Category == "ai-ollama-model-stale" && target.Path == oldManifestPath {
			foundStaleModel = true
			if target.Risk != domain.RiskHigh {
				t.Errorf("expected RiskHigh for stale model (D-01), got %v", target.Risk)
			}
		}
	}
	if !foundStaleModel {
		t.Error("expected stale intact model to be detected with RiskHigh")
	}

	// Test 3: Non-existent directory returns nil, nil
	missingTargets, err := scanOllama(filepath.Join(tmpDir, "nonexistent"), rules)
	if err != nil {
		t.Fatalf("expected nil error for missing directory, got %v", err)
	}
	if missingTargets != nil {
		t.Fatalf("expected nil targets for missing directory, got %v", missingTargets)
	}
}

func TestClaude(t *testing.T) {
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// 1. Create protected files
	protectedFiles := []string{
		"config.json",
		"history.jsonl",
		"settings.json",
		"auth.json",
		"credentials.json",
	}
	for _, pf := range protectedFiles {
		if err := os.WriteFile(filepath.Join(claudeDir, pf), []byte(`{"secret": true}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// 2. Create disposable subdirs
	subdirs := []string{"debug", "tmp", "cache", "telemetry"}
	for _, sub := range subdirs {
		dir := filepath.Join(claudeDir, sub)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "temp_data.bin"), []byte("temporary cached content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// 3. Create loose logs and other files
	if err := os.WriteFile(filepath.Join(claudeDir, "mcp.log"), []byte("log entry"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "other.txt"), []byte("do not delete"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Test 1: Scan with MaxAgeDays = 0
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}
	targets, err := scanClaude(context.Background(), &cleaner.DefaultExecutor{}, tmpDir, rules)
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}

	// Ensure NONE of the protected files or other.txt are targets
	for _, target := range targets {
		base := filepath.Base(target.Path)
		for _, pf := range protectedFiles {
			if base == pf {
				t.Fatalf("SECURITY VIOLATION: Protected file %s was returned as target!", pf)
			}
		}
		if base == "other.txt" {
			t.Fatalf("Unrelated file %s was returned as target!", base)
		}
		if target.Risk != domain.RiskLow {
			t.Errorf("expected RiskLow for Claude target %s, got %v", target.Name, target.Risk)
		}
	}

	// Check that debug, tmp, cache, telemetry subdirs and mcp.log are detected
	foundTargets := make(map[string]bool)
	for _, t := range targets {
		foundTargets[filepath.Base(t.Path)] = true
	}
	for _, sub := range subdirs {
		if !foundTargets[sub] {
			t.Errorf("expected disposable subdir %s to be detected", sub)
		}
	}
	if !foundTargets["mcp.log"] {
		t.Errorf("expected mcp.log to be detected")
	}

	// Test 2: Scan with MaxAgeDays = 30 and aged file/dir
	oldTime := time.Now().AddDate(0, 0, -60)
	recentTime := time.Now().AddDate(0, 0, -2)

	// Set debug dir to old, cache dir to recent
	_ = os.Chtimes(filepath.Join(claudeDir, "debug"), oldTime, oldTime)
	_ = os.Chtimes(filepath.Join(claudeDir, "cache"), recentTime, recentTime)
	// Set mcp.log to old
	_ = os.Chtimes(filepath.Join(claudeDir, "mcp.log"), oldTime, oldTime)

	rulesWithAge := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}
	agedTargets, err := scanClaude(context.Background(), &cleaner.DefaultExecutor{}, tmpDir, rulesWithAge)
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}

	agedFound := make(map[string]domain.Target)
	for _, t := range agedTargets {
		agedFound[filepath.Base(t.Path)] = t
	}
	debugTgt, ok := agedFound["debug"]
	if !ok {
		t.Errorf("expected old debug dir to be detected")
	} else if !debugTgt.TTLEligible {
		t.Errorf("expected old debug dir to have TTLEligible=true")
	}
	logTgt, ok := agedFound["mcp.log"]
	if !ok {
		t.Errorf("expected old mcp.log to be detected")
	} else if !logTgt.TTLEligible {
		t.Errorf("expected old mcp.log to have TTLEligible=true")
	}
	cacheTgt, ok := agedFound["cache"]
	if !ok {
		t.Errorf("expected recent cache dir to be reported in scan")
	} else {
		if cacheTgt.TTLEligible {
			t.Errorf("expected recent cache dir to have TTLEligible=false")
		}
		if cacheTgt.EligibleBytes != 0 {
			t.Errorf("expected recent cache dir to have EligibleBytes=0")
		}
	}

	// Test 3: Non-existent directory returns nil, nil
	missing, err := scanClaude(context.Background(), &cleaner.DefaultExecutor{}, filepath.Join(tmpDir, "nonexistent"), rules)
	if err != nil {
		t.Fatalf("expected nil error for missing directory, got %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil targets for missing directory, got %v", missing)
	}
}

func TestEditors(t *testing.T) {
	home := t.TempDir()

	// Setup basic Editor App Support / Config dirs with Cache and logs
	var codeDir string
	if runtime.GOOS == "darwin" {
		codeDir = filepath.Join(home, "Library", "Application Support", "Code")
	} else {
		codeDir = filepath.Join(home, ".config", "Code")
	}

	if err := os.MkdirAll(filepath.Join(codeDir, "Cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codeDir, "Cache", "v8_cache.bin"), []byte("v8-bytecode"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(codeDir, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codeDir, "logs", "session.log"), []byte("session-log"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Settings files that MUST NOT be touched
	if err := os.WriteFile(filepath.Join(codeDir, "settings.json"), []byte(`{"editor.tabSize": 2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codeDir, "keybindings.json"), []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Scan with MaxAgeDays = 0
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}
	targets, err := scanEditors(context.Background(), &cleaner.DefaultExecutor{}, home, rules)
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}

	var foundChromium, foundLogs, foundSettings bool
	for _, target := range targets {
		if target.Risk != domain.RiskLow {
			t.Errorf("expected RiskLow for editor target %s, got %v", target.Name, target.Risk)
		}
		if target.Path == filepath.Join(codeDir, "settings.json") || target.Path == filepath.Join(codeDir, "keybindings.json") {
			foundSettings = true
		}
		if target.Path == "editor://code/chromium-cache" {
			foundChromium = true
		}
		if target.Path == "editor://code/logs" {
			foundLogs = true
		}
	}

	if foundSettings {
		t.Fatal("SECURITY VIOLATION: settings.json or keybindings.json returned as target!")
	}
	if !foundChromium {
		t.Error("expected Code chromium-cache to be detected")
	}
	if !foundLogs {
		t.Error("expected Code logs to be detected")
	}
}

func TestHuggingFace(t *testing.T) {
	tmpDir := t.TempDir()
	hfRoot := filepath.Join(tmpDir, ".cache", "huggingface")

	hubDir := filepath.Join(hfRoot, "hub")
	datasetsDir := filepath.Join(hfRoot, "datasets")

	bertDir := filepath.Join(hubDir, "models--bert-base-uncased")
	llamaDir := filepath.Join(hubDir, "models--llama-3-8b")
	glueDir := filepath.Join(datasetsDir, "glue")

	for _, d := range []string{bertDir, llamaDir, glueDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "snapshot.bin"), []byte("model-or-dataset-data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	oldTime60 := time.Now().AddDate(0, 0, -60)
	oldTime45 := time.Now().AddDate(0, 0, -45)
	recentTime2 := time.Now().AddDate(0, 0, -2)

	_ = os.Chtimes(bertDir, oldTime60, oldTime60)
	_ = os.Chtimes(glueDir, oldTime45, oldTime45)
	_ = os.Chtimes(llamaDir, recentTime2, recentTime2)

	// Test with MaxAgeDays = 30
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}
	targets, err := scanHuggingFace(hfRoot, rules)
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}

	var foundBert, foundGlue, foundLlama bool
	for _, target := range targets {
		if target.Risk != domain.RiskMedium {
			t.Errorf("expected RiskMedium (D-03) for HuggingFace target %s, got %v", target.Name, target.Risk)
		}

		if target.Path == bertDir {
			foundBert = true
			if target.Category != "ai-huggingface-hub" {
				t.Errorf("expected category ai-huggingface-hub, got %s", target.Category)
			}
			if !target.TTLEligible {
				t.Errorf("expected TTLEligible=true for old bert")
			}
		}
		if target.Path == glueDir {
			foundGlue = true
			if target.Category != "ai-huggingface-datasets" {
				t.Errorf("expected category ai-huggingface-datasets, got %s", target.Category)
			}
			if !target.TTLEligible {
				t.Errorf("expected TTLEligible=true for old glue")
			}
		}
		if target.Path == llamaDir {
			foundLlama = true
			if target.TTLEligible {
				t.Errorf("recent llama-3-8b should have TTLEligible=false when MaxAgeDays=30")
			}
			if target.EligibleBytes != 0 {
				t.Errorf("recent llama-3-8b should have EligibleBytes=0, got %d", target.EligibleBytes)
			}
		}
	}

	if !foundBert {
		t.Error("expected old bert-base-uncased to be detected")
	}
	if !foundGlue {
		t.Error("expected old glue dataset to be detected")
	}
	if !foundLlama {
		t.Error("recent llama-3-8b should be detected in scan")
	}

	// Test with MaxAgeDays = 0 (all detected)
	rulesAll := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}
	targetsAll, err := scanHuggingFace(hfRoot, rulesAll)
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}
	if len(targetsAll) != 3 {
		t.Errorf("expected 3 targets when MaxAgeDays=0, got %d", len(targetsAll))
	}

	// Test non-existent directory returns nil, nil
	missing, err := scanHuggingFace(filepath.Join(tmpDir, "nonexistent"), rules)
	if err != nil {
		t.Fatalf("expected nil error for missing directory, got %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil targets for missing directory, got %v", missing)
	}
}

func TestAICleaner_FullScanAndClean(t *testing.T) {
	ctx := context.Background()
	tmpHome := t.TempDir()

	c := NewWithHome(nil, tmpHome)

	// Verify metadata methods
	if c.Name() != "ai" {
		t.Errorf("expected name 'ai', got '%s'", c.Name())
	}
	if !c.IsInstalled() {
		t.Error("expected IsInstalled to be true")
	}

	// Verify New() constructor
	cDefault := New(nil)
	if cDefault.Name() != "ai" {
		t.Errorf("expected name 'ai' for default constructor, got '%s'", cDefault.Name())
	}

	// Create fixtures across all 4 sub-cleaners in tmpHome
	// 1. Claude
	claudeTmp := filepath.Join(tmpHome, ".claude", "tmp")
	if err := os.MkdirAll(claudeTmp, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeTmp, "session.tmp"), []byte("temp"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. Editors
	var codeAppDir string
	if runtime.GOOS == "darwin" {
		codeAppDir = filepath.Join(tmpHome, "Library", "Application Support", "Code")
	} else {
		codeAppDir = filepath.Join(tmpHome, ".config", "Code")
	}
	if err := os.MkdirAll(filepath.Join(codeAppDir, "Cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codeAppDir, "Cache", "v8_cache.bin"), []byte("v8"), 0o644); err != nil {
		t.Fatal(err)
	}
	vscodeExt := filepath.Join(tmpHome, ".vscode", "extensions")
	obsoleteExt := filepath.Join(vscodeExt, "publisher.obsolete-1.0.0")
	if err := os.MkdirAll(obsoleteExt, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vscodeExt, ".obsolete"), []byte(`{"publisher.obsolete-1.0.0": true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// 3. Ollama
	ollamaBlobs := filepath.Join(tmpHome, ".ollama", "models", "blobs")
	if err := os.MkdirAll(ollamaBlobs, 0o755); err != nil {
		t.Fatal(err)
	}
	orphanBlob := filepath.Join(ollamaBlobs, "sha256-orphan123")
	if err := os.WriteFile(orphanBlob, []byte("orphan"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 4. HuggingFace
	hfHub := filepath.Join(tmpHome, ".cache", "huggingface", "hub", "models--test")
	if err := os.MkdirAll(hfHub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hfHub, "weight.bin"), []byte("weight"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Scan
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}
	targets, err := c.Scan(ctx, rules)
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}

	if len(targets) < 4 {
		t.Fatalf("expected at least 4 targets across all sub-cleaners, got %d", len(targets))
	}

	// Verify each target exists on disk before clean (skipping virtual targets like editor://)
	for _, target := range targets {
		if target.IsVirtual() {
			continue
		}
		if _, err := os.Stat(target.Path); err != nil {
			t.Fatalf("target path %s does not exist before Clean(): %v", target.Path, err)
		}
	}

	// Clean
	if err := c.Clean(ctx, targets); err != nil {
		t.Fatalf("unexpected clean error: %v", err)
	}

	// Verify all physical target paths are deleted
	for _, target := range targets {
		if target.IsVirtual() {
			continue
		}
		if _, err := os.Stat(target.Path); !os.IsNotExist(err) {
			t.Errorf("target path %s still exists after Clean()", target.Path)
		}
	}

	// Test context cancellation
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	_, err = c.Scan(canceledCtx, rules)
	if err == nil {
		t.Error("expected error when scanning with canceled context, got nil")
	}

	// Test empty homeDir
	cEmpty := NewWithHome(nil, "")
	emptyTargets, err := cEmpty.Scan(ctx, rules)
	if err != nil || len(emptyTargets) != 0 {
		t.Errorf("expected empty targets and nil error for empty home, got targets=%v, err=%v", emptyTargets, err)
	}

	// Test Clean with canceled context
	if err := c.Clean(canceledCtx, []domain.Target{{Path: "/tmp/fake"}}); err == nil {
		t.Error("expected error when cleaning with canceled context, got nil")
	}
}

func TestAICleaner_ConstructorsWithExecutor(t *testing.T) {
	mockExec := newMockExecutor()

	// 1. NewWithExecutor
	cExec := NewWithExecutor(mockExec, nil)
	if cExec == nil || cExec.Name() != "ai" {
		t.Fatalf("expected valid cleaner with name 'ai'")
	}

	// 2. NewWithExecutorAndHome
	tmpHome := t.TempDir()
	cExecHome := NewWithExecutorAndHome(mockExec, nil, tmpHome)
	if cExecHome == nil || cExecHome.Name() != "ai" {
		t.Fatalf("expected valid cleaner with name 'ai'")
	}

	// 3. Clean should safely skip virtual targets (e.g. editor://) without os.RemoveAll error
	virtualTarget := domain.Target{
		ID:        "test-virt",
		Name:      "VS Code: Chromium & GPU Caches",
		Path:      "editor://code/chromium-cache",
		SizeBytes: 1024,
	}
	if !virtualTarget.IsVirtual() {
		t.Fatalf("expected virtualTarget.IsVirtual() = true")
	}
	if err := cExecHome.Clean(context.Background(), []domain.Target{virtualTarget}); err != nil {
		t.Fatalf("expected Clean to safely skip virtual target, got error: %v", err)
	}
}

func TestAICleaner_Clean_PreDeletionRevalidation(t *testing.T) {
	home := t.TempDir()
	setupMockEditor(t, home, "Code", "com.microsoft.VSCode")

	mockExec := newMockExecutor()
	// Initially Code is NOT running
	mockExec.runningProcesses["Code"] = false

	c := NewWithExecutorAndHome(mockExec, nil, home)
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var codeTargets []domain.Target
	for _, target := range targets {
		if strings.HasPrefix(target.Path, "editor://code/") {
			codeTargets = append(codeTargets, target)
		}
	}
	if len(codeTargets) == 0 {
		t.Fatal("expected Code targets from Scan")
	}

	// Now simulate user starting Code before Clean() runs! (D-07 race condition)
	mockExec.runningProcesses["Code"] = true

	// Also simulate claude CLI target
	claudeDir := filepath.Join(home, ".claude", "cache")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	claudeSentinel := filepath.Join(claudeDir, "sentinel.txt")
	if err := os.WriteFile(claudeSentinel, []byte("claude-cache-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	claudeTarget := domain.Target{
		ID:        "claude-cache-test",
		Path:      claudeDir,
		Category:  "ai-claude-cache",
		SizeBytes: 18,
	}

	// Also set claude running
	mockExec.runningProcesses["claude"] = true

	allTargets := append(codeTargets, claudeTarget)

	err = c.Clean(context.Background(), allTargets)
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// Verify that neither Code targets nor Claude cache were deleted!
	if _, err := os.Stat(claudeSentinel); os.IsNotExist(err) {
		t.Fatal("SECURITY VIOLATION: Claude cache was deleted while claude CLI was running!")
	}

	// Check Code cache leaf
	var codeCacheDir string
	if runtime.GOOS == "darwin" {
		codeCacheDir = filepath.Join(home, "Library", "Caches", "com.microsoft.VSCode", "Cache", "data_0")
	} else {
		codeCacheDir = filepath.Join(home, ".cache", "Code", "Cache", "data_0")
	}
	if _, err := os.Stat(codeCacheDir); os.IsNotExist(err) {
		t.Fatal("SECURITY VIOLATION: Code cache was deleted while Code editor was running!")
	}
}

func TestAICleaner_Clean_VirtualEditorTarget(t *testing.T) {
	home := t.TempDir()
	mockExec := newMockExecutor()
	c := NewWithExecutorAndHome(mockExec, nil, home)

	var appDir string
	if runtime.GOOS == "darwin" {
		appDir = filepath.Join(home, "Library", "Application Support", "Code")
	} else {
		appDir = filepath.Join(home, ".config", "Code")
	}

	// 1. WorkspaceStorage setup
	ws1 := filepath.Join(appDir, "User", "workspaceStorage", "ws1")
	if err := os.MkdirAll(ws1, 0o755); err != nil {
		t.Fatal(err)
	}
	vscdbPrimary := filepath.Join(ws1, "state.vscdb")
	vscdbBackup := filepath.Join(ws1, "state.vscdb.backup")
	wsJson := filepath.Join(ws1, "workspace.json")
	if err := os.WriteFile(vscdbPrimary, []byte("PRIMARY-DB"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vscdbBackup, []byte("BACKUP-DB"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wsJson, []byte(`{"id":"ws1"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. Extensions setup
	extDir := filepath.Join(home, ".vscode", "extensions")
	if err := os.MkdirAll(extDir, 0o755); err != nil {
		t.Fatal(err)
	}
	obsoleteFile := filepath.Join(extDir, ".obsolete")
	if err := os.WriteFile(obsoleteFile, []byte(`{"pub.obs-1.0.0": true, "pub.active-1.0.0": false}`), 0o644); err != nil {
		t.Fatal(err)
	}
	obsExtPath := filepath.Join(extDir, "pub.obs-1.0.0")
	if err := os.MkdirAll(obsExtPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(obsExtPath, "package.json"), []byte("obs-pkg"), 0o644); err != nil {
		t.Fatal(err)
	}
	activeExtPath := filepath.Join(extDir, "pub.active-1.0.0")
	if err := os.MkdirAll(activeExtPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(activeExtPath, "package.json"), []byte("active-pkg"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 3. CachedExtensionVSIXs setup
	vsixDir := filepath.Join(appDir, "CachedExtensionVSIXs")
	if err := os.MkdirAll(vsixDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vsixDir, "tool.vsix"), []byte("vsix-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 4. Chromium cache leaf setup
	codeCacheLeaf := filepath.Join(appDir, "Code Cache", "js")
	if err := os.MkdirAll(codeCacheLeaf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codeCacheLeaf, "cached.js"), []byte("js-cache"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 5. Logs leaf setup
	logLeaf := filepath.Join(appDir, "logs", "session1")
	if err := os.MkdirAll(logLeaf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logLeaf, "main.log"), []byte("log-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Call Clean with the 4 virtual targets
	targets := []domain.Target{
		{Path: "editor://code/workspace-backups", Category: "editor-workspace-backups"},
		{Path: "editor://code/extensions", Category: "editor-extensions"},
		{Path: "editor://code/chromium-cache", Category: "editor-chromium-cache"},
		{Path: "editor://code/logs", Category: "editor-logs"},
	}

	if err := c.Clean(context.Background(), targets); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// Assert: state.vscdb and workspace.json are untouched!
	if _, err := os.Stat(vscdbPrimary); err != nil {
		t.Fatalf("SECURITY VIOLATION: primary state.vscdb was removed: %v", err)
	}
	if _, err := os.Stat(wsJson); err != nil {
		t.Fatalf("SECURITY VIOLATION: workspace.json was removed: %v", err)
	}
	// Assert: state.vscdb.backup is deleted!
	if _, err := os.Stat(vscdbBackup); !os.IsNotExist(err) {
		t.Fatalf("state.vscdb.backup should have been deleted")
	}

	// Assert: .obsolete metadata file and active extension are untouched!
	if _, err := os.Stat(obsoleteFile); err != nil {
		t.Fatalf("SECURITY VIOLATION: .obsolete metadata was removed: %v", err)
	}
	if _, err := os.Stat(activeExtPath); err != nil {
		t.Fatalf("SECURITY VIOLATION: active extension was removed: %v", err)
	}
	// Assert: obsolete extension and CachedExtensionVSIXs are deleted!
	if _, err := os.Stat(obsExtPath); !os.IsNotExist(err) {
		t.Fatalf("obsolete extension folder should have been deleted")
	}
	if _, err := os.Stat(vsixDir); !os.IsNotExist(err) {
		t.Fatalf("CachedExtensionVSIXs should have been deleted")
	}

	// Assert: Code Cache and logs are deleted!
	if _, err := os.Stat(codeCacheLeaf); !os.IsNotExist(err) {
		t.Fatalf("Code Cache leaf should have been deleted")
	}
	if _, err := os.Stat(logLeaf); !os.IsNotExist(err) {
		t.Fatalf("logs leaf should have been deleted")
	}
}

func TestAICleaner_Clean_SymlinkProtection(t *testing.T) {
	home := t.TempDir()
	mockExec := newMockExecutor()
	c := NewWithExecutorAndHome(mockExec, nil, home)

	// External precious directory
	externalDir := t.TempDir()
	preciousFile := filepath.Join(externalDir, "precious.txt")
	if err := os.WriteFile(preciousFile, []byte("DO-NOT-DELETE"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Physical target path that is a symlink to externalDir
	symlinkTarget := filepath.Join(home, "symlink-cache")
	if err := os.Symlink(externalDir, symlinkTarget); err != nil {
		t.Fatal(err)
	}

	target := domain.Target{
		Path:      symlinkTarget,
		Category:  "ai-generic",
		SizeBytes: 100,
	}

	if err := c.Clean(context.Background(), []domain.Target{target}); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// Assert external directory and file are completely intact
	if _, err := os.Stat(preciousFile); err != nil {
		t.Fatalf("SECURITY VIOLATION: external target was deleted via symlink: %v", err)
	}
}



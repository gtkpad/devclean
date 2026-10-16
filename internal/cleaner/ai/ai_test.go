package ai

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
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
	targets, err := scanClaude(claudeDir, rules)
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
	agedTargets, err := scanClaude(claudeDir, rulesWithAge)
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
	missing, err := scanClaude(filepath.Join(tmpDir, "nonexistent"), rules)
	if err != nil {
		t.Fatalf("expected nil error for missing directory, got %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil targets for missing directory, got %v", missing)
	}
}

func TestEditors(t *testing.T) {
	home := t.TempDir()

	// 1. Setup Editor App Support / Config dirs
	var codeDir, cursorDir string
	if runtime.GOOS == "darwin" {
		codeDir = filepath.Join(home, "Library", "Application Support", "Code")
		cursorDir = filepath.Join(home, "Library", "Application Support", "Cursor")
	} else {
		codeDir = filepath.Join(home, ".config", "Code")
		cursorDir = filepath.Join(home, ".config", "Cursor")
	}

	for _, edDir := range []string{codeDir, cursorDir} {
		if err := os.MkdirAll(filepath.Join(edDir, "CachedData"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(edDir, "CachedData", "v8_cache.bin"), []byte("v8-bytecode"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(edDir, "logs"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(edDir, "logs", "session.log"), []byte("session-log"), 0o644); err != nil {
			t.Fatal(err)
		}
		// Settings files that MUST NOT be touched
		if err := os.WriteFile(filepath.Join(edDir, "settings.json"), []byte(`{"editor.tabSize": 2}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(edDir, "keybindings.json"), []byte(`[]`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// 2. Setup Extensions (.vscode and .cursor)
	vscodeExt := filepath.Join(home, ".vscode", "extensions")
	if err := os.MkdirAll(filepath.Join(vscodeExt, "publisher.ext-1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vscodeExt, "publisher.ext-1.0.0", "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vscodeExt, "publisher.ext-2.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vscodeExt, "publisher.ext-2.0.0", "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// .obsolete marks ext-1.0.0 as obsolete
	obsoleteJSON := `{"publisher.ext-1.0.0": true, "publisher.ext-3.0.0": false}`
	if err := os.WriteFile(filepath.Join(vscodeExt, ".obsolete"), []byte(obsoleteJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	// Scan with MaxAgeDays = 0
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}
	targets, err := scanEditors(context.Background(), &cleaner.DefaultExecutor{}, home, rules)
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}

	var foundCodeCachedData, foundCodeLogs bool
	var foundCursorCachedData, foundCursorLogs bool
	var foundObsoleteExt, foundActiveExt, foundSettings bool

	for _, target := range targets {
		if target.Risk != domain.RiskLow {
			t.Errorf("expected RiskLow for editor target %s, got %v", target.Name, target.Risk)
		}

		base := filepath.Base(target.Path)
		if base == "settings.json" || base == "keybindings.json" {
			foundSettings = true
		}
		if base == "publisher.ext-2.0.0" {
			foundActiveExt = true
		}
		if base == "publisher.ext-1.0.0" {
			foundObsoleteExt = true
			if target.Category != "editor-obsolete-extension" {
				t.Errorf("expected category editor-obsolete-extension, got %s", target.Category)
			}
		}
		if target.Category == "editor-cached-data" {
			if target.Path == filepath.Join(codeDir, "CachedData") {
				foundCodeCachedData = true
			}
			if target.Path == filepath.Join(cursorDir, "CachedData") {
				foundCursorCachedData = true
			}
		}
		if target.Category == "editor-logs" {
			if target.Path == filepath.Join(codeDir, "logs") {
				foundCodeLogs = true
			}
			if target.Path == filepath.Join(cursorDir, "logs") {
				foundCursorLogs = true
			}
		}
	}

	if foundSettings {
		t.Fatal("SECURITY VIOLATION: settings.json or keybindings.json returned as target!")
	}
	if foundActiveExt {
		t.Fatal("SECURITY VIOLATION: active extension publisher.ext-2.0.0 returned as target!")
	}
	if !foundCodeCachedData {
		t.Error("expected Code CachedData to be detected")
	}
	if !foundCodeLogs {
		t.Error("expected Code logs to be detected")
	}
	if !foundCursorCachedData {
		t.Error("expected Cursor CachedData to be detected")
	}
	if !foundCursorLogs {
		t.Error("expected Cursor logs to be detected")
	}
	if !foundObsoleteExt {
		t.Error("expected obsolete extension publisher.ext-1.0.0 to be detected")
	}

	// Test TTL filtering: set Code CachedData to recent, Cursor CachedData to old
	oldTime := time.Now().AddDate(0, 0, -60)
	recentTime := time.Now().AddDate(0, 0, -2)
	_ = os.Chtimes(filepath.Join(codeDir, "CachedData"), recentTime, recentTime)
	_ = os.Chtimes(filepath.Join(cursorDir, "CachedData"), oldTime, oldTime)
	_ = os.Chtimes(filepath.Join(vscodeExt, "publisher.ext-1.0.0"), oldTime, oldTime)

	rulesAge := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}
	agedTargets, err := scanEditors(context.Background(), &cleaner.DefaultExecutor{}, home, rulesAge)
	if err != nil {
		t.Fatalf("unexpected scan error with TTL: %v", err)
	}

	var foundRecentCodeCachedData, foundOldCursorCachedData bool
	for _, target := range agedTargets {
		if target.Path == filepath.Join(codeDir, "CachedData") {
			foundRecentCodeCachedData = true
			if target.TTLEligible {
				t.Errorf("recent Code CachedData should have TTLEligible=false")
			}
			if target.EligibleBytes != 0 {
				t.Errorf("recent Code CachedData should have EligibleBytes=0, got %d", target.EligibleBytes)
			}
		}
		if target.Path == filepath.Join(cursorDir, "CachedData") {
			foundOldCursorCachedData = true
			if !target.TTLEligible {
				t.Errorf("old Cursor CachedData should have TTLEligible=true")
			}
			if target.EligibleBytes != target.SizeBytes {
				t.Errorf("old Cursor CachedData should have EligibleBytes == SizeBytes")
			}
		}
	}
	if !foundRecentCodeCachedData {
		t.Errorf("recent Code CachedData should be reported in scan")
	}
	if !foundOldCursorCachedData {
		t.Errorf("old Cursor CachedData should be reported in scan")
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

	// Verify each target exists on disk before clean
	for _, target := range targets {
		if _, err := os.Stat(target.Path); err != nil {
			t.Fatalf("target path %s does not exist before Clean(): %v", target.Path, err)
		}
	}

	// Clean
	if err := c.Clean(ctx, targets); err != nil {
		t.Fatalf("unexpected clean error: %v", err)
	}

	// Verify all target paths are deleted
	for _, target := range targets {
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


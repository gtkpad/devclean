package ai

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/trash"
)

// fileSHA256 computes the hexadecimal SHA-256 checksum of a file via streaming.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func TestE2E_FullLifecycle(t *testing.T) {
	// Passo 1: Isolamento hermético de ambiente
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	configDir := filepath.Join(homeDir, ".config", "devclean")
	trashDir := filepath.Join(tmpDir, "trash")
	workspace := filepath.Join(tmpDir, "workspace")

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create configDir: %v", err)
	}
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		t.Fatalf("failed to create trashDir: %v", err)
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	// Auxiliares para IsInstalled dos cleaners em ambientes herméticos
	_ = os.MkdirAll(filepath.Join(homeDir, ".cargo", "bin"), 0o755)
	_ = os.MkdirAll(filepath.Join(homeDir, "Library", "Caches", "uv"), 0o755)
	_ = os.MkdirAll(filepath.Join(homeDir, ".cache", "uv"), 0o755)

	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(homeDir, ".local", "share"))
	t.Setenv("PNPM_HOME", filepath.Join(homeDir, "pnpm"))
	t.Setenv("YARN_CACHE_FOLDER", filepath.Join(homeDir, "yarn"))
	t.Setenv("CARGO_HOME", filepath.Join(homeDir, ".cargo"))
	t.Setenv("PIP_CACHE_DIR", filepath.Join(homeDir, ".pip"))

	// Passo 2: Fixtures Multi-Ecossistema
	// 2a. Node.js project: package.json + node_modules/index.js
	nodeApp := filepath.Join(workspace, "node-project")
	nodeModules := filepath.Join(nodeApp, "node_modules")
	if err := os.MkdirAll(nodeModules, 0o755); err != nil {
		t.Fatalf("failed to create node_modules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nodeApp, "package.json"), []byte(`{"name":"e2e-node"}`), 0o644); err != nil {
		t.Fatalf("failed to create package.json: %v", err)
	}
	nodeFile := filepath.Join(nodeModules, "index.js")
	nodeContent := "module.exports = { name: 'devclean-e2e' };\n"
	if err := os.WriteFile(nodeFile, []byte(nodeContent), 0o644); err != nil {
		t.Fatalf("failed to create nodeFile: %v", err)
	}

	// 2b. Rust project: Cargo.toml + target/binary.bin
	rustApp := filepath.Join(workspace, "rust-project")
	rustTarget := filepath.Join(rustApp, "target")
	if err := os.MkdirAll(rustTarget, 0o755); err != nil {
		t.Fatalf("failed to create rust target dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rustApp, "Cargo.toml"), []byte("[package]\nname = \"e2e-rust\"\n"), 0o644); err != nil {
		t.Fatalf("failed to create Cargo.toml: %v", err)
	}
	rustFile := filepath.Join(rustTarget, "binary.bin")
	rustContent := []byte("binary-rust-payload-data-devclean-e2e")
	if err := os.WriteFile(rustFile, rustContent, 0o644); err != nil {
		t.Fatalf("failed to create rustFile: %v", err)
	}

	// 2c. Python project: pyproject.toml + .venv/lib/module.py
	pyApp := filepath.Join(workspace, "py-project")
	pyVenv := filepath.Join(pyApp, ".venv", "lib")
	if err := os.MkdirAll(pyVenv, 0o755); err != nil {
		t.Fatalf("failed to create pyVenv: %v", err)
	}
	pyprojectPath := filepath.Join(pyApp, "pyproject.toml")
	if err := os.WriteFile(pyprojectPath, []byte("[project]\nname = \"e2e-py\"\n"), 0o644); err != nil {
		t.Fatalf("failed to create pyproject.toml: %v", err)
	}
	// Definir mtime no passado para que isOrphanedVenv identifique como órfão
	pastTime := time.Now().Add(-60 * 24 * time.Hour)
	_ = os.Chtimes(pyprojectPath, pastTime, pastTime)

	pyFile := filepath.Join(pyVenv, "module.py")
	pyContent := "import sys\nprint('venv')\n"
	if err := os.WriteFile(pyFile, []byte(pyContent), 0o644); err != nil {
		t.Fatalf("failed to create pyFile: %v", err)
	}

	// Hashes SHA-256 pré-limpeza
	origNodeHash, err := fileSHA256(nodeFile)
	if err != nil {
		t.Fatalf("failed to compute nodeFile SHA256: %v", err)
	}
	origRustHash, err := fileSHA256(rustFile)
	if err != nil {
		t.Fatalf("failed to compute rustFile SHA256: %v", err)
	}
	origPyHash, err := fileSHA256(pyFile)
	if err != nil {
		t.Fatalf("failed to compute pyFile SHA256: %v", err)
	}

	// Passo 3: Configuração YAML
	cfgPath := filepath.Join(configDir, "config.yaml")
	cfgContent := fmt.Sprintf(`scan_paths:
  - %q
protected_patterns:
  - "*cacache*"
  - "*store*"
safety:
  trash_enabled: true
  trash_dir: %q
ecosystems:
  docker:
    enabled: false
  go:
    enabled: false
  dotnet:
    enabled: false
  java:
    enabled: false
  node:
    enabled: true
  rust:
    enabled: true
  python:
    enabled: true
    max_age_days: 0
`, workspace, trashDir)
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatalf("failed to write config.yaml: %v", err)
	}

	origConfigPath := configPath
	origVerbose := verbose
	origQuiet := quiet
	origProfile := profileName
	configPath = cfgPath
	verbose = false
	quiet = false
	profileName = ""
	defer func() {
		configPath = origConfigPath
		verbose = origVerbose
		quiet = origQuiet
		profileName = origProfile
	}()

	// Passo 4: Execução do Scan
	scanCmd := newScanCmd()
	var scanOut bytes.Buffer
	scanCmd.SetOut(&scanOut)
	if err := scanCmd.Execute(); err != nil {
		t.Fatalf("scan execution failed: %v", err)
	}

	// Passo 5: Execução do Clean com Trash
	cleanCmd := newCleanCmd()
	cleanCmd.SetArgs([]string{"--force", "--trash"})
	var cleanOut bytes.Buffer
	cleanCmd.SetOut(&cleanOut)
	if err := cleanCmd.Execute(); err != nil {
		t.Fatalf("clean --force --trash execution failed: %v", err)
	}

	// Validar que os arquivos foram removidos dos locais de origem
	if _, err := os.Stat(nodeFile); !os.IsNotExist(err) {
		t.Fatalf("expected nodeFile to not exist after clean, err: %v", err)
	}
	if _, err := os.Stat(rustFile); !os.IsNotExist(err) {
		t.Fatalf("expected rustFile to not exist after clean, err: %v", err)
	}
	if _, err := os.Stat(pyFile); !os.IsNotExist(err) {
		t.Fatalf("expected pyFile to not exist after clean, err: %v", err)
	}

	// Passo 6: Execução do Trash List
	listCmd := newTrashListCmd()
	var listOut bytes.Buffer
	listCmd.SetOut(&listOut)
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("trash list execution failed: %v", err)
	}

	// Passo 7: Execução do Trash Undo
	manifestPath := filepath.Join(trashDir, "manifest.json")
	manifest, err := trash.LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("failed to load trash manifest: %v", err)
	}
	if len(manifest.Batches) == 0 {
		t.Fatalf("expected at least 1 trash batch in manifest, got 0")
	}

	batchCount := len(manifest.Batches)
	for i := 0; i < batchCount; i++ {
		undoCmd := newTrashUndoCmd()
		var undoOut bytes.Buffer
		undoCmd.SetOut(&undoOut)
		if err := undoCmd.Execute(); err != nil {
			t.Fatalf("trash undo execution failed on iteration %d: %v", i, err)
		}
	}

	// Passo 8: Verificação Estrita de SHA256 pós-restauração
	restoredNodeHash, err := fileSHA256(nodeFile)
	if err != nil {
		t.Fatalf("failed to read restored nodeFile: %v", err)
	}
	if restoredNodeHash != origNodeHash {
		t.Fatalf("SHA256 mismatch for nodeFile: expected %s, got %s", origNodeHash, restoredNodeHash)
	}

	restoredRustHash, err := fileSHA256(rustFile)
	if err != nil {
		t.Fatalf("failed to read restored rustFile: %v", err)
	}
	if restoredRustHash != origRustHash {
		t.Fatalf("SHA256 mismatch for rustFile: expected %s, got %s", origRustHash, restoredRustHash)
	}

	restoredPyHash, err := fileSHA256(pyFile)
	if err != nil {
		t.Fatalf("failed to read restored pyFile: %v", err)
	}
	if restoredPyHash != origPyHash {
		t.Fatalf("SHA256 mismatch for pyFile: expected %s, got %s", origPyHash, restoredPyHash)
	}
}

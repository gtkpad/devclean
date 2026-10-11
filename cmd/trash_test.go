package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/trash"
)

func TestTrashCmd_Lifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	trashDir := filepath.Join(tmpDir, "trash")
	targetDir := filepath.Join(tmpDir, "workspace", "node_modules")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}
	testFile := filepath.Join(targetDir, "file.js")
	if err := os.WriteFile(testFile, []byte("console.log('payload');"), 0644); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgContent := fmt.Sprintf("safety:\n  trash_enabled: true\n  trash_dir: %q\n", trashDir)
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	// 1. Mover item para a lixeira via trash.Manager
	safetyCfg := config.SafetyConfig{TrashEnabled: true, TrashDir: trashDir}
	mgr := trash.New(safetyCfg)
	target := domain.Target{
		ID:        "target-node-mod",
		Path:      targetDir,
		SizeBytes: 24,
	}
	if err := mgr.Move(context.Background(), []domain.Target{target}, "node"); err != nil {
		t.Fatalf("trash.Move failed: %v", err)
	}

	// Verificar que targetDir foi movido
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Fatal("expected target file to be moved into trash")
	}

	// 2. Executar trash list
	listCmd := newTrashListCmd()
	var listBuf bytes.Buffer
	listCmd.SetOut(&listBuf)
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("trash list failed: %v", err)
	}

	// 3. Executar trash undo (último lote)
	undoCmd := newTrashUndoCmd()
	var undoBuf bytes.Buffer
	undoCmd.SetOut(&undoBuf)
	if err := undoCmd.Execute(); err != nil {
		t.Fatalf("trash undo failed: %v", err)
	}

	// Verificar restauração
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("expected target file to be restored by trash undo")
	}

	// 4. Executar trash purge com flag --all
	purgeCmd := newTrashPurgeCmd()
	purgeCmd.SetArgs([]string{"--all"})
	var purgeBuf bytes.Buffer
	purgeCmd.SetOut(&purgeBuf)
	if err := purgeCmd.Execute(); err != nil {
		t.Fatalf("trash purge --all failed: %v", err)
	}

	// 5. Executar trash purge sem --all (apenas expirados)
	purgeExpiredCmd := newTrashPurgeCmd()
	if err := purgeExpiredCmd.Execute(); err != nil {
		t.Fatalf("trash purge expired failed: %v", err)
	}

	// 6. Executar trash undo em lixeira vazia
	undoEmptyCmd := newTrashUndoCmd()
	if err := undoEmptyCmd.Execute(); err == nil {
		t.Error("expected error from trash undo on empty trash")
	}

	// 7. Executar trash undo com ID inexistente
	undoInvalidCmd := newTrashUndoCmd()
	undoInvalidCmd.SetArgs([]string{"--id", "nonexistent"})
	if err := undoInvalidCmd.Execute(); err == nil {
		t.Error("expected error from trash undo with nonexistent ID")
	}
}

func TestTrashCmd_UndoByID(t *testing.T) {
	tmpDir := t.TempDir()
	trashDir := filepath.Join(tmpDir, "trash")
	targetDir1 := filepath.Join(tmpDir, "workspace", "dir1")
	targetDir2 := filepath.Join(tmpDir, "workspace", "dir2")
	_ = os.MkdirAll(targetDir1, 0755)
	_ = os.MkdirAll(targetDir2, 0755)
	file1 := filepath.Join(targetDir1, "f1.txt")
	file2 := filepath.Join(targetDir2, "f2.txt")
	_ = os.WriteFile(file1, []byte("data1"), 0644)
	_ = os.WriteFile(file2, []byte("data2"), 0644)

	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgContent := fmt.Sprintf("safety:\n  trash_enabled: true\n  trash_dir: %q\n", trashDir)
	_ = os.WriteFile(cfgPath, []byte(cfgContent), 0644)

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	safetyCfg := config.SafetyConfig{TrashEnabled: true, TrashDir: trashDir}
	mgr := trash.New(safetyCfg)

	// Lote 1
	if err := mgr.Move(context.Background(), []domain.Target{{ID: "t1", Path: targetDir1, SizeBytes: 5}}, "node"); err != nil {
		t.Fatal(err)
	}
	// Lote 2
	if err := mgr.Move(context.Background(), []domain.Target{{ID: "t2", Path: targetDir2, SizeBytes: 5}}, "node"); err != nil {
		t.Fatal(err)
	}

	batches, err := mgr.List(context.Background())
	if err != nil || len(batches) < 2 {
		t.Fatalf("expected at least 2 batches, got %d (err: %v)", len(batches), err)
	}

	// Restaurar especificamente lote 1 por batch ID
	batchIDToRestore := batches[1].ID
	undoCmd := newTrashUndoCmd()
	undoCmd.SetArgs([]string{"--id", batchIDToRestore})
	if err := undoCmd.Execute(); err != nil {
		t.Fatalf("trash undo --id failed: %v", err)
	}

	if _, err := os.Stat(file1); os.IsNotExist(err) {
		t.Errorf("expected file1 in batch %s to be restored", batchIDToRestore)
	}
}

func TestTrashCmd_EmptyListAndPurgeExpired(t *testing.T) {
	tmpDir := t.TempDir()
	trashDir := filepath.Join(tmpDir, "trash")

	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgContent := fmt.Sprintf("safety:\n  trash_enabled: true\n  trash_dir: %q\n", trashDir)
	_ = os.WriteFile(cfgPath, []byte(cfgContent), 0644)

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	// Testar list quando vazia
	listCmd := newTrashListCmd()
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("trash list empty failed: %v", err)
	}

	// Testar purge sem --all (expirados)
	purgeCmd := newTrashPurgeCmd()
	if err := purgeCmd.Execute(); err != nil {
		t.Fatalf("trash purge expired failed: %v", err)
	}
}

func TestTrashCmd_ConfigErrors(t *testing.T) {
	origConfigPath := configPath
	configPath = "/non/existent/path/config.yaml"
	defer func() { configPath = origConfigPath }()

	if err := newTrashListCmd().Execute(); err == nil {
		t.Error("expected error from listCmd with invalid configPath")
	}
	if err := newTrashUndoCmd().Execute(); err == nil {
		t.Error("expected error from undoCmd with invalid configPath")
	}
	if err := newTrashPurgeCmd().Execute(); err == nil {
		t.Error("expected error from purgeCmd with invalid configPath")
	}
}

func TestTrashCmd_CorruptManifest(t *testing.T) {
	tmpDir := t.TempDir()
	trashDir := filepath.Join(tmpDir, "trash")
	_ = os.MkdirAll(trashDir, 0755)
	manifestPath := filepath.Join(trashDir, "manifest.json")
	_ = os.WriteFile(manifestPath, []byte("invalid json"), 0644)

	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgContent := fmt.Sprintf("safety:\n  trash_enabled: true\n  trash_dir: %q\n", trashDir)
	_ = os.WriteFile(cfgPath, []byte(cfgContent), 0644)

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	if err := newTrashListCmd().Execute(); err == nil {
		t.Error("expected error from listCmd with corrupt manifest")
	}
	if err := newTrashPurgeCmd().Execute(); err == nil {
		t.Error("expected error from purgeCmd with corrupt manifest")
	}
}


func TestTrashCmd_Metadata(t *testing.T) {
	cmd := newTrashCmd()
	if cmd.Use != "trash" {
		t.Errorf("expected Use 'trash', got %q", cmd.Use)
	}
	subCmds := cmd.Commands()
	if len(subCmds) != 3 {
		t.Errorf("expected 3 subcommands in trash, got %d", len(subCmds))
	}
}

func TestFormatAge(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Minute, "30m"},
		{5 * time.Hour, "5h"},
		{3 * 24 * time.Hour, "3d"},
		{14 * 24 * time.Hour, "2w"},
	}
	for _, tt := range tests {
		got := formatAge(tt.d)
		if got != tt.want {
			t.Errorf("formatAge(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

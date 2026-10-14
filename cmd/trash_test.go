package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	if len(subCmds) != 4 {
		t.Errorf("expected 4 subcommands in trash (undo, list, clean, purge), got %d", len(subCmds))
	}
}

func setupTestTrashEnv(t *testing.T, retentionDays int, maxSizeMB int) (string, *trash.Manager, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	trashDir := filepath.Join(tmpDir, "trash")
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgContent := fmt.Sprintf("safety:\n  trash_enabled: true\n  trash_dir: %q\n  retention_days: %d\n  max_size_mb: %d\n", trashDir, retentionDays, maxSizeMB)
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	origConfigPath := configPath
	configPath = cfgPath

	safetyCfg := config.SafetyConfig{
		TrashEnabled:  true,
		TrashDir:      trashDir,
		RetentionDays: retentionDays,
		MaxSizeMB:     maxSizeMB,
	}
	mgr := trash.New(safetyCfg)

	cleanup := func() {
		configPath = origConfigPath
	}
	return tmpDir, mgr, cleanup
}

func addTestBatch(t *testing.T, mgr *trash.Manager, tmpDir, name string, size int64, age time.Duration) string {
	t.Helper()
	targetDir := filepath.Join(tmpDir, "workspace", name)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, size)
	if err := os.WriteFile(filepath.Join(targetDir, "file.bin"), data, 0644); err != nil {
		t.Fatal(err)
	}

	if err := mgr.Move(context.Background(), []domain.Target{{ID: name, Path: targetDir, SizeBytes: size}}, "node"); err != nil {
		t.Fatalf("Move failed: %v", err)
	}

	m, err := trash.LoadManifest(mgr.ManifestPath())
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}
	// The newest batch was prepended to Batches or added
	batchIdx := len(m.Batches) - 1
	b := &m.Batches[batchIdx]
	bTime := time.Now().Add(-age)
	b.CreatedAt = bTime
	for i := range b.Entries {
		b.Entries[i].MovedAt = bTime
	}
	if err := trash.SaveManifest(mgr.ManifestPath(), m); err != nil {
		t.Fatalf("SaveManifest failed: %v", err)
	}
	return b.ID
}

func TestTrashCleanCmd_ExpiredOnlyDefault(t *testing.T) {
	tmpDir, mgr, cleanup := setupTestTrashEnv(t, 7, 100)
	defer cleanup()

	// 1 fresh batch (1 day old) and 1 expired batch (10 days old)
	addTestBatch(t, mgr, tmpDir, "fresh-batch", 1024, 24*time.Hour)
	addTestBatch(t, mgr, tmpDir, "expired-batch", 2048, 10*24*time.Hour)

	cleanCmd := newTrashCleanCmd()
	var out bytes.Buffer
	cleanCmd.SetOut(&out)

	if err := cleanCmd.Execute(); err != nil {
		t.Fatalf("trash clean failed: %v", err)
	}

	outStr := out.String()
	if !strings.Contains(outStr, "1 lote expirado removido") {
		t.Errorf("expected output to mention 1 lote expirado removido, got: %s", outStr)
	}

	// Verify only fresh batch remains
	manifest, err := trash.LoadManifest(mgr.ManifestPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Batches) != 1 {
		t.Fatalf("expected exactly 1 batch remaining, got %d", len(manifest.Batches))
	}
}

func TestTrashCleanCmd_AllFlag(t *testing.T) {
	tmpDir, mgr, cleanup := setupTestTrashEnv(t, 7, 100)
	defer cleanup()

	addTestBatch(t, mgr, tmpDir, "fresh-batch-1", 1024, 2*time.Hour)
	addTestBatch(t, mgr, tmpDir, "fresh-batch-2", 2048, 3*time.Hour)

	cleanCmd := newTrashCleanCmd()
	cleanCmd.SetArgs([]string{"--all"})
	var out bytes.Buffer
	cleanCmd.SetOut(&out)

	if err := cleanCmd.Execute(); err != nil {
		t.Fatalf("trash clean --all failed: %v", err)
	}

	outStr := out.String()
	if !strings.Contains(outStr, "Todos os lotes da lixeira foram removidos.") {
		t.Errorf("expected all purged message, got: %s", outStr)
	}

	manifest, err := trash.LoadManifest(mgr.ManifestPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Batches) != 0 {
		t.Fatalf("expected 0 batches remaining, got %d", len(manifest.Batches))
	}
}

func TestTrashCleanCmd_JsonOutput(t *testing.T) {
	tmpDir, mgr, cleanup := setupTestTrashEnv(t, 7, 100)
	defer cleanup()

	addTestBatch(t, mgr, tmpDir, "expired-batch", 4096, 12*24*time.Hour)

	// Clean expired in JSON format
	cleanCmd := newTrashCleanCmd()
	cleanCmd.SetArgs([]string{"--format", "json"})
	var out bytes.Buffer
	cleanCmd.SetOut(&out)

	if err := cleanCmd.Execute(); err != nil {
		t.Fatalf("trash clean --format json failed: %v", err)
	}

	var stats trash.CleanStats
	if err := json.Unmarshal(out.Bytes(), &stats); err != nil {
		t.Fatalf("unmarshaling clean stats JSON failed: %v, raw: %s", err, out.String())
	}
	if stats.BatchesPurged != 1 {
		t.Errorf("expected 1 batch purged, got %d", stats.BatchesPurged)
	}
	if stats.EntriesPurged != 1 {
		t.Errorf("expected 1 entry purged, got %d", stats.EntriesPurged)
	}
	if stats.BytesFreed != 4096 {
		t.Errorf("expected 4096 bytes freed, got %d", stats.BytesFreed)
	}

	// Purge all in JSON format
	addTestBatch(t, mgr, tmpDir, "fresh-batch", 1024, 1*time.Hour)
	purgeAllCmd := newTrashCleanCmd()
	purgeAllCmd.SetArgs([]string{"--all", "--format", "json"})
	var outAll bytes.Buffer
	purgeAllCmd.SetOut(&outAll)

	if err := purgeAllCmd.Execute(); err != nil {
		t.Fatalf("trash clean --all --format json failed: %v", err)
	}

	var allResp map[string]any
	if err := json.Unmarshal(outAll.Bytes(), &allResp); err != nil {
		t.Fatalf("unmarshaling clean --all JSON failed: %v, raw: %s", err, outAll.String())
	}
	if allResp["status"] != "all_purged" {
		t.Errorf("expected status 'all_purged', got %v", allResp["status"])
	}
}

func TestTrashPurgeCmd_AliasEquivalence(t *testing.T) {
	tmpDir, mgr, cleanup := setupTestTrashEnv(t, 7, 100)
	defer cleanup()

	addTestBatch(t, mgr, tmpDir, "fresh-batch", 1024, 1*time.Hour)
	addTestBatch(t, mgr, tmpDir, "expired-batch", 2048, 14*24*time.Hour)

	// 1. Alias purge default cleans expired
	purgeCmd := newTrashPurgeCmd()
	var out bytes.Buffer
	purgeCmd.SetOut(&out)
	if err := purgeCmd.Execute(); err != nil {
		t.Fatalf("trash purge alias failed: %v", err)
	}
	if !strings.Contains(out.String(), "1 lote expirado removido") {
		t.Errorf("expected purge to remove expired batch, got: %s", out.String())
	}

	// 2. Alias purge --all cleans everything
	purgeAllCmd := newTrashPurgeCmd()
	purgeAllCmd.SetArgs([]string{"--all"})
	var outAll bytes.Buffer
	purgeAllCmd.SetOut(&outAll)
	if err := purgeAllCmd.Execute(); err != nil {
		t.Fatalf("trash purge --all failed: %v", err)
	}
	if !strings.Contains(outAll.String(), "Todos os lotes da lixeira foram removidos.") {
		t.Errorf("expected all purged message, got: %s", outAll.String())
	}
}

func TestTrashListCmd_QuotaAndExpiredBadge(t *testing.T) {
	tmpDir, mgr, cleanup := setupTestTrashEnv(t, 7, 100)
	defer cleanup()

	addTestBatch(t, mgr, tmpDir, "fresh-batch", 1024, 2*24*time.Hour)
	addTestBatch(t, mgr, tmpDir, "expired-batch", 2048, 10*24*time.Hour)

	listCmd := newTrashListCmd()
	var out bytes.Buffer
	listCmd.SetOut(&out)

	if err := listCmd.Execute(); err != nil {
		t.Fatalf("trash list failed: %v", err)
	}

	outStr := out.String()
	// Check quota header
	if !strings.Contains(outStr, "Lixeira:") || !strings.Contains(outStr, "100.0 MB") || !strings.Contains(outStr, "% ocupado") {
		t.Errorf("expected quota header in list output, got:\n%s", outStr)
	}
	// Check [EXPIRADO] badge
	if !strings.Contains(outStr, "[EXPIRADO]") {
		t.Errorf("expected [EXPIRADO] badge in list output, got:\n%s", outStr)
	}
	// Check remaining retention
	if !strings.Contains(outStr, "tempo restante:") {
		t.Errorf("expected 'tempo restante:' in list output, got:\n%s", outStr)
	}
}

func TestTrashListCmd_Json(t *testing.T) {
	tmpDir, mgr, cleanup := setupTestTrashEnv(t, 7, 100)
	defer cleanup()

	addTestBatch(t, mgr, tmpDir, "fresh-batch", 1024, 2*24*time.Hour)
	addTestBatch(t, mgr, tmpDir, "expired-batch", 2048, 10*24*time.Hour)

	listCmd := newTrashListCmd()
	listCmd.SetArgs([]string{"--format", "json"})
	var out bytes.Buffer
	listCmd.SetOut(&out)

	if err := listCmd.Execute(); err != nil {
		t.Fatalf("trash list --format json failed: %v", err)
	}

	type jsonEntry struct {
		ID           string `json:"id"`
		SizeBytes    int64  `json:"size_bytes"`
		OriginalPath string `json:"original_path"`
	}
	type jsonBatch struct {
		ID                 string      `json:"id"`
		TotalSize          int64       `json:"total_size"`
		ItemCount          int         `json:"item_count"`
		Expired            bool        `json:"expired"`
		RemainingRetention string      `json:"remaining_retention"`
		Entries            []jsonEntry `json:"entries"`
	}
	type jsonOutput struct {
		Quota        trash.QuotaInfo `json:"quota"`
		TotalBatches int             `json:"total_batches"`
		Batches      []jsonBatch     `json:"batches"`
	}

	var parsed jsonOutput
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("unmarshaling json output failed: %v, raw: %s", err, out.String())
	}

	if parsed.TotalBatches != 2 {
		t.Errorf("expected TotalBatches 2, got %d", parsed.TotalBatches)
	}
	if parsed.Quota.MaxBytes != 100*1024*1024 {
		t.Errorf("expected MaxBytes %d, got %d", 100*1024*1024, parsed.Quota.MaxBytes)
	}

	foundExpired := false
	foundFresh := false
	for _, b := range parsed.Batches {
		if b.Expired {
			foundExpired = true
		} else {
			foundFresh = true
			if b.RemainingRetention == "" {
				t.Error("expected non-empty remaining_retention for fresh batch")
			}
		}
	}
	if !foundExpired || !foundFresh {
		t.Errorf("expected both expired and fresh batches in json, got expired=%v, fresh=%v", foundExpired, foundFresh)
	}
}

func TestTrash_InvalidFormat(t *testing.T) {
	tmpDir, _, cleanup := setupTestTrashEnv(t, 7, 100)
	defer cleanup()
	_ = tmpDir

	cleanCmd := newTrashCleanCmd()
	cleanCmd.SetArgs([]string{"--format", "yaml"})
	if err := cleanCmd.Execute(); err == nil {
		t.Error("expected error for invalid format in clean")
	}

	listCmd := newTrashListCmd()
	listCmd.SetArgs([]string{"--format", "yaml"})
	if err := listCmd.Execute(); err == nil {
		t.Error("expected error for invalid format in list")
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


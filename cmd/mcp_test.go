package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/history"
)

func TestMCP_Adapters(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. historyAdapter tests
	ha := &historyAdapter{dataDir: tmpDir}
	entry := history.Entry{
		Timestamp:  time.Now(),
		Targets:    2,
		FreedBytes: 2048,
		Ecosystems: []string{"node", "go"},
		Duration:   500 * time.Millisecond,
		Mode:       "force",
	}
	if err := history.Append(tmpDir, entry); err != nil {
		t.Fatalf("history.Append failed: %v", err)
	}

	entries, err := ha.Read()
	if err != nil {
		t.Fatalf("ha.Read() returned error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("ha.Read() expected 1 entry, got %d", len(entries))
	}

	filtered, err := ha.ReadFiltered(history.FilterOpts{Ecosystem: "node"})
	if err != nil {
		t.Fatalf("ha.ReadFiltered() returned error: %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("ha.ReadFiltered(node) expected 1 entry, got %d", len(filtered))
	}

	filteredEmpty, err := ha.ReadFiltered(history.FilterOpts{Ecosystem: "rust"})
	if err != nil {
		t.Fatalf("ha.ReadFiltered() error: %v", err)
	}
	if len(filteredEmpty) != 0 {
		t.Fatalf("ha.ReadFiltered(rust) expected 0 entries, got %d", len(filteredEmpty))
	}

	// 2. configAdapter tests
	cfg := &config.Config{ScanPaths: []string{"/test/scan/path"}}
	cfgDir := filepath.Join(tmpDir, "devclean-config")
	ca := &configAdapter{cfg: cfg, cfgDir: cfgDir}

	if ca.Config() == nil || len(ca.Config().ScanPaths) != 1 {
		t.Errorf("ca.Config() expected ScanPaths with 1 item")
	}
	if ca.ConfigDir() != cfgDir {
		t.Errorf("ca.ConfigDir() expected %q, got %q", cfgDir, ca.ConfigDir())
	}
}

func TestMCP_CmdMetadataAndConfigError(t *testing.T) {
	cmd := newMCPCmd()
	if cmd.Use != "mcp" {
		t.Errorf("expected cmd.Use = 'mcp', got %q", cmd.Use)
	}
	if cmd.Short == "" || cmd.Long == "" {
		t.Error("expected non-empty Short and Long descriptions")
	}

	// Test config load error on execution
	origConfigPath := configPath
	configPath = "/invalid/path/devclean.yaml"
	defer func() { configPath = origConfigPath }()

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error with nonexistent configPath, got nil")
	}
	if !strings.Contains(err.Error(), "loading config") {
		t.Errorf("expected 'loading config' error, got %v", err)
	}
}

func TestMCPCmd_ExecutionWithClosedStdin(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	_ = os.WriteFile(cfgPath, []byte("scan_paths: []\n"), 0644)

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_ = w.Close()

	oldStdin := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		_ = r.Close()
	}()

	cmd := newMCPCmd()
	_ = cmd.Execute()
}


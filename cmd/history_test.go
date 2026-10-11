package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/history"
)

func TestParseDuration_Days(t *testing.T) {
	d, err := parseDuration("7d")
	if err != nil {
		t.Fatalf("parseDuration(7d) failed: %v", err)
	}
	expected := 7 * 24 * time.Hour
	if d != expected {
		t.Errorf("expected %v, got %v", expected, d)
	}
}

func TestParseDuration_Hours(t *testing.T) {
	d, err := parseDuration("2h")
	if err != nil {
		t.Fatalf("parseDuration(2h) failed: %v", err)
	}
	if d != 2*time.Hour {
		t.Errorf("expected 2h, got %v", d)
	}
}

func TestParseDuration_Invalid(t *testing.T) {
	_, err := parseDuration("abc")
	if err == nil {
		t.Error("expected error for invalid duration")
	}
}

func TestParseDuration_InvalidNumberWithD(t *testing.T) {
	_, err := parseDuration("xyzd")
	if err == nil {
		t.Error("expected error for non-numeric days prefix")
	}
}


func TestHistoryCmd_ExecutionAndFlags(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	dataDir := filepath.Join(tmpDir, ".local", "share", "devclean")

	_ = history.Append(dataDir, history.Entry{
		Timestamp:  time.Now(),
		Targets:    3,
		FreedBytes: 1048576,
		Ecosystems: []string{"node", "go"},
		Duration:   time.Second,
		Mode:       "force",
	})
	_ = history.Append(dataDir, history.Entry{
		Timestamp:  time.Now().Add(-48 * time.Hour),
		Targets:    1,
		FreedBytes: 524288,
		Ecosystems: []string{"rust"},
		Duration:   2 * time.Second,
		Mode:       "auto",
	})

	cfgPath := filepath.Join(tmpDir, "config.yaml")
	_ = os.WriteFile(cfgPath, []byte("history:\n  retention_days: 30\n"), 0644)

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	// 1. Invocação com --json
	histCmdJSON := newHistoryCmd()
	histCmdJSON.SetArgs([]string{"--json"})
	if err := histCmdJSON.Execute(); err != nil {
		t.Fatalf("history --json failed: %v", err)
	}

	// 2. Invocação com filtros --since e --ecosystem
	histCmdFiltered := newHistoryCmd()
	histCmdFiltered.SetArgs([]string{"--since", "7d", "--ecosystem", "node"})
	if err := histCmdFiltered.Execute(); err != nil {
		t.Fatalf("history --since 7d --ecosystem node failed: %v", err)
	}

	// 3. Invocação com --since inválido
	histCmdInvalid := newHistoryCmd()
	histCmdInvalid.SetArgs([]string{"--since", "invalid-duration"})
	if err := histCmdInvalid.Execute(); err == nil {
		t.Error("expected error for invalid --since flag, got nil")
	}
}

func TestHistoryCmd_EmptyHistory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfgPath := filepath.Join(tmpDir, "config.yaml")
	_ = os.WriteFile(cfgPath, []byte("history:\n  retention_days: 30\n"), 0644)

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	histCmd := newHistoryCmd()
	if err := histCmd.Execute(); err != nil {
		t.Fatalf("history on empty directory failed: %v", err)
	}
}

func TestHistoryCmd_InvalidConfig(t *testing.T) {
	origConfigPath := configPath
	configPath = "/invalid/path.yaml"
	defer func() { configPath = origConfigPath }()

	cmd := newHistoryCmd()
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid config")
	}
}



func TestPrintHistoryTable(t *testing.T) {
	entries := []history.Entry{
		{
			Timestamp:  time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC),
			Targets:    5,
			FreedBytes: 2048000,
			Ecosystems: []string{"node", "docker"},
			Duration:   1500 * time.Millisecond,
			Mode:       "force",
		},
	}

	// Redirecionar stdout temporariamente para capturar tabela
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	printHistoryTable(entries)

	_ = w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, "Date") || !strings.Contains(out, "Targets") || !strings.Contains(out, "node, docker") {
		t.Errorf("printHistoryTable output unexpected: %s", out)
	}
}

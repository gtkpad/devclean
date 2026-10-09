package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/history"
)

func TestPrintStats_Aggregation(t *testing.T) {
	entries := []history.Entry{
		{
			Timestamp:  time.Now().Add(-24 * time.Hour),
			Targets:    5,
			FreedBytes: 1024 * 1024 * 100, // 100MB
			Ecosystems: []string{"node"},
			Duration:   2 * time.Second,
			Mode:       "force",
		},
		{
			Timestamp:  time.Now(),
			Targets:    3,
			FreedBytes: 1024 * 1024 * 50, // 50MB
			Ecosystems: []string{"go", "rust"},
			Duration:   1 * time.Second,
			Mode:       "auto",
		},
		{
			Timestamp:  time.Now(),
			Targets:    1,
			FreedBytes: 1, // tiny bytes to trigger barWidth < 1 && freedBytes > 0
			Ecosystems: []string{"python"},
			Duration:   time.Millisecond,
			Mode:       "force",
		},
	}


	// Verify aggregation doesn't panic
	printStats(entries)
}

func TestPrintStats_SingleEntry(t *testing.T) {
	entries := []history.Entry{
		{
			Timestamp:  time.Now(),
			Targets:    1,
			FreedBytes: 512,
			Ecosystems: []string{"python"},
			Duration:   100 * time.Millisecond,
			Mode:       "tui",
		},
	}
	printStats(entries)
}

func TestStatsCmd_Execution(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	dataDir := filepath.Join(tmpDir, ".local", "share", "devclean")

	// 1. Executar quando vazio
	cmd := newStatsCmd()
	if err := cmd.Execute(); err != nil {
		t.Fatalf("stats empty failed: %v", err)
	}

	// 2. Gravar entradas e executar novamente
	_ = history.Append(dataDir, history.Entry{
		Timestamp:  time.Now(),
		Targets:    2,
		FreedBytes: 1024 * 1024,
		Ecosystems: []string{"node"},
		Duration:   time.Second,
		Mode:       "force",
	})

	cmdWithData := newStatsCmd()
	if err := cmdWithData.Execute(); err != nil {
		t.Fatalf("stats with data failed: %v", err)
	}

	// 3. Simular erro ao ler history
	invalidFile := filepath.Join(dataDir, "history.jsonl")
	_ = os.WriteFile(invalidFile, []byte("{invalid-json"), 0644)
	// Nota: history.Read pode retornar erro com json corrompido
	_ = cmdWithData.Execute()
}

package cmd

import (
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

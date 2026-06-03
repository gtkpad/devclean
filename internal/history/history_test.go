package history

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAppend_CreatesFileAndWritesEntry(t *testing.T) {
	dir := t.TempDir()
	entry := Entry{
		Timestamp:  time.Now(),
		Targets:    5,
		FreedBytes: 1024 * 1024,
		Ecosystems: []string{"node"},
		Duration:   2 * time.Second,
		Mode:       "force",
	}

	if err := Append(dir, entry); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	entries, err := Read(dir)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Targets != 5 {
		t.Errorf("expected targets=5, got %d", entries[0].Targets)
	}
}

func TestRead_MultipleEntries(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 3; i++ {
		Append(dir, Entry{
			Timestamp:  time.Now(),
			Targets:    i,
			Ecosystems: []string{"go"},
			Mode:       "auto",
		})
	}

	entries, err := Read(dir)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
}

func TestRead_NonExistentFile(t *testing.T) {
	dir := t.TempDir()
	entries, err := Read(filepath.Join(dir, "nope"))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(entries))
	}
}

func TestReadFiltered_BySince(t *testing.T) {
	dir := t.TempDir()
	old := Entry{Timestamp: time.Now().Add(-48 * time.Hour), Targets: 1, Ecosystems: []string{"node"}, Mode: "auto"}
	recent := Entry{Timestamp: time.Now(), Targets: 2, Ecosystems: []string{"go"}, Mode: "force"}
	Append(dir, old)
	Append(dir, recent)

	filtered, err := ReadFiltered(dir, FilterOpts{Since: time.Now().Add(-24 * time.Hour)})
	if err != nil {
		t.Fatalf("ReadFiltered failed: %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(filtered))
	}
	if filtered[0].Targets != 2 {
		t.Errorf("expected recent entry (targets=2), got targets=%d", filtered[0].Targets)
	}
}

func TestReadFiltered_ByEcosystem(t *testing.T) {
	dir := t.TempDir()
	Append(dir, Entry{Timestamp: time.Now(), Ecosystems: []string{"node"}, Mode: "auto"})
	Append(dir, Entry{Timestamp: time.Now(), Ecosystems: []string{"go", "rust"}, Mode: "auto"})
	Append(dir, Entry{Timestamp: time.Now(), Ecosystems: []string{"python"}, Mode: "auto"})

	filtered, err := ReadFiltered(dir, FilterOpts{Ecosystem: "go"})
	if err != nil {
		t.Fatalf("ReadFiltered failed: %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(filtered))
	}
}

func TestTruncate_RemovesOldEntries(t *testing.T) {
	dir := t.TempDir()
	old := Entry{Timestamp: time.Now().Add(-100 * 24 * time.Hour), Ecosystems: []string{"node"}, Mode: "auto"}
	recent := Entry{Timestamp: time.Now(), Ecosystems: []string{"go"}, Mode: "auto"}
	Append(dir, old)
	Append(dir, recent)

	if err := Truncate(dir, 90*24*time.Hour); err != nil {
		t.Fatalf("Truncate failed: %v", err)
	}

	entries, err := Read(dir)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after truncate, got %d", len(entries))
	}
}

func TestTruncate_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	if err := Truncate(dir, 90*24*time.Hour); err != nil {
		t.Fatalf("Truncate on empty should not error: %v", err)
	}
}

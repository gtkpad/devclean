package scheduler

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecordRun_and_ReadLastRun(t *testing.T) {
	dir := t.TempDir()
	ts := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	input := RunResult{
		Timestamp:    ts,
		TargetsCount: 5,
		BytesFreed:   1024 * 1024 * 100,
		Duration:     "2.3s",
	}

	if err := RecordRun(dir, input); err != nil {
		t.Fatalf("RecordRun() error: %v", err)
	}

	got, err := ReadLastRun(dir)
	if err != nil {
		t.Fatalf("ReadLastRun() error: %v", err)
	}
	if got == nil {
		t.Fatal("ReadLastRun() returned nil")
	}
	if !got.Timestamp.Equal(input.Timestamp) {
		t.Errorf("Timestamp = %v, want %v", got.Timestamp, input.Timestamp)
	}
	if got.TargetsCount != input.TargetsCount {
		t.Errorf("TargetsCount = %d, want %d", got.TargetsCount, input.TargetsCount)
	}
	if got.BytesFreed != input.BytesFreed {
		t.Errorf("BytesFreed = %d, want %d", got.BytesFreed, input.BytesFreed)
	}
	if got.Duration != input.Duration {
		t.Errorf("Duration = %q, want %q", got.Duration, input.Duration)
	}
}

func TestReadLastRun_FileNotExist(t *testing.T) {
	dir := t.TempDir()
	got, err := ReadLastRun(dir)
	if err != nil {
		t.Fatalf("ReadLastRun() error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestRecordRun_CreatesDirectory(t *testing.T) {
	base := t.TempDir()
	nested := filepath.Join(base, "sub", "dir")

	input := RunResult{
		Timestamp:    time.Now(),
		TargetsCount: 1,
		BytesFreed:   512,
		Duration:     "0.1s",
	}

	if err := RecordRun(nested, input); err != nil {
		t.Fatalf("RecordRun() error: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(filepath.Join(nested, "last-run.json")); err != nil {
		t.Errorf("last-run.json not created: %v", err)
	}
}

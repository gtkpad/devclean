package scheduler

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRotateIfNeeded_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "schedule.log")

	err := RotateIfNeeded(logPath, 1024, 3)
	if err != nil {
		t.Fatalf("expected nil error for nonexistent file, got: %v", err)
	}
}

func TestRotateIfNeeded_BelowThreshold(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "schedule.log")

	data := []byte("small log file content\n")
	if err := os.WriteFile(logPath, data, 0644); err != nil {
		t.Fatalf("writing test log: %v", err)
	}

	err := RotateIfNeeded(logPath, 1024, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Active file must still exist and no .1.log created
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("active log file should still exist: %v", err)
	}
	firstBackup := filepath.Join(tmpDir, "schedule.1.log")
	if _, err := os.Stat(firstBackup); !os.IsNotExist(err) {
		t.Errorf("backup file %s should not exist", firstBackup)
	}
}

func TestRotateIfNeeded_SingleRotation(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "schedule.log")

	content := []byte("execution log line 1\nexecution log line 2\n")
	if err := os.WriteFile(logPath, content, 0644); err != nil {
		t.Fatalf("writing test log: %v", err)
	}

	// Threshold smaller than content size
	err := RotateIfNeeded(logPath, int64(len(content)-5), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Active file must have been moved to schedule.1.log
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Error("active log file should have been moved")
	}

	firstBackup := filepath.Join(tmpDir, "schedule.1.log")
	got, err := os.ReadFile(firstBackup)
	if err != nil {
		t.Fatalf("reading backup log: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("backup content mismatch: got %q, want %q", got, content)
	}
}

func TestRotateIfNeeded_CircularMaxBackups(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "schedule.log")

	maxBackups := 3
	maxBytes := int64(10)

	// Simulate 5 successive cycles
	for i := 1; i <= 5; i++ {
		content := fmt.Sprintf("cycle-%d-payload-exceeding-threshold\n", i)
		if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
			t.Fatalf("writing cycle %d log: %v", i, err)
		}

		if err := RotateIfNeeded(logPath, maxBytes, maxBackups); err != nil {
			t.Fatalf("rotation at cycle %d failed: %v", i, err)
		}
	}

	// After 5 cycles:
	// schedule.1.log should contain cycle 5
	// schedule.2.log should contain cycle 4
	// schedule.3.log should contain cycle 3
	// cycle 1 and cycle 2 should have been purged
	// schedule.4.log should NOT exist

	b1, err := os.ReadFile(filepath.Join(tmpDir, "schedule.1.log"))
	if err != nil || !bytes.Contains(b1, []byte("cycle-5")) {
		t.Errorf("expected cycle-5 in schedule.1.log, got %q (err: %v)", b1, err)
	}

	b2, err := os.ReadFile(filepath.Join(tmpDir, "schedule.2.log"))
	if err != nil || !bytes.Contains(b2, []byte("cycle-4")) {
		t.Errorf("expected cycle-4 in schedule.2.log, got %q (err: %v)", b2, err)
	}

	b3, err := os.ReadFile(filepath.Join(tmpDir, "schedule.3.log"))
	if err != nil || !bytes.Contains(b3, []byte("cycle-3")) {
		t.Errorf("expected cycle-3 in schedule.3.log, got %q (err: %v)", b3, err)
	}

	b4Path := filepath.Join(tmpDir, "schedule.4.log")
	if _, err := os.Stat(b4Path); !os.IsNotExist(err) {
		t.Errorf("schedule.4.log should not exist, circular cap exceeded")
	}
}

func TestRotateIfNeeded_SymlinkRemoval(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "schedule.log")
	targetPath := filepath.Join(tmpDir, "secret_target.txt")

	if err := os.WriteFile(targetPath, []byte("confidential system data"), 0600); err != nil {
		t.Fatalf("writing target file: %v", err)
	}

	if err := os.Symlink(targetPath, logPath); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	// RotateIfNeeded must detect symlink and remove it without touching target
	err := RotateIfNeeded(logPath, 10, 3)
	if err != nil {
		t.Fatalf("unexpected error during symlink check: %v", err)
	}

	if _, err := os.Lstat(logPath); !os.IsNotExist(err) {
		t.Error("expected symlink schedule.log to be removed")
	}

	// Secret target must remain untouched
	if _, err := os.Stat(targetPath); err != nil {
		t.Errorf("target file should not have been removed: %v", err)
	}
}

func TestRotateIfNeeded_DefaultParameters(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "schedule.log")

	// Calling with 0 values should apply defaults
	if err := os.WriteFile(logPath, []byte("short"), 0644); err != nil {
		t.Fatalf("writing log: %v", err)
	}

	err := RotateIfNeeded(logPath, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error with defaults: %v", err)
	}

	// Below default 10MB -> should not rotate
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("log should not be rotated below 10MB default")
	}
}

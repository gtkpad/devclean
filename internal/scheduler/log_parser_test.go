package scheduler

import (
	"strings"
	"testing"
	"time"
)

func TestParseLogRuns_SingleOKRun(t *testing.T) {
	logContent := `=== [START] 2026-10-13 14:00:00 | version=v2.2 | args=[--preset safe] ===
[INFO] Scanning development artifacts...
[INFO] Deleting cache in /Users/gabriel/.cache/uv...
Would delete: /Users/gabriel/.cache/uv/archive (42.0 MB)
=== [END] 2026-10-13 14:00:03 | status=OK | targets=14 | freed=145.2 MB | duration=3.1s ===
`

	runs, err := ParseLogRuns(strings.NewReader(logContent))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}

	run := runs[0]
	expectedStart, _ := time.Parse("2006-01-02 15:04:05", "2026-10-13 14:00:00")
	expectedEnd, _ := time.Parse("2006-01-02 15:04:05", "2026-10-13 14:00:03")

	if !run.StartTime.Equal(expectedStart) {
		t.Errorf("StartTime = %v, want %v", run.StartTime, expectedStart)
	}
	if !run.EndTime.Equal(expectedEnd) {
		t.Errorf("EndTime = %v, want %v", run.EndTime, expectedEnd)
	}
	if run.Version != "v2.2" {
		t.Errorf("Version = %q, want 'v2.2'", run.Version)
	}
	if len(run.Args) != 2 || run.Args[0] != "--preset" || run.Args[1] != "safe" {
		t.Errorf("Args = %v, want [--preset safe]", run.Args)
	}
	if run.Status != "OK" {
		t.Errorf("Status = %q, want 'OK'", run.Status)
	}
	if run.TargetsCount != 14 {
		t.Errorf("TargetsCount = %d, want 14", run.TargetsCount)
	}
	if run.FreedHuman != "145.2 MB" {
		t.Errorf("FreedHuman = %q, want '145.2 MB'", run.FreedHuman)
	}
	if run.BytesFreed <= 0 {
		t.Errorf("BytesFreed = %d, want > 0", run.BytesFreed)
	}
	if run.Duration != "3.1s" {
		t.Errorf("Duration = %q, want '3.1s'", run.Duration)
	}
	if !strings.Contains(run.Body, "Scanning development artifacts") {
		t.Errorf("Body missing scan info: %q", run.Body)
	}
	if len(run.ErrorLines) != 0 {
		t.Errorf("expected 0 error lines, got: %v", run.ErrorLines)
	}
}

func TestParseLogRuns_PartialRunWithErrors(t *testing.T) {
	logContent := `=== [START] 2026-10-13 15:00:00 | version=v2.2 | args=[] ===
[INFO] Starting cleanup...
Deleting: /tmp/cleanable/artifact
Error: permission denied for /protected/dir
Warning: skipping item
[ERROR] unable to delete locked resource
=== [END] 2026-10-13 15:00:02 | status=PARTIAL | targets=5 | freed=10.5 KB | duration=2.0s ===
`

	runs, err := ParseLogRuns(strings.NewReader(logContent))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}

	run := runs[0]
	if run.Status != "PARTIAL" {
		t.Errorf("Status = %q, want 'PARTIAL'", run.Status)
	}
	if run.TargetsCount != 5 {
		t.Errorf("TargetsCount = %d, want 5", run.TargetsCount)
	}
	if run.FreedHuman != "10.5 KB" {
		t.Errorf("FreedHuman = %q, want '10.5 KB'", run.FreedHuman)
	}
	expectedBytes := int64(float64(10.5) * 1024)
	if run.BytesFreed != expectedBytes {
		t.Errorf("BytesFreed = %d, want %d", run.BytesFreed, expectedBytes)
	}
	if len(run.ErrorLines) != 2 {
		t.Fatalf("expected 2 error lines, got %d: %v", len(run.ErrorLines), run.ErrorLines)
	}
	if !strings.Contains(run.ErrorLines[0], "permission denied") {
		t.Errorf("error line 0 = %q, want permission denied", run.ErrorLines[0])
	}
	if !strings.Contains(run.ErrorLines[1], "[ERROR]") {
		t.Errorf("error line 1 = %q, want [ERROR]", run.ErrorLines[1])
	}
}

func TestParseLogRuns_FailedRun(t *testing.T) {
	logContent := `=== [START] 2026-10-13 16:00:00 | version=v2.2 | args=[--preset aggressive] ===
fatal error: out of memory
=== [END] 2026-10-13 16:00:01 | status=FAIL | targets=0 | freed=0 B | duration=0.5s ===
`

	runs, err := ParseLogRuns(strings.NewReader(logContent))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}

	run := runs[0]
	if run.Status != "FAIL" {
		t.Errorf("Status = %q, want 'FAIL'", run.Status)
	}
	if run.TargetsCount != 0 {
		t.Errorf("TargetsCount = %d, want 0", run.TargetsCount)
	}
	if run.BytesFreed != 0 {
		t.Errorf("BytesFreed = %d, want 0", run.BytesFreed)
	}
	if run.FreedHuman != "0 B" {
		t.Errorf("FreedHuman = %q, want '0 B'", run.FreedHuman)
	}
	if len(run.ErrorLines) != 1 {
		t.Errorf("expected 1 error line, got %d", len(run.ErrorLines))
	}
}

func TestParseLogRuns_MultipleRuns(t *testing.T) {
	logContent := `some garbage header before logs
=== [START] 2026-10-13 10:00:00 | version=v2.2 | args=[--preset safe] ===
Clean line 1
=== [END] 2026-10-13 10:00:01 | status=OK | targets=2 | freed=1.5 GB | duration=1.0s ===
unrelated log text between runs
=== [START] 2026-10-13 11:00:00 | version=v2.2 | args=[--preset standard] ===
Clean line 2
=== [END] 2026-10-13 11:00:02 | status=PARTIAL | targets=3 | freed=500.0 KB | duration=2.0s ===
`

	runs, err := ParseLogRuns(strings.NewReader(logContent))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}

	if runs[0].Status != "OK" || runs[0].TargetsCount != 2 || runs[0].FreedHuman != "1.5 GB" {
		t.Errorf("run 0 mismatch: %+v", runs[0])
	}
	if runs[1].Status != "PARTIAL" || runs[1].TargetsCount != 3 || runs[1].FreedHuman != "500.0 KB" {
		t.Errorf("run 1 mismatch: %+v", runs[1])
	}
}

func TestParseLogRuns_IncompleteRun(t *testing.T) {
	logContent := `=== [START] 2026-10-13 12:00:00 | version=v2.2 | args=[--auto] ===
Starting process that gets killed by SIGKILL
`

	runs, err := ParseLogRuns(strings.NewReader(logContent))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runs) != 1 {
		t.Fatalf("expected 1 incomplete run, got %d", len(runs))
	}

	run := runs[0]
	if run.Status != "FAIL" {
		t.Errorf("expected incomplete run status 'FAIL', got %q", run.Status)
	}
	if run.Version != "v2.2" {
		t.Errorf("Version = %q, want 'v2.2'", run.Version)
	}
	if !strings.Contains(run.Body, "SIGKILL") {
		t.Errorf("Body missing SIGKILL: %q", run.Body)
	}
}

func TestParseLogRuns_EmptyAndCorrupted(t *testing.T) {
	runs, err := ParseLogRuns(strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error on empty: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("expected 0 runs on empty, got %d", len(runs))
	}

	corrupted := `just some random text
without any start or end markers
`
	runs, err = ParseLogRuns(strings.NewReader(corrupted))
	if err != nil {
		t.Fatalf("unexpected error on corrupted: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("expected 0 runs on corrupted, got %d", len(runs))
	}
}

func TestParseLogRuns_LongLines(t *testing.T) {
	longLine := strings.Repeat("A", 128*1024) // 128 KB single line
	logContent := "=== [START] 2026-10-13 13:00:00 | version=v2.2 | args=[] ===\n" +
		longLine + "\n" +
		"=== [END] 2026-10-13 13:00:01 | status=OK | targets=1 | freed=100 B | duration=1.0s ===\n"

	runs, err := ParseLogRuns(strings.NewReader(logContent))
	if err != nil {
		t.Fatalf("unexpected error with long line: %v", err)
	}

	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].TargetsCount != 1 {
		t.Errorf("TargetsCount = %d, want 1", runs[0].TargetsCount)
	}
}

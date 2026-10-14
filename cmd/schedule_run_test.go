package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/scheduler"
)

func TestScheduleRunCmd_ExecutionCycle(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmpDir, ".local", "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	// Create config with scheduler args
	resolver := platform.New()
	paths := resolver.Resolve()
	if err := os.MkdirAll(paths.ConfigDir, 0755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	cfgContent := "scheduler:\n  args:\n    - \"--preset\"\n    - \"safe\"\n"
	if err := os.WriteFile(filepath.Join(paths.ConfigDir, "config.yaml"), []byte(cfgContent), 0644); err != nil {
		t.Fatalf("writing config.yaml: %v", err)
	}

	// Mock clean subprocess runner
	oldRunner := runCleanSubprocess
	defer func() { runCleanSubprocess = oldRunner }()

	var capturedArgs []string
	runCleanSubprocess = func(ctx context.Context, configDir string, args []string, logWriter io.Writer) cleanExecutionResult {
		capturedArgs = args
		_, _ = logWriter.Write([]byte("Scanning development caches...\n"))
		_, _ = logWriter.Write([]byte("✨ Clean Summary\n\nEspaço Recuperado:  145.2 MB\nAlvos Processados:  14 (Lixeira: 0, Permanente: 14)\n"))
		return cleanExecutionResult{
			ExitCode:     0,
			TargetsCount: 14,
			BytesFreed:   152253235,
			FreedHuman:   "145.2 MB",
			Status:       "OK",
		}
	}

	cmd := newScheduleCmd()
	cmd.SetArgs([]string{"run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("schedule run failed: %v", err)
	}

	// 1. Verify scheduler args were passed
	if len(capturedArgs) != 2 || capturedArgs[0] != "--preset" || capturedArgs[1] != "safe" {
		t.Errorf("expected [--preset safe] passed to runner, got: %v", capturedArgs)
	}

	// 2. Verify schedule.log contents
	logPath := filepath.Join(paths.LogDir, "schedule.log")
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading schedule.log: %v", err)
	}
	logStr := string(logBytes)

	if !strings.Contains(logStr, "=== [START]") {
		t.Errorf("schedule.log missing [START] marker:\n%s", logStr)
	}
	if !strings.Contains(logStr, "version=") {
		t.Errorf("schedule.log missing version:\n%s", logStr)
	}
	if !strings.Contains(logStr, "args=[--preset safe]") {
		t.Errorf("schedule.log missing args in header:\n%s", logStr)
	}
	if !strings.Contains(logStr, "Scanning development caches...") {
		t.Errorf("schedule.log missing subprocess log lines:\n%s", logStr)
	}
	if !strings.Contains(logStr, "=== [END]") {
		t.Errorf("schedule.log missing [END] marker:\n%s", logStr)
	}
	if !strings.Contains(logStr, "status=OK") {
		t.Errorf("schedule.log missing status=OK:\n%s", logStr)
	}
	if !strings.Contains(logStr, "targets=14") {
		t.Errorf("schedule.log missing targets=14:\n%s", logStr)
	}
	if !strings.Contains(logStr, "freed=145.2 MB") {
		t.Errorf("schedule.log missing freed=145.2 MB:\n%s", logStr)
	}

	// 3. Verify last-run.json was written
	lastRun, err := scheduler.ReadLastRun(paths.ConfigDir)
	if err != nil {
		t.Fatalf("reading last-run.json: %v", err)
	}
	if lastRun == nil {
		t.Fatal("expected last-run.json to be created")
	}
	if lastRun.TargetsCount != 14 {
		t.Errorf("lastRun.TargetsCount = %d, want 14", lastRun.TargetsCount)
	}
	if lastRun.BytesFreed != 152253235 {
		t.Errorf("lastRun.BytesFreed = %d, want 152253235", lastRun.BytesFreed)
	}
}

func TestScheduleRunCmd_ErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmpDir, ".local", "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	resolver := platform.New()
	paths := resolver.Resolve()

	oldRunner := runCleanSubprocess
	defer func() { runCleanSubprocess = oldRunner }()

	runCleanSubprocess = func(ctx context.Context, configDir string, args []string, logWriter io.Writer) cleanExecutionResult {
		_, _ = logWriter.Write([]byte("fatal: engine crashed with out of memory\n"))
		return cleanExecutionResult{
			ExitCode:     1,
			TargetsCount: 0,
			BytesFreed:   0,
			FreedHuman:   "0 B",
			Status:       "FAIL",
			Err:          errors.New("exit status 1"),
		}
	}

	cmd := newScheduleCmd()
	cmd.SetArgs([]string{"run"})
	// Runner should not crash/break, it logs status=FAIL and returns gracefully
	if err := cmd.Execute(); err != nil {
		t.Fatalf("schedule run should handle failure gracefully, got error: %v", err)
	}

	logPath := filepath.Join(paths.LogDir, "schedule.log")
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading schedule.log: %v", err)
	}
	logStr := string(logBytes)

	if !strings.Contains(logStr, "status=FAIL") {
		t.Errorf("expected status=FAIL in log footer, got:\n%s", logStr)
	}
	if !strings.Contains(logStr, "targets=0") {
		t.Errorf("expected targets=0 in log footer, got:\n%s", logStr)
	}
}

func TestScheduleRunCmd_PartialStatus(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmpDir, ".local", "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	resolver := platform.New()
	paths := resolver.Resolve()

	oldRunner := runCleanSubprocess
	defer func() { runCleanSubprocess = oldRunner }()

	runCleanSubprocess = func(ctx context.Context, configDir string, args []string, logWriter io.Writer) cleanExecutionResult {
		_, _ = logWriter.Write([]byte("Deleting: /tmp/normal\nError: permission denied for /protected/root\n"))
		return cleanExecutionResult{
			ExitCode:     0,
			TargetsCount: 1,
			BytesFreed:   1024,
			FreedHuman:   "1.0 KB",
			Status:       "PARTIAL",
		}
	}

	cmd := newScheduleCmd()
	cmd.SetArgs([]string{"run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("schedule run failed: %v", err)
	}

	logPath := filepath.Join(paths.LogDir, "schedule.log")
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading schedule.log: %v", err)
	}
	logStr := string(logBytes)

	if !strings.Contains(logStr, "status=PARTIAL") {
		t.Errorf("expected status=PARTIAL in log footer, got:\n%s", logStr)
	}
}

func TestScheduleRunCmd_SymlinkRemoval(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmpDir, ".local", "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	resolver := platform.New()
	paths := resolver.Resolve()
	if err := os.MkdirAll(paths.LogDir, 0755); err != nil {
		t.Fatalf("creating log dir: %v", err)
	}

	secretFile := filepath.Join(tmpDir, "secret.conf")
	if err := os.WriteFile(secretFile, []byte("secret-data"), 0600); err != nil {
		t.Fatalf("writing secret: %v", err)
	}

	logPath := filepath.Join(paths.LogDir, "schedule.log")
	if err := os.Symlink(secretFile, logPath); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	oldRunner := runCleanSubprocess
	defer func() { runCleanSubprocess = oldRunner }()
	runCleanSubprocess = func(ctx context.Context, configDir string, args []string, logWriter io.Writer) cleanExecutionResult {
		return cleanExecutionResult{ExitCode: 0, Status: "OK", FreedHuman: "0 B"}
	}

	cmd := newScheduleCmd()
	cmd.SetArgs([]string{"run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("schedule run failed: %v", err)
	}

	// Target must be untouched
	secBytes, err := os.ReadFile(secretFile)
	if err != nil || string(secBytes) != "secret-data" {
		t.Errorf("secret file modified or missing: %v, content: %q", err, string(secBytes))
	}

	// schedule.log must now be a regular file, not a symlink
	fi, err := os.Lstat(logPath)
	if err != nil {
		t.Fatalf("reading log lstat: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Error("schedule.log should no longer be a symlink")
	}
}

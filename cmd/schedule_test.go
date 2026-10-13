package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/scheduler"
)

func TestScheduleCmd_MissingFlags(t *testing.T) {
	cmd := newScheduleCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no flags provided")
	}
}

func TestScheduleCmd_BothFlags(t *testing.T) {
	cmd := newScheduleCmd()
	cmd.SetArgs([]string{"--daily", "--cron", "0 12 * * *"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when both --daily and --cron provided")
	}
}

func TestScheduleCmd_InvalidCron(t *testing.T) {
	cmd := newScheduleCmd()
	cmd.SetArgs([]string{"--cron", "invalid cron expr too many"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid cron expression")
	}
}

func TestScheduleCmd_StatusFlag(t *testing.T) {
	cmd := newScheduleCmd()
	cmd.SetArgs([]string{"--status"})
	// Status should succeed (shows "no schedule" for fresh system)
	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error for --status: %v", err)
	}

	// Status with watch mode
	cmdWatch := newScheduleCmd()
	cmdWatch.SetArgs([]string{"--status", "--watch"})
	if err := cmdWatch.Execute(); err != nil {
		t.Errorf("unexpected error for --status --watch: %v", err)
	}
}

func TestUnscheduleCmd_Execution(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Test unschedule when not installed
	unCmd := newUnscheduleCmd()
	if err := unCmd.Execute(); err != nil {
		t.Errorf("unschedule on fresh system failed: %v", err)
	}

	// Test unschedule --watch when not installed
	unCmdWatch := newUnscheduleCmd()
	unCmdWatch.SetArgs([]string{"--watch"})
	if err := unCmdWatch.Execute(); err != nil {
		t.Errorf("unschedule --watch failed: %v", err)
	}
}

func TestShowStatus_Function(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")

	// Standard status
	if err := showStatus(configDir, false); err != nil {
		t.Errorf("showStatus failed: %v", err)
	}

	// Watch status
	if err := showStatus(configDir, true); err != nil {
		t.Errorf("showStatus watchMode failed: %v", err)
	}
}

func TestShowStatus_Active(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	configDir := filepath.Join(tmpDir, "config")
	_ = os.MkdirAll(configDir, 0755)

	// Simular plist de schedule instalado (macOS)
	launchDir := filepath.Join(tmpDir, "Library", "LaunchAgents")
	_ = os.MkdirAll(launchDir, 0755)
	plistPath := filepath.Join(launchDir, "com.devclean.scheduled.plist")
	plistContent := `<plist><dict>
	<key>ProgramArguments</key>
	<array>
		<string>/usr/local/bin/devclean</string>
	</array>
</dict></plist>`
	_ = os.WriteFile(plistPath, []byte(plistContent), 0644)

	// Simular histórico de execução
	_ = scheduler.RecordRun(configDir, scheduler.RunResult{
		Timestamp:    time.Now(),
		TargetsCount: 5,
		BytesFreed:   1048576,
		Duration:     "500ms",
	})

	if err := showStatus(configDir, false); err != nil {
		t.Errorf("showStatus failed on active schedule: %v", err)
	}
}

func TestScheduleCmd_WatchModeAlreadyInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	launchDir := filepath.Join(tmpDir, "Library", "LaunchAgents")
	_ = os.MkdirAll(launchDir, 0755)
	watchPlist := filepath.Join(launchDir, "com.devclean.watch.plist")
	_ = os.WriteFile(watchPlist, []byte("<plist></plist>"), 0644)

	cmd := newScheduleCmd()
	cmd.SetArgs([]string{"--watch"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error on schedule --watch: %v", err)
	}

	cmdStatusWatch := newScheduleCmd()
	cmdStatusWatch.SetArgs([]string{"--status", "--watch"})
	if err := cmdStatusWatch.Execute(); err != nil {
		t.Errorf("unexpected error on --status --watch: %v", err)
	}
}

func TestUnscheduleCmd_WhenInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	launchDir := filepath.Join(tmpDir, "Library", "LaunchAgents")
	_ = os.MkdirAll(launchDir, 0755)
	plistPath := filepath.Join(launchDir, "com.devclean.scheduled.plist")
	_ = os.WriteFile(plistPath, []byte("<plist></plist>"), 0644)

	watchPlist := filepath.Join(launchDir, "com.devclean.watch.plist")
	_ = os.WriteFile(watchPlist, []byte("<plist></plist>"), 0644)

	unCmd := newUnscheduleCmd()
	_ = unCmd.Execute()

	unWatchCmd := newUnscheduleCmd()
	unWatchCmd.SetArgs([]string{"--watch"})
	_ = unWatchCmd.Execute()
}

func TestScheduleCmd_WatchModeInstall(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cmd := newScheduleCmd()
	cmd.SetArgs([]string{"--watch"})
	_ = cmd.Execute()
}

func TestResolveCronExpression_Weekly(t *testing.T) {
	expr, err := resolveCronExpression(false, true, false, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expr != "0 0 * * 0" {
		t.Errorf("got %q, want '0 0 * * 0'", expr)
	}
}

func TestResolveCronExpression_Hourly(t *testing.T) {
	expr, err := resolveCronExpression(false, false, true, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expr != "0 * * * *" {
		t.Errorf("got %q, want '0 * * * *'", expr)
	}

	// Hourly with minute in --at
	exprAt, err := resolveCronExpression(false, false, true, "", "15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exprAt != "15 * * * *" {
		t.Errorf("got %q, want '15 * * * *'", exprAt)
	}
}

func TestResolveCronExpression_DailyWithAt(t *testing.T) {
	expr, err := resolveCronExpression(true, false, false, "", "04:30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expr != "30 4 * * *" {
		t.Errorf("got %q, want '30 4 * * *'", expr)
	}
}

func TestResolveCronExpression_InvalidAt(t *testing.T) {
	invalidCases := []string{"25:00", "abc", "12:65", "12", "-1:00", "12:00:00"}
	for _, tc := range invalidCases {
		_, err := resolveCronExpression(true, false, false, "", tc)
		if err == nil {
			t.Errorf("expected error for invalid --at %q, got nil", tc)
		}
	}
}

func TestResolveCronExpression_ConflictingShortcuts(t *testing.T) {
	// weekly and daily together
	_, err := resolveCronExpression(true, true, false, "", "")
	if err == nil {
		t.Error("expected error for weekly and daily together, got nil")
	}

	// shortcut and cron together
	_, err = resolveCronExpression(true, false, false, "0 0 * * *", "")
	if err == nil {
		t.Error("expected error for shortcut and cron together, got nil")
	}

	// neither provided
	_, err = resolveCronExpression(false, false, false, "", "")
	if err == nil {
		t.Error("expected error when no shortcut or cron provided, got nil")
	}
}

func TestValidateScheduledRun(t *testing.T) {
	// skipValidation returns nil
	if err := validateScheduledRun([]string{"--preset", "safe"}, true); err != nil {
		t.Errorf("expected nil when skipValidation=true, got %v", err)
	}

	// mock failure
	oldExec := dryRunExecutor
	defer func() { dryRunExecutor = oldExec }()

	dryRunExecutor = func(args []string) error {
		return fmt.Errorf("simulated dry-run failure")
	}
	if err := validateScheduledRun([]string{"--preset", "safe"}, false); err == nil {
		t.Error("expected error when dryRunExecutor fails, got nil")
	}

	dryRunExecutor = func(args []string) error {
		return nil
	}
	if err := validateScheduledRun([]string{"--preset", "safe"}, false); err != nil {
		t.Errorf("expected nil when dryRunExecutor succeeds, got %v", err)
	}
}





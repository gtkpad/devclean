package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

type mockScheduler struct {
	installed   *scheduler.Schedule
	isInstalled bool
	removed     bool
}

func (m *mockScheduler) Install(s scheduler.Schedule) error {
	m.installed = &s
	m.isInstalled = true
	return nil
}

func (m *mockScheduler) Remove() error {
	m.removed = true
	m.isInstalled = false
	return nil
}

func (m *mockScheduler) Status() (*scheduler.Status, error) {
	return &scheduler.Status{Active: m.isInstalled}, nil
}

func (m *mockScheduler) IsInstalled() bool {
	return m.isInstalled
}

func (m *mockScheduler) InstallWatch(scheduler.WatchSchedule) error { return nil }
func (m *mockScheduler) RemoveWatch() error                         { return nil }
func (m *mockScheduler) IsWatchInstalled() bool                     { return false }

func TestScheduleCmd_ShortcutsAndArgs(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	configDir := filepath.Join(tmpDir, ".config", "devclean")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(configDir, "config.yaml")
	initialContent := "# Preserved Comment\nscan_paths:\n  - /tmp\n"
	if err := os.WriteFile(configFile, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	mock := &mockScheduler{}
	oldFactory := schedulerFactory
	defer func() { schedulerFactory = oldFactory }()
	schedulerFactory = func(dir string) scheduler.Scheduler {
		return mock
	}

	oldExec := dryRunExecutor
	defer func() { dryRunExecutor = oldExec }()
	dryRunExecutor = func(args []string) error {
		return nil
	}

	cmd := newScheduleCmd()
	cmd.SetArgs([]string{"--weekly", "--preset", "safe"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected successful schedule execution, got: %v", err)
	}

	if mock.installed == nil {
		t.Fatal("expected scheduler.Install to be called")
	}
	if mock.installed.CronExpr != "0 0 * * 0" {
		t.Errorf("CronExpr = %q, want '0 0 * * 0'", mock.installed.CronExpr)
	}

	// Verify config.yaml
	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "# Preserved Comment") {
		t.Errorf("expected comment '# Preserved Comment' to be preserved in config.yaml, got:\n%s", content)
	}
	if !strings.Contains(content, "safe") || !strings.Contains(content, "preset") {
		t.Errorf("expected scheduler.args to contain preset safe, got:\n%s", content)
	}
}

func TestScheduleCmd_DryRunFailure(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	mock := &mockScheduler{}
	oldFactory := schedulerFactory
	defer func() { schedulerFactory = oldFactory }()
	schedulerFactory = func(dir string) scheduler.Scheduler {
		return mock
	}

	oldExec := dryRunExecutor
	defer func() { dryRunExecutor = oldExec }()
	dryRunExecutor = func(args []string) error {
		return fmt.Errorf("simulated dry-run failure")
	}

	cmd := newScheduleCmd()
	cmd.SetArgs([]string{"--daily"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error due to dry-run failure, got nil")
	}
	if mock.installed != nil {
		t.Error("expected scheduler.Install NOT to be called after dry-run failure")
	}
}

func TestScheduleCmd_SkipValidation(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	mock := &mockScheduler{}
	oldFactory := schedulerFactory
	defer func() { schedulerFactory = oldFactory }()
	schedulerFactory = func(dir string) scheduler.Scheduler {
		return mock
	}

	oldExec := dryRunExecutor
	defer func() { dryRunExecutor = oldExec }()
	dryRunExecutor = func(args []string) error {
		return fmt.Errorf("dry-run should not be called")
	}

	cmd := newScheduleCmd()
	cmd.SetArgs([]string{"--daily", "--skip-validation"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected schedule to succeed with --skip-validation, got: %v", err)
	}
	if mock.installed == nil {
		t.Fatal("expected scheduler.Install to be called")
	}
	if mock.installed.CronExpr != "0 12 * * *" {
		t.Errorf("CronExpr = %q, want '0 12 * * *'", mock.installed.CronExpr)
	}
}

func TestScheduleCmd_ProviderFlag(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	mock := &mockScheduler{}
	oldFactory := schedulerFactory
	defer func() { schedulerFactory = oldFactory }()
	schedulerFactory = func(dir string) scheduler.Scheduler {
		return mock
	}
	oldExec := dryRunExecutor
	defer func() { dryRunExecutor = oldExec }()
	dryRunExecutor = func(args []string) error { return nil }

	// Invalid provider
	cmd := newScheduleCmd()
	cmd.SetArgs([]string{"--daily", "--provider", "invalid-provider"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid provider, got nil")
	}

	// Valid provider systemd
	cmdSystemd := newScheduleCmd()
	cmdSystemd.SetArgs([]string{"--daily", "--provider", "systemd", "--skip-validation"})
	if err := cmdSystemd.Execute(); err != nil {
		t.Errorf("expected provider systemd to succeed, got: %v", err)
	}

	// Valid provider cron
	cmdCron := newScheduleCmd()
	cmdCron.SetArgs([]string{"--daily", "--provider", "cron", "--skip-validation"})
	if err := cmdCron.Execute(); err != nil {
		t.Errorf("expected provider cron to succeed, got: %v", err)
	}
}





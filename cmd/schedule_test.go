package cmd

import (
	"path/filepath"
	"testing"
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

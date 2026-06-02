package cmd

import (
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
}

//go:build darwin

package scheduler

import (
	"strings"
	"testing"
)

func TestGeneratePlist_DailyNoon(t *testing.T) {
	fields := CronFields{
		Minute:     "0",
		Hour:       "12",
		DayOfMonth: "*",
		Month:      "*",
		DayOfWeek:  "*",
	}

	content, err := generatePlist("/usr/local/bin/devclean", fields)
	if err != nil {
		t.Fatalf("generatePlist() error: %v", err)
	}

	if !strings.Contains(content, "<key>Hour</key>") {
		t.Error("expected Hour key in plist")
	}
	if !strings.Contains(content, "<integer>12</integer>") {
		t.Error("expected integer 12 for Hour")
	}
	if !strings.Contains(content, "<key>Minute</key>") {
		t.Error("expected Minute key in plist")
	}
	if !strings.Contains(content, "<integer>0</integer>") {
		t.Error("expected integer 0 for Minute")
	}
	if strings.Contains(content, "<key>Day</key>") {
		t.Error("should NOT contain Day key for wildcard day_of_month")
	}
	if strings.Contains(content, "<key>Weekday</key>") {
		t.Error("should NOT contain Weekday key for wildcard day_of_week")
	}
	if strings.Contains(content, "<key>Month</key>") {
		t.Error("should NOT contain Month key for wildcard month")
	}
}

func TestGeneratePlist_MondayMorning(t *testing.T) {
	fields := CronFields{
		Minute:     "0",
		Hour:       "9",
		DayOfMonth: "*",
		Month:      "*",
		DayOfWeek:  "1",
	}

	content, err := generatePlist("/usr/local/bin/devclean", fields)
	if err != nil {
		t.Fatalf("generatePlist() error: %v", err)
	}

	if !strings.Contains(content, "<key>Weekday</key>") {
		t.Error("expected Weekday key in plist")
	}
	if !strings.Contains(content, "<integer>1</integer>") {
		t.Error("expected integer 1 for Weekday")
	}
}

func TestGeneratePlist_ContainsLabel(t *testing.T) {
	fields := CronFields{
		Minute:     "0",
		Hour:       "0",
		DayOfMonth: "*",
		Month:      "*",
		DayOfWeek:  "*",
	}

	content, err := generatePlist("/usr/local/bin/devclean", fields)
	if err != nil {
		t.Fatalf("generatePlist() error: %v", err)
	}

	if !strings.Contains(content, plistLabel) {
		t.Error("expected plist label in output")
	}
	if !strings.Contains(content, "/usr/local/bin/devclean") {
		t.Error("expected binary path in output")
	}
	if !strings.Contains(content, "<string>clean</string>") {
		t.Error("expected clean argument in ProgramArguments")
	}
	if !strings.Contains(content, "<string>--auto</string>") {
		t.Error("expected --auto argument in ProgramArguments")
	}
}

func TestExtractBinaryFromPlist(t *testing.T) {
	fields := CronFields{Minute: "0", Hour: "12", DayOfMonth: "*", Month: "*", DayOfWeek: "*"}
	content, _ := generatePlist("/opt/bin/devclean", fields)

	got := extractBinaryFromPlist(content)
	if got != "/opt/bin/devclean" {
		t.Errorf("extractBinaryFromPlist() = %q, want %q", got, "/opt/bin/devclean")
	}
}

package scheduler

import (
	"strings"
	"testing"
)

func TestCronToOnCalendar(t *testing.T) {
	tests := []struct {
		name     string
		cronExpr string
		expected string
		wantErr  bool
		errSub   string
	}{
		// Standard patterns from requirements
		{
			name:     "daily at noon",
			cronExpr: "0 12 * * *",
			expected: "*-*-* 12:00:00",
		},
		{
			name:     "weekly Sunday (0)",
			cronExpr: "0 0 * * 0",
			expected: "Sun *-*-* 00:00:00",
		},
		{
			name:     "weekly Sunday (7)",
			cronExpr: "0 0 * * 7",
			expected: "Sun *-*-* 00:00:00",
		},
		{
			name:     "hourly",
			cronExpr: "0 * * * *",
			expected: "*-*-* *:00:00",
		},
		{
			name:     "weekdays at 04:30",
			cronExpr: "30 4 * * 1-5",
			expected: "Mon..Fri *-*-* 04:30:00",
		},
		{
			name:     "every 15 minutes",
			cronExpr: "*/15 * * * *",
			expected: "*-*-* *:00/15:00",
		},
		// Additional coverage: date formatting
		{
			name:     "first day of January at midnight",
			cronExpr: "0 0 1 1 *",
			expected: "*-01-01 00:00:00",
		},
		{
			name:     "15th of every month at midnight",
			cronExpr: "0 0 15 * *",
			expected: "*-*-15 00:00:00",
		},
		{
			name:     "hour range and weekdays",
			cronExpr: "0 9-17 * * 1-5",
			expected: "Mon..Fri *-*-* 09..17:00:00",
		},
		{
			name:     "minute list",
			cronExpr: "0,30 * * * *",
			expected: "*-*-* *:00,30:00",
		},
		{
			name:     "DOW list",
			cronExpr: "0 0 * * 1,3,5",
			expected: "Mon,Wed,Fri *-*-* 00:00:00",
		},
		{
			name:     "single day of week Monday",
			cronExpr: "0 0 * * 1",
			expected: "Mon *-*-* 00:00:00",
		},
		{
			name:     "range with step in minute",
			cronExpr: "0-30/5 * * * *",
			expected: "*-*-* *:00..30/5:00",
		},
		{
			name:     "whitespace normalization with tabs and multiple spaces",
			cronExpr: "  0 \t 12   *   *   *  ",
			expected: "*-*-* 12:00:00",
		},
		// Error cases
		{
			name:     "invalid field count",
			cronExpr: "0 12 * *",
			wantErr:  true,
		},
		{
			name:     "minute out of range",
			cronExpr: "60 12 * * *",
			wantErr:  true,
		},
		{
			name:     "hour out of range",
			cronExpr: "0 24 * * *",
			wantErr:  true,
		},
		{
			name:     "day of month out of range",
			cronExpr: "0 12 32 * *",
			wantErr:  true,
		},
		{
			name:     "month out of range",
			cronExpr: "0 12 * 13 *",
			wantErr:  true,
		},
		{
			name:     "day of week out of range",
			cronExpr: "0 0 * * 8",
			wantErr:  true,
		},
		{
			name:     "shell injection semicolon",
			cronExpr: "0 12 * * * ; rm -rf /",
			wantErr:  true,
		},
		{
			name:     "non-numeric field",
			cronExpr: "abc 12 * * *",
			wantErr:  true,
		},
		{
			name:     "step in day of week unsupported",
			cronExpr: "0 0 * * */2",
			wantErr:  true,
			errSub:   "step values not supported for day of week",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CronToOnCalendar(tt.cronExpr)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CronToOnCalendar(%q) error = %v, wantErr %v", tt.cronExpr, err, tt.wantErr)
			}
			if tt.wantErr {
				if tt.errSub != "" && (err == nil || !strings.Contains(err.Error(), tt.errSub)) {
					t.Errorf("CronToOnCalendar(%q) error = %v, want error containing %q", tt.cronExpr, err, tt.errSub)
				}
				return
			}
			if got != tt.expected {
				t.Errorf("CronToOnCalendar(%q) = %q, want %q", tt.cronExpr, got, tt.expected)
			}
		})
	}
}

func TestMapCronDayOfWeek_EdgeCases(t *testing.T) {
	wild, err := mapCronDayOfWeek("*")
	if err != nil || wild != "*" {
		t.Errorf("mapCronDayOfWeek(*) = %q, %v; want *", wild, err)
	}

	_, err = mapCronDayOfWeek("invalid")
	if err == nil {
		t.Error("expected error for invalid day of week")
	}

	_, err = mapCronDayOfWeek("1-invalid")
	if err == nil {
		t.Error("expected error for 1-invalid")
	}

	_, err = mapCronDayOfWeek("invalid-5")
	if err == nil {
		t.Error("expected error for invalid-5")
	}

	_, err = mapCronDayOfWeek("9")
	if err == nil {
		t.Error("expected error for out of range day of week 9")
	}

	_, err = mapCronDayOfWeek("1-9")
	if err == nil {
		t.Error("expected error for out of range day of week range 1-9")
	}
}

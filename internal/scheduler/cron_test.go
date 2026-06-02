package scheduler

import (
	"testing"
)

func TestValidateCron(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{"every day at noon", "0 12 * * *", false},
		{"every 15min weekdays 9-17", "*/15 9-17 * * 1-5", false},
		{"first of month midnight", "0 0 1 * *", false},
		{"all wildcards", "* * * * *", false},
		{"list values", "0,15,30,45 * * * *", false},
		{"minute out of range", "60 12 * * *", true},
		{"hour out of range", "0 25 * * *", true},
		{"day out of range", "0 12 32 * *", true},
		{"month out of range", "0 12 * 13 *", true},
		{"not a cron", "not a cron", true},
		{"too few fields", "0 12 *", true},
		{"injection semicolon", "0 12 * * * ; rm -rf /", true},
		{"injection pipe", "0 12 * * *|cat /etc/passwd", true},
		{"injection backtick", "0 12 * * `id`", true},
		{"injection ampersand", "0 12 * * *&&echo pwned", true},
		{"step expression", "*/5 * * * *", false},
		{"range with step", "0-30/5 * * * *", false},
		{"dow value 7 (sunday)", "0 0 * * 7", false},
		{"invalid step", "*/0 * * * *", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCron(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCron(%q) error = %v, wantErr %v", tt.expr, err, tt.wantErr)
			}
		})
	}
}

func TestParseCron(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		expect CronFields
	}{
		{
			name: "noon daily",
			expr: "0 12 * * *",
			expect: CronFields{
				Minute:     "0",
				Hour:       "12",
				DayOfMonth: "*",
				Month:      "*",
				DayOfWeek:  "*",
			},
		},
		{
			name: "weekday mornings",
			expr: "30 9 * * 1-5",
			expect: CronFields{
				Minute:     "30",
				Hour:       "9",
				DayOfMonth: "*",
				Month:      "*",
				DayOfWeek:  "1-5",
			},
		},
		{
			name: "monthly first",
			expr: "0 0 1 * *",
			expect: CronFields{
				Minute:     "0",
				Hour:       "0",
				DayOfMonth: "1",
				Month:      "*",
				DayOfWeek:  "*",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCron(tt.expr)
			if err != nil {
				t.Fatalf("ParseCron(%q) unexpected error: %v", tt.expr, err)
			}
			if got != tt.expect {
				t.Errorf("ParseCron(%q) = %+v, want %+v", tt.expr, got, tt.expect)
			}
		})
	}
}

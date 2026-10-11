package scheduler

import (
	"errors"
	"strings"
	"testing"
	"time"
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

func TestCronScheduler_InstallAndRemove_Success(t *testing.T) {
	tmpDir := t.TempDir()
	mock := NewMockExecutor()
	sched := NewCronSchedulerWithExecutor(tmpDir, mock)

	// Status when crontab is empty
	st, err := sched.Status()
	if err != nil {
		t.Fatalf("Status error: %v", err)
	}
	if st.Active {
		t.Error("expected Active = false")
	}
	if sched.IsInstalled() {
		t.Error("expected IsInstalled = false initially")
	}

	// Install schedule
	err = sched.Install(Schedule{
		CronExpr:   "0 12 * * *",
		BinaryPath: "/usr/local/bin/devclean",
	})
	if err != nil {
		t.Fatalf("Install unexpected error: %v", err)
	}

	// Verify crontab write was executed
	var crontabWritten bool
	for _, call := range mock.Calls {
		if len(call) >= 2 && call[0] == "crontab" && call[1] == "-" {
			crontabWritten = true
			break
		}
	}
	if !crontabWritten {
		t.Error("expected call to 'crontab -' in mock calls")
	}

	// Simulate crontab containing devclean schedule
	crontabContent := cronMarker + "\n0 12 * * * /usr/local/bin/devclean clean --auto\n"
	mock.Outputs["crontab -l"] = []byte(crontabContent)

	if !sched.IsInstalled() {
		t.Error("expected IsInstalled = true")
	}

	// Subsequent Install should return ErrAlreadyInstalled
	err = sched.Install(Schedule{
		CronExpr:   "0 12 * * *",
		BinaryPath: "/usr/local/bin/devclean",
	})
	if !errors.Is(err, ErrAlreadyInstalled) {
		t.Fatalf("expected ErrAlreadyInstalled, got %v", err)
	}

	// Record a run and check Status
	runTime := time.Now().Truncate(time.Second)
	_ = RecordRun(tmpDir, RunResult{
		Timestamp:    runTime,
		TargetsCount: 3,
		BytesFreed:   4096,
		Duration:     "50ms",
	})

	st, err = sched.Status()
	if err != nil {
		t.Fatalf("Status error: %v", err)
	}
	if !st.Active {
		t.Error("expected Active = true")
	}
	if st.CronExpr != "0 12 * * *" {
		t.Errorf("CronExpr = %q, want '0 12 * * *'", st.CronExpr)
	}
	if st.BinaryPath != "/usr/local/bin/devclean" {
		t.Errorf("BinaryPath = %q, want '/usr/local/bin/devclean'", st.BinaryPath)
	}
	if st.LastRun == nil || !st.LastRun.Equal(runTime) {
		t.Errorf("LastRun mismatch: got %v, want %v", st.LastRun, runTime)
	}

	// Remove schedule
	err = sched.Remove()
	if err != nil {
		t.Fatalf("Remove unexpected error: %v", err)
	}

	// Simulate crontab without marker
	mock.Outputs["crontab -l"] = []byte("")

	if sched.IsInstalled() {
		t.Error("expected IsInstalled = false after Remove")
	}

	// Subsequent Remove should return ErrNotInstalled
	err = sched.Remove()
	if !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("expected ErrNotInstalled on repeated Remove, got %v", err)
	}
}

func TestCronScheduler_Watch(t *testing.T) {
	tmpDir := t.TempDir()
	mock := NewMockExecutor()
	sched := NewCronSchedulerWithExecutor(tmpDir, mock)

	if sched.IsWatchInstalled() {
		t.Error("expected IsWatchInstalled = false initially")
	}

	// InstallWatch with interval < 30min (clamped to 30min: */30 * * * *)
	err := sched.InstallWatch(WatchSchedule{
		BinaryPath: "/usr/local/bin/devclean",
		Interval:   15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("InstallWatch unexpected error: %v", err)
	}

	// InstallWatch with interval >= 60min (2 hours: 0 */2 * * *)
	sched2 := NewCronSchedulerWithExecutor(tmpDir, mock)
	err = sched2.InstallWatch(WatchSchedule{
		BinaryPath: "/usr/local/bin/devclean",
		Interval:   2 * time.Hour,
	})
	if err != nil {
		t.Fatalf("InstallWatch unexpected error for 2h: %v", err)
	}

	// Simulate crontab containing watch entry
	watchEntry := watchCronMarker + "\n0 */2 * * * /usr/local/bin/devclean --watch-daemon\n"
	mock.Outputs["crontab -l"] = []byte(watchEntry)

	if !sched.IsWatchInstalled() {
		t.Error("expected IsWatchInstalled = true")
	}

	// Subsequent InstallWatch returns ErrAlreadyInstalled
	err = sched.InstallWatch(WatchSchedule{BinaryPath: "/usr/local/bin/devclean", Interval: time.Hour})
	if !errors.Is(err, ErrAlreadyInstalled) {
		t.Fatalf("expected ErrAlreadyInstalled, got %v", err)
	}

	// RemoveWatch
	err = sched.RemoveWatch()
	if err != nil {
		t.Fatalf("RemoveWatch unexpected error: %v", err)
	}

	// Simulate cleaned crontab
	mock.Outputs["crontab -l"] = []byte("")
	if sched.IsWatchInstalled() {
		t.Error("expected IsWatchInstalled = false after RemoveWatch")
	}

	// Repeat RemoveWatch returns ErrNotInstalled
	err = sched.RemoveWatch()
	if !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("expected ErrNotInstalled, got %v", err)
	}
}

func TestCronScheduler_ErrorBranches(t *testing.T) {
	tmpDir := t.TempDir()
	mock := NewMockExecutor()
	sched := NewCronSchedulerWithExecutor(tmpDir, mock)

	// 1. Invalid cron expression
	err := sched.Install(Schedule{CronExpr: "invalid"})
	if err == nil || !strings.Contains(err.Error(), "invalid cron expression") {
		t.Fatalf("expected invalid cron error, got %v", err)
	}

	// 2. LookPath error when BinaryPath is empty
	mock.Errors["lookpath:devclean"] = errors.New("devclean not in PATH")
	err = sched.Install(Schedule{CronExpr: "0 12 * * *"})
	if err == nil || !strings.Contains(err.Error(), "not found in PATH") {
		t.Fatalf("expected LookPath error, got %v", err)
	}
	delete(mock.Errors, "lookpath:devclean")

	// 3. crontab write error
	mock.Errors["crontab -"] = errors.New("cannot write crontab")
	mock.Outputs["crontab -"] = []byte("crontab: access denied")
	err = sched.Install(Schedule{CronExpr: "0 12 * * *", BinaryPath: "/bin/devclean"})
	if err == nil || !strings.Contains(err.Error(), "crontab failed") {
		t.Fatalf("expected crontab write error, got %v", err)
	}
	delete(mock.Errors, "crontab -")

	// 4. Watch LookPath error
	mock.Errors["lookpath:devclean"] = errors.New("devclean not in PATH")
	err = sched.InstallWatch(WatchSchedule{Interval: time.Hour})
	if err == nil || !strings.Contains(err.Error(), "not found in PATH") {
		t.Fatalf("expected watch LookPath error, got %v", err)
	}
}

package scheduler

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	if !strings.Contains(content, "<string>schedule</string>") {
		t.Error("expected schedule argument in ProgramArguments")
	}
	if !strings.Contains(content, "<string>run</string>") {
		t.Error("expected run argument in ProgramArguments")
	}
}

func TestExtractBinaryFromPlist(t *testing.T) {
	fields := CronFields{Minute: "0", Hour: "12", DayOfMonth: "*", Month: "*", DayOfWeek: "*"}
	content, _ := generatePlist("/opt/bin/devclean", fields)

	got := extractBinaryFromPlist(content)
	if got != "/opt/bin/devclean" {
		t.Errorf("extractBinaryFromPlist() = %q, want %q", got, "/opt/bin/devclean")
	}

	if extractBinaryFromPlist("invalid content") != "" {
		t.Error("expected empty string for content without ProgramArguments")
	}
	if extractBinaryFromPlist("<key>ProgramArguments</key><dict></dict>") != "" {
		t.Error("expected empty string for malformed ProgramArguments")
	}
}

func TestLaunchdScheduler_InstallAndRemove_Success(t *testing.T) {
	tmpDir := t.TempDir()
	launchDir := filepath.Join(tmpDir, "LaunchAgents")
	mock := NewMockExecutor()

	sched := NewLaunchdSchedulerWithExecutor(tmpDir, mock, launchDir)

	// Verify not installed initially
	if sched.IsInstalled() {
		t.Error("expected IsInstalled = false initially")
	}

	// Install
	err := sched.Install(Schedule{
		CronExpr:   "0 12 * * *",
		BinaryPath: "/bin/devclean",
	})
	if err != nil {
		t.Fatalf("Install unexpected error: %v", err)
	}

	plistFile := filepath.Join(launchDir, plistLabel+".plist")
	if _, err := os.Stat(plistFile); err != nil {
		t.Fatalf("expected plist file to exist at %s: %v", plistFile, err)
	}

	if !sched.IsInstalled() {
		t.Error("expected IsInstalled = true after install")
	}

	// Verify launchctl load called
	var foundLoad bool
	for _, call := range mock.Calls {
		if len(call) >= 3 && call[0] == "launchctl" && call[1] == "load" && call[2] == plistFile {
			foundLoad = true
			break
		}
	}
	if !foundLoad {
		t.Error("expected launchctl load call in mock calls")
	}

	// Subsequent Install should return ErrAlreadyInstalled
	err = sched.Install(Schedule{
		CronExpr:   "0 12 * * *",
		BinaryPath: "/bin/devclean",
	})
	if !errors.Is(err, ErrAlreadyInstalled) {
		t.Fatalf("expected ErrAlreadyInstalled, got %v", err)
	}

	// Remove
	if err := sched.Remove(); err != nil {
		t.Fatalf("Remove unexpected error: %v", err)
	}

	if sched.IsInstalled() {
		t.Error("expected IsInstalled = false after remove")
	}

	if _, err := os.Stat(plistFile); !os.IsNotExist(err) {
		t.Error("expected plist file to be deleted")
	}

	// Subsequent Remove should return ErrNotInstalled
	err = sched.Remove()
	if !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("expected ErrNotInstalled on repeated Remove, got %v", err)
	}
}

func TestLaunchdScheduler_StatusAndWatch(t *testing.T) {
	tmpDir := t.TempDir()
	launchDir := filepath.Join(tmpDir, "LaunchAgents")
	mock := NewMockExecutor()

	sched := NewLaunchdSchedulerWithExecutor(tmpDir, mock, launchDir)

	// Status when inactive
	st, err := sched.Status()
	if err != nil {
		t.Fatalf("Status unexpected error: %v", err)
	}
	if st.Active {
		t.Error("expected Active = false")
	}
	if st.LastRun != nil {
		t.Error("expected LastRun = nil")
	}

	// Record a run in configDir
	runTime := time.Now().Truncate(time.Second)
	_ = RecordRun(tmpDir, RunResult{
		Timestamp:    runTime,
		TargetsCount: 5,
		BytesFreed:   2048,
		Duration:     "100ms",
	})

	// Install schedule
	err = sched.Install(Schedule{
		CronExpr:   "0 12 * * *",
		BinaryPath: "/bin/devclean",
	})
	if err != nil {
		t.Fatalf("Install unexpected error: %v", err)
	}

	// Status when active
	st, err = sched.Status()
	if err != nil {
		t.Fatalf("Status unexpected error: %v", err)
	}
	if !st.Active {
		t.Error("expected Active = true")
	}
	if st.BinaryPath != "/bin/devclean" {
		t.Errorf("BinaryPath = %q, want /bin/devclean", st.BinaryPath)
	}
	if st.LastRun == nil || !st.LastRun.Equal(runTime) {
		t.Errorf("LastRun mismatch: got %v, want %v", st.LastRun, runTime)
	}
	if st.LastResult == nil || st.LastResult.BytesFreed != 2048 {
		t.Errorf("LastResult mismatch: got %+v", st.LastResult)
	}

	// Watch testing
	if sched.IsWatchInstalled() {
		t.Error("expected IsWatchInstalled = false initially")
	}

	// InstallWatch
	err = sched.InstallWatch(WatchSchedule{
		BinaryPath: "/bin/devclean",
		Interval:   10 * time.Minute, // Should be clamped to minimum 1800s (30m)
	})
	if err != nil {
		t.Fatalf("InstallWatch unexpected error: %v", err)
	}

	watchPlist := filepath.Join(launchDir, watchPlistLabel+".plist")
	if _, err := os.Stat(watchPlist); err != nil {
		t.Fatalf("expected watch plist at %s", watchPlist)
	}

	watchContent, _ := os.ReadFile(watchPlist)
	if !strings.Contains(string(watchContent), "<integer>1800</integer>") {
		t.Error("expected interval clamped to 1800 in watch plist")
	}

	if !sched.IsWatchInstalled() {
		t.Error("expected IsWatchInstalled = true after install")
	}

	// Repeat InstallWatch returns ErrAlreadyInstalled
	err = sched.InstallWatch(WatchSchedule{BinaryPath: "/bin/devclean", Interval: time.Hour})
	if !errors.Is(err, ErrAlreadyInstalled) {
		t.Fatalf("expected ErrAlreadyInstalled for watch, got %v", err)
	}

	// RemoveWatch
	err = sched.RemoveWatch()
	if err != nil {
		t.Fatalf("RemoveWatch unexpected error: %v", err)
	}
	if sched.IsWatchInstalled() {
		t.Error("expected IsWatchInstalled = false after RemoveWatch")
	}

	// Repeat RemoveWatch returns ErrNotInstalled
	err = sched.RemoveWatch()
	if !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("expected ErrNotInstalled on repeated RemoveWatch, got %v", err)
	}
}

func TestLaunchdScheduler_ErrorBranches(t *testing.T) {
	tmpDir := t.TempDir()
	launchDir := filepath.Join(tmpDir, "LaunchAgents")
	mock := NewMockExecutor()

	sched := NewLaunchdSchedulerWithExecutor(tmpDir, mock, launchDir)

	// 1. Invalid cron expression
	err := sched.Install(Schedule{CronExpr: "not-a-cron"})
	if err == nil || !strings.Contains(err.Error(), "invalid cron expression") {
		t.Fatalf("expected invalid cron error, got %v", err)
	}

	// 2. LookPath failure when BinaryPath is empty
	mock.Errors["lookpath:devclean"] = errors.New("binary not found in PATH")
	err = sched.Install(Schedule{CronExpr: "0 12 * * *"})
	if err == nil || !strings.Contains(err.Error(), "not found in PATH") {
		t.Fatalf("expected LookPath error, got %v", err)
	}
	delete(mock.Errors, "lookpath:devclean")

	// 3. Launchctl load failure during Install
	plistFile := filepath.Join(launchDir, plistLabel+".plist")
	loadCmdKey := "launchctl load " + plistFile
	mock.Errors[loadCmdKey] = errors.New("permission denied")
	mock.Outputs[loadCmdKey] = []byte("Bootstrap failed: 5: Input/output error")

	err = sched.Install(Schedule{CronExpr: "0 12 * * *", BinaryPath: "/bin/devclean"})
	if err == nil || !strings.Contains(err.Error(), "launchctl load failed") {
		t.Fatalf("expected launchctl load error, got %v", err)
	}
	// Plist file must have been cleaned up
	if _, err := os.Stat(plistFile); !os.IsNotExist(err) {
		t.Error("expected plist file to be removed on launchctl load failure")
	}
	delete(mock.Errors, loadCmdKey)

	// 4. Watch LookPath failure when BinaryPath is empty
	mock.Errors["lookpath:devclean"] = errors.New("watch binary not found")
	err = sched.InstallWatch(WatchSchedule{Interval: time.Hour})
	if err == nil || !strings.Contains(err.Error(), "not found in PATH") {
		t.Fatalf("expected watch LookPath error, got %v", err)
	}
	delete(mock.Errors, "lookpath:devclean")

	// 5. Watch launchctl load failure
	watchPlist := filepath.Join(launchDir, watchPlistLabel+".plist")
	watchLoadCmdKey := "launchctl load " + watchPlist
	mock.Errors[watchLoadCmdKey] = errors.New("watch load error")
	mock.Outputs[watchLoadCmdKey] = []byte("error output")

	err = sched.InstallWatch(WatchSchedule{BinaryPath: "/bin/devclean", Interval: time.Hour})
	if err == nil || !strings.Contains(err.Error(), "launchctl load failed") {
		t.Fatalf("expected watch launchctl load error, got %v", err)
	}
	if _, err := os.Stat(watchPlist); !os.IsNotExist(err) {
		t.Error("expected watch plist file to be removed on launchctl load failure")
	}
}

func TestOSExecutor_Methods(t *testing.T) {
	exec := &osExecutor{}
	ctx := context.Background()

	// LookPath
	p, err := exec.LookPath("go")
	if err != nil || p == "" {
		t.Logf("LookPath('go') note: %v", err)
	}

	// Output
	out, err := exec.Output(ctx, "echo", "test-echo")
	if err != nil || !strings.Contains(string(out), "test-echo") {
		t.Errorf("exec.Output failed: %v, out=%s", err, string(out))
	}

	// CombinedOutput
	out, err = exec.CombinedOutput(ctx, "echo", "test-combined")
	if err != nil || !strings.Contains(string(out), "test-combined") {
		t.Errorf("exec.CombinedOutput failed: %v, out=%s", err, string(out))
	}

	// CombinedOutputWithStdin
	out, err = exec.CombinedOutputWithStdin(ctx, "stdin-payload", "cat")
	if err != nil || !strings.Contains(string(out), "stdin-payload") {
		t.Errorf("exec.CombinedOutputWithStdin failed: %v, out=%s", err, string(out))
	}
}

func TestScheduler_NewConstructor(t *testing.T) {
	s := New(t.TempDir())
	if s == nil {
		t.Fatal("expected non-nil Scheduler from New()")
	}
}

package scheduler

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSystemdScheduler_InstallSuccess(t *testing.T) {
	configDir := t.TempDir()
	unitsDir := t.TempDir()
	mock := NewMockExecutor()

	sched := NewSystemdSchedulerWithExecutor(configDir, mock, unitsDir)

	if sched.IsInstalled() {
		t.Fatal("expected IsInstalled = false initially")
	}

	err := sched.Install(Schedule{
		CronExpr:   "0 12 * * *",
		BinaryPath: "/usr/local/bin/devclean",
	})
	if err != nil {
		t.Fatalf("Install unexpected error: %v", err)
	}

	if !sched.IsInstalled() {
		t.Fatal("expected IsInstalled = true after install")
	}

	// Verify service file content
	serviceBytes, err := os.ReadFile(filepath.Join(unitsDir, "devclean.service"))
	if err != nil {
		t.Fatalf("reading devclean.service: %v", err)
	}
	serviceStr := string(serviceBytes)
	if !strings.Contains(serviceStr, "Type=oneshot") {
		t.Errorf("devclean.service missing Type=oneshot: %s", serviceStr)
	}
	if !strings.Contains(serviceStr, "ExecStart=/usr/local/bin/devclean schedule run") {
		t.Errorf("devclean.service missing expected ExecStart: %s", serviceStr)
	}

	// Verify timer file content
	timerBytes, err := os.ReadFile(filepath.Join(unitsDir, "devclean.timer"))
	if err != nil {
		t.Fatalf("reading devclean.timer: %v", err)
	}
	timerStr := string(timerBytes)
	if !strings.Contains(timerStr, "OnCalendar=*-*-* 12:00:00") {
		t.Errorf("devclean.timer missing expected OnCalendar: %s", timerStr)
	}
	if !strings.Contains(timerStr, "Persistent=true") {
		t.Errorf("devclean.timer missing Persistent=true: %s", timerStr)
	}
	if !strings.Contains(timerStr, "RandomizedDelaySec=5m") {
		t.Errorf("devclean.timer missing RandomizedDelaySec=5m: %s", timerStr)
	}
	if !strings.Contains(timerStr, "WantedBy=timers.target") {
		t.Errorf("devclean.timer missing WantedBy=timers.target: %s", timerStr)
	}

	// Verify mock calls
	calls := mock.Calls
	var hasDaemonReload, hasEnableNow bool
	for _, call := range calls {
		cmdStr := strings.Join(call, " ")
		if strings.Contains(cmdStr, "systemctl --user daemon-reload") {
			hasDaemonReload = true
		}
		if strings.Contains(cmdStr, "systemctl --user enable --now devclean.timer") {
			hasEnableNow = true
		}
	}
	if !hasDaemonReload {
		t.Errorf("expected systemctl --user daemon-reload in calls: %v", calls)
	}
	if !hasEnableNow {
		t.Errorf("expected systemctl --user enable --now devclean.timer in calls: %v", calls)
	}
}

func TestSystemdScheduler_InstallAlreadyExists(t *testing.T) {
	configDir := t.TempDir()
	unitsDir := t.TempDir()
	mock := NewMockExecutor()

	sched := NewSystemdSchedulerWithExecutor(configDir, mock, unitsDir)

	err := sched.Install(Schedule{
		CronExpr:   "0 12 * * *",
		BinaryPath: "/usr/local/bin/devclean",
	})
	if err != nil {
		t.Fatalf("first Install failed: %v", err)
	}

	err = sched.Install(Schedule{
		CronExpr:   "0 12 * * *",
		BinaryPath: "/usr/local/bin/devclean",
	})
	if !errors.Is(err, ErrAlreadyInstalled) {
		t.Fatalf("expected ErrAlreadyInstalled, got %v", err)
	}
}

func TestSystemdScheduler_InstallErrorsAndRollback(t *testing.T) {
	configDir := t.TempDir()
	unitsDir := t.TempDir()
	mock := NewMockExecutor()

	sched := NewSystemdSchedulerWithExecutor(configDir, mock, unitsDir)

	// 1. Invalid cron expression
	err := sched.Install(Schedule{CronExpr: "invalid"})
	if err == nil {
		t.Fatal("expected error for invalid cron")
	}

	// 2. Binary path lookup error
	mock.Errors["lookpath:devclean"] = errors.New("devclean not in PATH")
	err = sched.Install(Schedule{CronExpr: "0 12 * * *"})
	if err == nil || !strings.Contains(err.Error(), "not found in PATH") {
		t.Fatalf("expected LookPath error, got %v", err)
	}
	delete(mock.Errors, "lookpath:devclean")

	// 3. Error during systemctl enable --now triggers rollback
	mock.Errors["systemctl --user enable --now devclean.timer"] = errors.New("failed to enable timer")
	err = sched.Install(Schedule{
		CronExpr:   "0 12 * * *",
		BinaryPath: "/usr/local/bin/devclean",
	})
	if err == nil {
		t.Fatal("expected error from enable --now")
	}

	// Verify rollback removed unit files
	if _, err := os.Stat(filepath.Join(unitsDir, "devclean.service")); !os.IsNotExist(err) {
		t.Error("expected devclean.service to be removed on rollback")
	}
	if _, err := os.Stat(filepath.Join(unitsDir, "devclean.timer")); !os.IsNotExist(err) {
		t.Error("expected devclean.timer to be removed on rollback")
	}
	delete(mock.Errors, "systemctl --user enable --now devclean.timer")

	// 4. Error during daemon-reload triggers rollback
	mock.Errors["systemctl --user daemon-reload"] = errors.New("failed daemon-reload")
	err = sched.Install(Schedule{
		CronExpr:   "0 12 * * *",
		BinaryPath: "/usr/local/bin/devclean",
	})
	if err == nil {
		t.Fatal("expected error from daemon-reload")
	}
	if _, err := os.Stat(filepath.Join(unitsDir, "devclean.service")); !os.IsNotExist(err) {
		t.Error("expected devclean.service to be removed on rollback")
	}
}

func TestSystemdScheduler_RemoveSuccess(t *testing.T) {
	configDir := t.TempDir()
	unitsDir := t.TempDir()
	mock := NewMockExecutor()

	sched := NewSystemdSchedulerWithExecutor(configDir, mock, unitsDir)

	err := sched.Install(Schedule{
		CronExpr:   "0 12 * * *",
		BinaryPath: "/usr/local/bin/devclean",
	})
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	err = sched.Remove()
	if err != nil {
		t.Fatalf("Remove unexpected error: %v", err)
	}

	if sched.IsInstalled() {
		t.Error("expected IsInstalled = false after Remove")
	}

	// Verify files removed
	if _, err := os.Stat(filepath.Join(unitsDir, "devclean.service")); !os.IsNotExist(err) {
		t.Error("expected devclean.service to be removed")
	}
	if _, err := os.Stat(filepath.Join(unitsDir, "devclean.timer")); !os.IsNotExist(err) {
		t.Error("expected devclean.timer to be removed")
	}

	// Verify disable call
	var hasDisable bool
	for _, call := range mock.Calls {
		if strings.Join(call, " ") == "systemctl --user disable --now devclean.timer" {
			hasDisable = true
			break
		}
	}
	if !hasDisable {
		t.Error("expected systemctl --user disable --now devclean.timer call")
	}

	// Second remove should return ErrNotInstalled
	err = sched.Remove()
	if !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("expected ErrNotInstalled on repeated Remove, got %v", err)
	}
}

func TestSystemdScheduler_StatusActive(t *testing.T) {
	configDir := t.TempDir()
	unitsDir := t.TempDir()
	mock := NewMockExecutor()

	sched := NewSystemdSchedulerWithExecutor(configDir, mock, unitsDir)

	// Inactive status initially
	st, err := sched.Status()
	if err != nil {
		t.Fatalf("Status error: %v", err)
	}
	if st.Active {
		t.Error("expected Active = false initially")
	}

	// Install schedule
	err = sched.Install(Schedule{
		CronExpr:   "0 12 * * *",
		BinaryPath: "/usr/local/bin/devclean",
	})
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Record last run
	runTime := time.Now().Truncate(time.Second)
	_ = RecordRun(configDir, RunResult{
		Timestamp:    runTime,
		TargetsCount: 5,
		BytesFreed:   1024 * 1024,
		Duration:     "120ms",
	})

	st, err = sched.Status()
	if err != nil {
		t.Fatalf("Status error: %v", err)
	}
	if !st.Active {
		t.Error("expected Active = true")
	}
	if st.CronExpr != "*-*-* 12:00:00" {
		t.Errorf("CronExpr = %q, want '*-*-* 12:00:00'", st.CronExpr)
	}
	if st.BinaryPath != "/usr/local/bin/devclean" {
		t.Errorf("BinaryPath = %q, want '/usr/local/bin/devclean'", st.BinaryPath)
	}
	if st.LastRun == nil || !st.LastRun.Equal(runTime) {
		t.Errorf("LastRun = %v, want %v", st.LastRun, runTime)
	}
}

func TestSystemdScheduler_WatchLifecycle(t *testing.T) {
	configDir := t.TempDir()
	unitsDir := t.TempDir()
	mock := NewMockExecutor()

	sched := NewSystemdSchedulerWithExecutor(configDir, mock, unitsDir)

	if sched.IsWatchInstalled() {
		t.Error("expected IsWatchInstalled = false initially")
	}

	// InstallWatch
	err := sched.InstallWatch(WatchSchedule{
		BinaryPath: "/usr/local/bin/devclean",
		Interval:   15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("InstallWatch unexpected error: %v", err)
	}

	if !sched.IsWatchInstalled() {
		t.Error("expected IsWatchInstalled = true after install")
	}

	// Verify watch service content
	watchBytes, err := os.ReadFile(filepath.Join(unitsDir, "devclean-watch.service"))
	if err != nil {
		t.Fatalf("reading devclean-watch.service: %v", err)
	}
	watchStr := string(watchBytes)
	if !strings.Contains(watchStr, "ExecStart=/usr/local/bin/devclean --watch-daemon") {
		t.Errorf("unexpected content in devclean-watch.service: %s", watchStr)
	}

	// Repeat install returns ErrAlreadyInstalled
	err = sched.InstallWatch(WatchSchedule{
		BinaryPath: "/usr/local/bin/devclean",
		Interval:   15 * time.Minute,
	})
	if !errors.Is(err, ErrAlreadyInstalled) {
		t.Fatalf("expected ErrAlreadyInstalled, got %v", err)
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

func TestSystemdScheduler_DefaultsAndErrors(t *testing.T) {
	// 1. Defaults when nil executor and empty unitsDir
	sched := NewSystemdSchedulerWithExecutor("cfg", nil, "")
	sysSched, ok := sched.(*systemdScheduler)
	if !ok {
		t.Fatal("expected *systemdScheduler")
	}
	if sysSched.exec == nil {
		t.Error("expected non-nil default exec")
	}
	if !strings.Contains(sysSched.getUnitsDir(), ".config/systemd/user") {
		t.Errorf("expected default unitsDir to contain .config/systemd/user, got %s", sysSched.getUnitsDir())
	}

	// 2. InstallWatch LookPath lookup and errors
	configDir := t.TempDir()
	unitsDir := t.TempDir()
	mock := NewMockExecutor()
	sched2 := NewSystemdSchedulerWithExecutor(configDir, mock, unitsDir)

	mock.Errors["lookpath:devclean"] = errors.New("devclean not in PATH")
	err := sched2.InstallWatch(WatchSchedule{Interval: 15 * time.Minute})
	if err == nil || !strings.Contains(err.Error(), "not found in PATH") {
		t.Fatalf("expected LookPath error, got %v", err)
	}
	delete(mock.Errors, "lookpath:devclean")

	// 3. InstallWatch daemon-reload failure
	mock.Errors["systemctl --user daemon-reload"] = errors.New("daemon-reload failed")
	err = sched2.InstallWatch(WatchSchedule{BinaryPath: "/bin/devclean", Interval: 15 * time.Minute})
	if err == nil || !strings.Contains(err.Error(), "daemon-reload failed") {
		t.Fatalf("expected daemon-reload error, got %v", err)
	}
	delete(mock.Errors, "systemctl --user daemon-reload")

	// 4. InstallWatch enable failure
	mock.Errors["systemctl --user enable --now devclean-watch.service"] = errors.New("enable failed")
	err = sched2.InstallWatch(WatchSchedule{BinaryPath: "/bin/devclean", Interval: 15 * time.Minute})
	if err == nil || !strings.Contains(err.Error(), "enable failed") {
		t.Fatalf("expected enable error, got %v", err)
	}
	delete(mock.Errors, "systemctl --user enable --now devclean-watch.service")

	// 5. Extraction edge cases
	if onCal := extractOnCalendar("some other content\nno match"); onCal != "" {
		t.Errorf("expected empty onCal, got %s", onCal)
	}
	if bin := extractBinaryFromService("some other content\nno match"); bin != "" {
		t.Errorf("expected empty bin, got %s", bin)
	}
}

package scheduler

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

var (
	systemdServiceTemplate = template.Must(template.New("service").Parse(`[Unit]
Description=DevClean Automated Cleanup Service
Documentation=https://github.com/gtkpad/devclean

[Service]
Type=oneshot
ExecStart={{.BinaryPath}} schedule run
`))

	systemdTimerTemplate = template.Must(template.New("timer").Parse(`[Unit]
Description=DevClean Automated Cleanup Timer
Documentation=https://github.com/gtkpad/devclean

[Timer]
OnCalendar={{.OnCalendar}}
Persistent=true
RandomizedDelaySec=5m

[Install]
WantedBy=timers.target
`))

	systemdWatchTemplate = template.Must(template.New("watch").Parse(`[Unit]
Description=DevClean Watch Daemon
Documentation=https://github.com/gtkpad/devclean

[Service]
Type=simple
ExecStart={{.BinaryPath}} --watch-daemon
Restart=on-failure

[Install]
WantedBy=default.target
`))
)

type systemdScheduler struct {
	configDir string
	unitsDir  string
	exec      CommandExecutor
}

// NewSystemdSchedulerWithExecutor creates a systemd scheduler with a custom executor and units directory.
func NewSystemdSchedulerWithExecutor(configDir string, exec CommandExecutor, unitsDir string) Scheduler {
	if exec == nil {
		exec = &osExecutor{}
	}
	return &systemdScheduler{
		configDir: configDir,
		unitsDir:  unitsDir,
		exec:      exec,
	}
}

func (s *systemdScheduler) getUnitsDir() string {
	if s.unitsDir != "" {
		return s.unitsDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user")
}

func (s *systemdScheduler) servicePath() string {
	return filepath.Join(s.getUnitsDir(), "devclean.service")
}

func (s *systemdScheduler) timerPath() string {
	return filepath.Join(s.getUnitsDir(), "devclean.timer")
}

func (s *systemdScheduler) watchPath() string {
	return filepath.Join(s.getUnitsDir(), "devclean-watch.service")
}

func (s *systemdScheduler) rollback(servicePath, timerPath string) {
	_ = os.Remove(servicePath)
	_ = os.Remove(timerPath)
	_, _ = s.exec.CombinedOutput(context.Background(), "systemctl", "--user", "daemon-reload")
}

func (s *systemdScheduler) Install(sched Schedule) error {
	if err := ValidateCron(sched.CronExpr); err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	binaryPath := sched.BinaryPath
	if binaryPath == "" {
		p, err := s.exec.LookPath("devclean")
		if err != nil {
			return fmt.Errorf("devclean binary not found in PATH: %w", err)
		}
		binaryPath = p
	}

	onCalendar, err := CronToOnCalendar(sched.CronExpr)
	if err != nil {
		return fmt.Errorf("converting cron to oncalendar: %w", err)
	}

	if s.IsInstalled() {
		return ErrAlreadyInstalled
	}

	unitsDir := s.getUnitsDir()
	if err := os.MkdirAll(unitsDir, 0755); err != nil {
		return err
	}

	// Generate devclean.service
	var serviceBuf bytes.Buffer
	if err := systemdServiceTemplate.Execute(&serviceBuf, map[string]string{
		"BinaryPath": binaryPath,
	}); err != nil {
		return fmt.Errorf("generating service unit: %w", err)
	}

	// Generate devclean.timer
	var timerBuf bytes.Buffer
	if err := systemdTimerTemplate.Execute(&timerBuf, map[string]string{
		"OnCalendar": onCalendar,
	}); err != nil {
		return fmt.Errorf("generating timer unit: %w", err)
	}

	spath := s.servicePath()
	tpath := s.timerPath()

	if err := os.WriteFile(spath, serviceBuf.Bytes(), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(tpath, timerBuf.Bytes(), 0644); err != nil {
		_ = os.Remove(spath)
		return err
	}

	// Reload systemctl daemon
	out, err := s.exec.CombinedOutput(context.Background(), "systemctl", "--user", "daemon-reload")
	if err != nil {
		s.rollback(spath, tpath)
		return fmt.Errorf("systemctl daemon-reload failed: %s: %w", strings.TrimSpace(string(out)), err)
	}

	// Enable and start timer
	out, err = s.exec.CombinedOutput(context.Background(), "systemctl", "--user", "enable", "--now", "devclean.timer")
	if err != nil {
		s.rollback(spath, tpath)
		return fmt.Errorf("systemctl enable failed: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return nil
}

func (s *systemdScheduler) Remove() error {
	if !s.IsInstalled() {
		return ErrNotInstalled
	}

	_, _ = s.exec.CombinedOutput(context.Background(), "systemctl", "--user", "disable", "--now", "devclean.timer")

	_ = os.Remove(s.servicePath())
	_ = os.Remove(s.timerPath())

	_, _ = s.exec.CombinedOutput(context.Background(), "systemctl", "--user", "daemon-reload")

	return nil
}

func (s *systemdScheduler) Status() (*Status, error) {
	active := s.IsInstalled()
	st := &Status{Active: active}

	if active {
		if timerData, err := os.ReadFile(s.timerPath()); err == nil {
			st.CronExpr = extractOnCalendar(string(timerData))
		}
		if serviceData, err := os.ReadFile(s.servicePath()); err == nil {
			st.BinaryPath = extractBinaryFromService(string(serviceData))
		}
	}

	lastRun, err := ReadLastRun(s.configDir)
	if err == nil && lastRun != nil {
		st.LastRun = &lastRun.Timestamp
		st.LastResult = lastRun
	}

	return st, nil
}

func (s *systemdScheduler) IsInstalled() bool {
	_, err := os.Stat(s.timerPath())
	return err == nil
}

func (s *systemdScheduler) InstallWatch(ws WatchSchedule) error {
	binaryPath := ws.BinaryPath
	if binaryPath == "" {
		p, err := s.exec.LookPath("devclean")
		if err != nil {
			return fmt.Errorf("devclean binary not found in PATH: %w", err)
		}
		binaryPath = p
	}

	wpath := s.watchPath()
	if _, err := os.Stat(wpath); err == nil {
		return ErrAlreadyInstalled
	}

	if err := os.MkdirAll(s.getUnitsDir(), 0755); err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := systemdWatchTemplate.Execute(&buf, map[string]string{
		"BinaryPath": binaryPath,
	}); err != nil {
		return fmt.Errorf("generating watch service: %w", err)
	}

	if err := os.WriteFile(wpath, buf.Bytes(), 0644); err != nil {
		return err
	}

	out, err := s.exec.CombinedOutput(context.Background(), "systemctl", "--user", "daemon-reload")
	if err != nil {
		_ = os.Remove(wpath)
		return fmt.Errorf("systemctl daemon-reload failed: %s: %w", strings.TrimSpace(string(out)), err)
	}

	out, err = s.exec.CombinedOutput(context.Background(), "systemctl", "--user", "enable", "--now", "devclean-watch.service")
	if err != nil {
		_ = os.Remove(wpath)
		_, _ = s.exec.CombinedOutput(context.Background(), "systemctl", "--user", "daemon-reload")
		return fmt.Errorf("systemctl enable failed: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return nil
}

func (s *systemdScheduler) RemoveWatch() error {
	wpath := s.watchPath()
	if _, err := os.Stat(wpath); os.IsNotExist(err) {
		return ErrNotInstalled
	}

	_, _ = s.exec.CombinedOutput(context.Background(), "systemctl", "--user", "disable", "--now", "devclean-watch.service")
	_ = os.Remove(wpath)
	_, _ = s.exec.CombinedOutput(context.Background(), "systemctl", "--user", "daemon-reload")
	return nil
}

func (s *systemdScheduler) IsWatchInstalled() bool {
	_, err := os.Stat(s.watchPath())
	return err == nil
}

func extractOnCalendar(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "OnCalendar=") {
			return strings.TrimPrefix(line, "OnCalendar=")
		}
	}
	return ""
}

func extractBinaryFromService(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ExecStart=") {
			rest := strings.TrimPrefix(line, "ExecStart=")
			parts := strings.Fields(rest)
			if len(parts) > 0 {
				return parts[0]
			}
		}
	}
	return ""
}

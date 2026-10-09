package scheduler

import (
	"context"
	"fmt"
	"strings"
)

type cronScheduler struct {
	configDir string
	exec      CommandExecutor
}

// NewCronSchedulerWithExecutor creates a Cron scheduler with custom executor.
func NewCronSchedulerWithExecutor(configDir string, exec CommandExecutor) Scheduler {
	if exec == nil {
		exec = &osExecutor{}
	}
	return &cronScheduler{
		configDir: configDir,
		exec:      exec,
	}
}

func (s *cronScheduler) readCrontab() string {
	out, err := s.exec.CombinedOutput(context.Background(), "crontab", "-l")
	if err != nil {
		return ""
	}
	return string(out)
}

func (s *cronScheduler) writeCrontab(content string) error {
	out, err := s.exec.CombinedOutputWithStdin(context.Background(), content, "crontab", "-")
	if err != nil {
		return fmt.Errorf("crontab failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (s *cronScheduler) Install(sched Schedule) error {
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

	existing := s.readCrontab()
	if strings.Contains(existing, cronMarker) {
		return ErrAlreadyInstalled
	}

	entry := fmt.Sprintf("\n%s\n%s %s clean --auto\n", cronMarker, sched.CronExpr, binaryPath)
	return s.writeCrontab(existing + entry)
}

func (s *cronScheduler) Remove() error {
	existing := s.readCrontab()
	if !strings.Contains(existing, cronMarker) {
		return ErrNotInstalled
	}

	filtered := removeCronEntry(existing)
	return s.writeCrontab(filtered)
}

func (s *cronScheduler) Status() (*Status, error) {
	existing := s.readCrontab()
	active := strings.Contains(existing, cronMarker)

	st := &Status{Active: active}

	if active {
		st.CronExpr = extractCronExpr(existing)
		st.BinaryPath = extractBinaryPath(existing)
	}

	lastRun, err := ReadLastRun(s.configDir)
	if err == nil && lastRun != nil {
		st.LastRun = &lastRun.Timestamp
		st.LastResult = lastRun
	}

	return st, nil
}

func (s *cronScheduler) IsInstalled() bool {
	return strings.Contains(s.readCrontab(), cronMarker)
}

const watchCronMarker = "# devclean-watch"

func (s *cronScheduler) InstallWatch(ws WatchSchedule) error {
	binaryPath := ws.BinaryPath
	if binaryPath == "" {
		p, err := s.exec.LookPath("devclean")
		if err != nil {
			return fmt.Errorf("devclean binary not found in PATH: %w", err)
		}
		binaryPath = p
	}

	existing := s.readCrontab()
	if strings.Contains(existing, watchCronMarker) {
		return ErrAlreadyInstalled
	}

	intervalMins := int(ws.Interval.Minutes())
	if intervalMins < 30 {
		intervalMins = 30
	}

	var cronExpr string
	if intervalMins >= 60 {
		hours := intervalMins / 60
		cronExpr = fmt.Sprintf("0 */%d * * *", hours)
	} else {
		cronExpr = fmt.Sprintf("*/%d * * * *", intervalMins)
	}

	entry := fmt.Sprintf("\n%s\n%s %s --watch-daemon\n", watchCronMarker, cronExpr, binaryPath)
	return s.writeCrontab(existing + entry)
}

func (s *cronScheduler) RemoveWatch() error {
	existing := s.readCrontab()
	if !strings.Contains(existing, watchCronMarker) {
		return ErrNotInstalled
	}

	lines := strings.Split(existing, "\n")
	var filtered []string
	skip := false
	for _, line := range lines {
		if line == watchCronMarker {
			skip = true
			continue
		}
		if skip {
			skip = false
			continue
		}
		filtered = append(filtered, line)
	}

	return s.writeCrontab(strings.Join(filtered, "\n"))
}

func (s *cronScheduler) IsWatchInstalled() bool {
	return strings.Contains(s.readCrontab(), watchCronMarker)
}

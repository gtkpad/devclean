//go:build linux

package scheduler

import (
	"fmt"
	"os/exec"
	"strings"
)

type cronScheduler struct {
	configDir string
}

// New creates a platform-specific Scheduler using crontab on Linux.
func New(configDir string) Scheduler {
	return &cronScheduler{configDir: configDir}
}

func (s *cronScheduler) Install(sched Schedule) error {
	if err := ValidateCron(sched.CronExpr); err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	binaryPath := sched.BinaryPath
	if binaryPath == "" {
		p, err := exec.LookPath("devclean")
		if err != nil {
			return fmt.Errorf("devclean binary not found in PATH: %w", err)
		}
		binaryPath = p
	}

	existing := readCrontab()
	if strings.Contains(existing, cronMarker) {
		return ErrAlreadyInstalled
	}

	entry := fmt.Sprintf("\n%s\n%s %s clean --auto\n", cronMarker, sched.CronExpr, binaryPath)
	newContent := existing + entry

	return writeCrontab(newContent)
}

func (s *cronScheduler) Remove() error {
	existing := readCrontab()
	if !strings.Contains(existing, cronMarker) {
		return ErrNotInstalled
	}

	filtered := removeCronEntry(existing)
	return writeCrontab(filtered)
}

func (s *cronScheduler) Status() (*Status, error) {
	existing := readCrontab()
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
	return strings.Contains(readCrontab(), cronMarker)
}

const watchCronMarker = "# devclean-watch"

func (s *cronScheduler) InstallWatch(ws WatchSchedule) error {
	binaryPath := ws.BinaryPath
	if binaryPath == "" {
		p, err := exec.LookPath("devclean")
		if err != nil {
			return fmt.Errorf("devclean binary not found in PATH: %w", err)
		}
		binaryPath = p
	}

	existing := readCrontab()
	if strings.Contains(existing, watchCronMarker) {
		return ErrAlreadyInstalled
	}

	// Convert interval to cron-compatible: run every N minutes
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
	newContent := existing + entry

	return writeCrontab(newContent)
}

func (s *cronScheduler) RemoveWatch() error {
	existing := readCrontab()
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

	return writeCrontab(strings.Join(filtered, "\n"))
}

func (s *cronScheduler) IsWatchInstalled() bool {
	return strings.Contains(readCrontab(), watchCronMarker)
}

func readCrontab() string {
	cmd := exec.Command("crontab", "-l")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func writeCrontab(content string) error {
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(content)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("crontab write failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

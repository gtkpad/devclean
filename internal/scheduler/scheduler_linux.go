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

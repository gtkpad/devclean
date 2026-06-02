//go:build linux

package scheduler

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

const cronMarker = "# devclean-scheduled"

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

func removeCronEntry(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	skip := false
	for _, line := range lines {
		if line == cronMarker {
			skip = true
			continue
		}
		if skip {
			skip = false
			continue
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

func extractCronExpr(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if line == cronMarker && i+1 < len(lines) {
			parts := strings.Fields(lines[i+1])
			if len(parts) >= 5 {
				return strings.Join(parts[:5], " ")
			}
		}
	}
	return ""
}

func extractBinaryPath(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if line == cronMarker && i+1 < len(lines) {
			parts := strings.Fields(lines[i+1])
			if len(parts) >= 6 {
				return parts[5]
			}
		}
	}
	return ""
}

// GenerateCronEntry is exported for testing crontab string manipulation.
func GenerateCronEntry(cronExpr, binaryPath string) string {
	var buf bytes.Buffer
	buf.WriteString("\n")
	buf.WriteString(cronMarker)
	buf.WriteString("\n")
	buf.WriteString(fmt.Sprintf("%s %s clean --auto\n", cronExpr, binaryPath))
	return buf.String()
}

// RemoveCronEntryFromContent is exported for testing removal logic.
func RemoveCronEntryFromContent(content string) string {
	return removeCronEntry(content)
}

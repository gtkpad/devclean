package scheduler

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

const (
	plistLabel      = "com.devclean.scheduled"
	watchPlistLabel = "com.devclean.watch"
)

type launchdScheduler struct {
	configDir       string
	launchAgentsDir string
	exec            CommandExecutor
}

// NewLaunchdSchedulerWithExecutor creates a Launchd scheduler with custom executor and directory.
func NewLaunchdSchedulerWithExecutor(configDir string, exec CommandExecutor, launchAgentsDir string) Scheduler {
	if exec == nil {
		exec = &osExecutor{}
	}
	return &launchdScheduler{
		configDir:       configDir,
		launchAgentsDir: launchAgentsDir,
		exec:            exec,
	}
}

func (s *launchdScheduler) plistPath() string {
	if s.launchAgentsDir != "" {
		return filepath.Join(s.launchAgentsDir, plistLabel+".plist")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", plistLabel+".plist")
}

func (s *launchdScheduler) watchPlistPath() string {
	if s.launchAgentsDir != "" {
		return filepath.Join(s.launchAgentsDir, watchPlistLabel+".plist")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", watchPlistLabel+".plist")
}

func (s *launchdScheduler) Install(sched Schedule) error {
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

	ppath := s.plistPath()
	if _, err := os.Stat(ppath); err == nil {
		return ErrAlreadyInstalled
	}

	fields, err := ParseCron(sched.CronExpr)
	if err != nil {
		return err
	}

	plistContent, err := generatePlist(binaryPath, fields)
	if err != nil {
		return fmt.Errorf("generating plist: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(ppath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(ppath, []byte(plistContent), 0644); err != nil {
		return err
	}

	out, err := s.exec.CombinedOutput(context.Background(), "launchctl", "load", ppath)
	if err != nil {
		_ = os.Remove(ppath)
		return fmt.Errorf("launchctl load failed: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return nil
}

func (s *launchdScheduler) Remove() error {
	ppath := s.plistPath()
	if _, err := os.Stat(ppath); os.IsNotExist(err) {
		return ErrNotInstalled
	}

	_, _ = s.exec.CombinedOutput(context.Background(), "launchctl", "unload", ppath)

	return os.Remove(ppath)
}

func (s *launchdScheduler) Status() (*Status, error) {
	ppath := s.plistPath()
	active := false
	if _, err := os.Stat(ppath); err == nil {
		active = true
	}

	st := &Status{Active: active}

	if active {
		data, err := os.ReadFile(ppath)
		if err == nil {
			st.CronExpr = extractCronFromPlist(string(data))
			st.BinaryPath = extractBinaryFromPlist(string(data))
		}
	}

	lastRun, err := ReadLastRun(s.configDir)
	if err == nil && lastRun != nil {
		st.LastRun = &lastRun.Timestamp
		st.LastResult = lastRun
	}

	return st, nil
}

func (s *launchdScheduler) IsInstalled() bool {
	_, err := os.Stat(s.plistPath())
	return err == nil
}

var plistTemplate = template.Must(template.New("plist").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{.Label}}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.BinaryPath}}</string>
		<string>clean</string>
		<string>--auto</string>
	</array>
	<key>StartCalendarInterval</key>
	<dict>
{{- range .CalendarKeys}}
		<key>{{.Key}}</key>
		<integer>{{.Value}}</integer>
{{- end}}
	</dict>
	<key>StandardOutPath</key>
	<string>/tmp/devclean-schedule.log</string>
	<key>StandardErrorPath</key>
	<string>/tmp/devclean-schedule.err</string>
	<key>RunAtLoad</key>
	<false/>
</dict>
</plist>
`))

type calendarKey struct {
	Key   string
	Value int
}

type plistData struct {
	Label        string
	BinaryPath   string
	CalendarKeys []calendarKey
}

func generatePlist(binaryPath string, fields CronFields) (string, error) {
	var keys []calendarKey

	if fields.Minute != "*" {
		v, _ := strconv.Atoi(fields.Minute)
		keys = append(keys, calendarKey{"Minute", v})
	}
	if fields.Hour != "*" {
		v, _ := strconv.Atoi(fields.Hour)
		keys = append(keys, calendarKey{"Hour", v})
	}
	if fields.DayOfMonth != "*" {
		v, _ := strconv.Atoi(fields.DayOfMonth)
		keys = append(keys, calendarKey{"Day", v})
	}
	if fields.Month != "*" {
		v, _ := strconv.Atoi(fields.Month)
		keys = append(keys, calendarKey{"Month", v})
	}
	if fields.DayOfWeek != "*" {
		v, _ := strconv.Atoi(fields.DayOfWeek)
		keys = append(keys, calendarKey{"Weekday", v})
	}

	data := plistData{
		Label:        plistLabel,
		BinaryPath:   binaryPath,
		CalendarKeys: keys,
	}

	var buf bytes.Buffer
	if err := plistTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func extractCronFromPlist(content string) string {
	return ""
}

func extractBinaryFromPlist(content string) string {
	idx := strings.Index(content, "<key>ProgramArguments</key>")
	if idx < 0 {
		return ""
	}
	rest := content[idx:]
	start := strings.Index(rest, "<string>")
	if start < 0 {
		return ""
	}
	start += len("<string>")
	end := strings.Index(rest[start:], "</string>")
	if end < 0 {
		return ""
	}
	return rest[start : start+end]
}

func (s *launchdScheduler) InstallWatch(ws WatchSchedule) error {
	binaryPath := ws.BinaryPath
	if binaryPath == "" {
		p, err := s.exec.LookPath("devclean")
		if err != nil {
			return fmt.Errorf("devclean binary not found in PATH: %w", err)
		}
		binaryPath = p
	}

	ppath := s.watchPlistPath()
	if _, err := os.Stat(ppath); err == nil {
		return ErrAlreadyInstalled
	}

	intervalSecs := int(ws.Interval.Seconds())
	if intervalSecs < 1800 {
		intervalSecs = 1800
	}

	home, _ := os.UserHomeDir()
	logPath := filepath.Join(home, ".local", "share", "devclean", "watch.log")

	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>--watch-daemon</string>
	</array>
	<key>StartInterval</key>
	<integer>%d</integer>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
	<key>RunAtLoad</key>
	<false/>
</dict>
</plist>
`, watchPlistLabel, binaryPath, intervalSecs, logPath, logPath)

	if err := os.MkdirAll(filepath.Dir(ppath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(ppath, []byte(content), 0644); err != nil {
		return err
	}

	out, err := s.exec.CombinedOutput(context.Background(), "launchctl", "load", ppath)
	if err != nil {
		_ = os.Remove(ppath)
		return fmt.Errorf("launchctl load failed: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return nil
}

func (s *launchdScheduler) RemoveWatch() error {
	ppath := s.watchPlistPath()
	if _, err := os.Stat(ppath); os.IsNotExist(err) {
		return ErrNotInstalled
	}

	_, _ = s.exec.CombinedOutput(context.Background(), "launchctl", "unload", ppath)

	return os.Remove(ppath)
}

func (s *launchdScheduler) IsWatchInstalled() bool {
	_, err := os.Stat(s.watchPlistPath())
	return err == nil
}

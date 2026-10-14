package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/scheduler"
)

func sampleLogContent() string {
	var sb strings.Builder
	// Run 1: OK
	sb.WriteString("=== [START] 2026-10-13 10:00:00 | version=v2.2 | args=[--preset safe] ===\n")
	sb.WriteString("Scanning caches...\n")
	sb.WriteString("Cleaning completed.\n")
	sb.WriteString("=== [END] 2026-10-13 10:00:05 | status=OK | targets=12 | freed=120.5 MB | duration=5.0s ===\n")

	// Run 2: PARTIAL
	sb.WriteString("=== [START] 2026-10-13 11:00:00 | version=v2.2 | args=[--preset standard] ===\n")
	sb.WriteString("Scanning caches...\n")
	sb.WriteString("Permission denied for /var/cache/sys\n")
	sb.WriteString("=== [END] 2026-10-13 11:00:08 | status=PARTIAL | targets=8 | freed=45.0 MB | duration=8.0s ===\n")

	// Run 3: FAIL
	sb.WriteString("=== [START] 2026-10-13 12:00:00 | version=v2.2 | args=[] ===\n")
	sb.WriteString("[ERROR] Fatal panic in cleaner\n")
	sb.WriteString("=== [END] 2026-10-13 12:00:02 | status=FAIL | targets=0 | freed=0 B | duration=2.0s ===\n")

	return sb.String()
}

func setupLogsTestEnv(t *testing.T) (string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmpDir, ".local", "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	resolver := platform.New()
	paths := resolver.Resolve()
	if err := os.MkdirAll(paths.LogDir, 0755); err != nil {
		t.Fatalf("creating log dir: %v", err)
	}
	if err := os.MkdirAll(paths.ConfigDir, 0755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	return paths.LogDir, paths.ConfigDir
}

func TestScheduleLogsCmd_EmptyState(t *testing.T) {
	_, _ = setupLogsTestEnv(t)

	var buf bytes.Buffer
	cmd := newScheduleCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"logs"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil error on empty state, got: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Nenhum log encontrado. O agendador ainda não executou.") {
		t.Errorf("output missing empty state message, got:\n%s", out)
	}
	if !strings.Contains(out, "devclean clean") {
		t.Errorf("output missing devclean clean instruction, got:\n%s", out)
	}
}

func TestScheduleLogsCmd_TableRender(t *testing.T) {
	logDir, _ := setupLogsTestEnv(t)

	logPath := filepath.Join(logDir, "schedule.log")
	if err := os.WriteFile(logPath, []byte(sampleLogContent()), 0644); err != nil {
		t.Fatalf("writing sample log: %v", err)
	}

	var buf bytes.Buffer
	cmd := newScheduleCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"logs"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("schedule logs failed: %v", err)
	}

	out := buf.String()

	// Table headers
	headers := []string{"DATA / HORA", "STATUS", "ALVOS", "LIBERADO", "DURAÇÃO"}
	for _, h := range headers {
		if !strings.Contains(out, h) {
			t.Errorf("table missing header %q, output:\n%s", h, out)
		}
	}

	// Badges
	badges := []string{"[SUCESSO]", "[PARCIAL]", "[FALHA]"}
	for _, b := range badges {
		if !strings.Contains(out, b) {
			t.Errorf("table missing badge %q, output:\n%s", b, out)
		}
	}

	// Recent execution detail
	if !strings.Contains(out, "Execução Mais Recente") {
		t.Errorf("output missing recent run detail section, got:\n%s", out)
	}

	// Footer tips
	if !strings.Contains(out, "--follow") || !strings.Contains(out, "--clear") || !strings.Contains(out, "--format json") {
		t.Errorf("output missing footer tips, got:\n%s", out)
	}
}

func TestScheduleLogsCmd_LinesFlag(t *testing.T) {
	logDir, _ := setupLogsTestEnv(t)

	// Create 5 runs
	var sb strings.Builder
	for i := 1; i <= 5; i++ {
		sb.WriteString(fmt.Sprintf("=== [START] 2026-10-13 %02d:00:00 | version=v2.2 | args=[] ===\n", i+10))
		sb.WriteString(fmt.Sprintf("Log line %d\n", i))
		sb.WriteString(fmt.Sprintf("=== [END] 2026-10-13 %02d:00:05 | status=OK | targets=%d | freed=%d.0 MB | duration=5.0s ===\n", i+10, i, i*10))
	}

	logPath := filepath.Join(logDir, "schedule.log")
	if err := os.WriteFile(logPath, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("writing sample log: %v", err)
	}

	var buf bytes.Buffer
	cmd := newScheduleCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"logs", "-n", "2"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("schedule logs -n 2 failed: %v", err)
	}

	out := buf.String()
	// Should contain run 4 and run 5 (14:00:00 and 15:00:00)
	if !strings.Contains(out, "14:00:00") || !strings.Contains(out, "15:00:00") {
		t.Errorf("expected runs 4 and 5 in output, got:\n%s", out)
	}
	// Should NOT contain run 1, 2, or 3 timestamps in table
	if strings.Contains(out, "11:00:00") || strings.Contains(out, "12:00:00") || strings.Contains(out, "13:00:00") {
		t.Errorf("expected runs 1-3 to be excluded with -n 2, got:\n%s", out)
	}
}

func TestScheduleLogsCmd_FormatJSON(t *testing.T) {
	logDir, _ := setupLogsTestEnv(t)

	logPath := filepath.Join(logDir, "schedule.log")
	if err := os.WriteFile(logPath, []byte(sampleLogContent()), 0644); err != nil {
		t.Fatalf("writing sample log: %v", err)
	}

	var buf bytes.Buffer
	cmd := newScheduleCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"logs", "--format", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("schedule logs --format json failed: %v", err)
	}

	var runs []scheduler.LogRun
	if err := json.Unmarshal(buf.Bytes(), &runs); err != nil {
		t.Fatalf("invalid json output: %v, raw output:\n%s", err, buf.String())
	}

	if len(runs) != 3 {
		t.Fatalf("expected 3 runs parsed in json, got %d", len(runs))
	}
	if runs[0].Status != "OK" || runs[1].Status != "PARTIAL" || runs[2].Status != "FAIL" {
		t.Errorf("unexpected statuses: %v, %v, %v", runs[0].Status, runs[1].Status, runs[2].Status)
	}
}

func TestScheduleLogsCmd_FormatJSON_Empty(t *testing.T) {
	_, _ = setupLogsTestEnv(t)

	var buf bytes.Buffer
	cmd := newScheduleCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"logs", "--format", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("schedule logs --format json on empty failed: %v", err)
	}

	trimmed := strings.TrimSpace(buf.String())
	if trimmed != "[]" {
		t.Errorf("expected empty JSON array '[]', got: %q", trimmed)
	}
}

func TestScheduleLogsCmd_ClearWithYes(t *testing.T) {
	logDir, _ := setupLogsTestEnv(t)

	// Create canonical log files and an unrelated file
	fnames := []string{"schedule.log", "schedule.1.log", "schedule.2.log", "schedule.3.log"}
	for _, fn := range fnames {
		if err := os.WriteFile(filepath.Join(logDir, fn), []byte("test log data"), 0644); err != nil {
			t.Fatalf("writing log %s: %v", fn, err)
		}
	}
	unrelated := filepath.Join(logDir, "other.txt")
	if err := os.WriteFile(unrelated, []byte("preserve me"), 0644); err != nil {
		t.Fatalf("writing unrelated: %v", err)
	}

	var buf bytes.Buffer
	cmd := newScheduleCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"logs", "--clear", "-y"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("schedule logs --clear -y failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "4 arquivo(s)") && !strings.Contains(out, "4") {
		t.Errorf("expected confirmation indicating 4 files removed, got: %s", out)
	}

	// Verify logs were removed
	for _, fn := range fnames {
		if _, err := os.Stat(filepath.Join(logDir, fn)); !os.IsNotExist(err) {
			t.Errorf("expected file %s to be removed, but it exists", fn)
		}
	}

	// Verify unrelated file preserved (T-03-07 whitelist protection)
	if _, err := os.Stat(unrelated); err != nil {
		t.Errorf("unrelated file should not have been removed: %v", err)
	}
}

func TestScheduleLogsCmd_ClearInteractiveConfirmed(t *testing.T) {
	logDir, _ := setupLogsTestEnv(t)

	logPath := filepath.Join(logDir, "schedule.log")
	if err := os.WriteFile(logPath, []byte("data"), 0644); err != nil {
		t.Fatalf("writing log: %v", err)
	}

	oldIsTerm := scheduleLogsIsTerminal
	oldStdin := scheduleLogsStdinReader
	defer func() {
		scheduleLogsIsTerminal = oldIsTerm
		scheduleLogsStdinReader = oldStdin
	}()

	scheduleLogsIsTerminal = func(fd int) bool { return true }
	scheduleLogsStdinReader = strings.NewReader("y\n")

	var buf bytes.Buffer
	cmd := newScheduleCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"logs", "--clear"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("schedule logs --clear interactive failed: %v", err)
	}

	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Errorf("expected schedule.log to be removed after confirming 'y'")
	}
}

func TestScheduleLogsCmd_ClearAborted(t *testing.T) {
	logDir, _ := setupLogsTestEnv(t)

	logPath := filepath.Join(logDir, "schedule.log")
	if err := os.WriteFile(logPath, []byte("data"), 0644); err != nil {
		t.Fatalf("writing log: %v", err)
	}

	oldIsTerm := scheduleLogsIsTerminal
	oldStdin := scheduleLogsStdinReader
	defer func() {
		scheduleLogsIsTerminal = oldIsTerm
		scheduleLogsStdinReader = oldStdin
	}()

	scheduleLogsIsTerminal = func(fd int) bool { return true }
	scheduleLogsStdinReader = strings.NewReader("n\n")

	var buf bytes.Buffer
	cmd := newScheduleCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"logs", "--clear"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("schedule logs --clear abort failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Operação cancelada pelo usuário.") {
		t.Errorf("expected cancel message, got: %s", out)
	}

	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("schedule.log should be intact when aborted: %v", err)
	}
}

func TestScheduleLogsCmd_ClearNonTTYWithoutYes(t *testing.T) {
	_, _ = setupLogsTestEnv(t)

	oldIsTerm := scheduleLogsIsTerminal
	defer func() {
		scheduleLogsIsTerminal = oldIsTerm
	}()
	scheduleLogsIsTerminal = func(fd int) bool { return false }

	cmd := newScheduleCmd()
	cmd.SetArgs([]string{"logs", "--clear"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when running --clear without TTY and without --yes")
	}
	if !strings.Contains(err.Error(), "confirmação interativa indisponível sem TTY") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestScheduleLogsCmd_FollowCanceled(t *testing.T) {
	logDir, _ := setupLogsTestEnv(t)

	logPath := filepath.Join(logDir, "schedule.log")
	if err := os.WriteFile(logPath, []byte("initial log line\n"), 0644); err != nil {
		t.Fatalf("writing log: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after brief delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	var buf bytes.Buffer
	cmd := newScheduleCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"logs", "-f"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error on clean context cancel, got: %v", err)
	}
}

type mockSchedulerWithStatus struct {
	status *scheduler.Status
}

func (m *mockSchedulerWithStatus) Install(s scheduler.Schedule) error { return nil }
func (m *mockSchedulerWithStatus) Remove() error                      { return nil }
func (m *mockSchedulerWithStatus) Status() (*scheduler.Status, error) { return m.status, nil }
func (m *mockSchedulerWithStatus) IsInstalled() bool                  { return m.status != nil && m.status.Active }
func (m *mockSchedulerWithStatus) InstallWatch(ws scheduler.WatchSchedule) error { return nil }
func (m *mockSchedulerWithStatus) RemoveWatch() error                 { return nil }
func (m *mockSchedulerWithStatus) IsWatchInstalled() bool             { return false }

func TestScheduleLogsCmd_EmptyState_WithActiveSchedule(t *testing.T) {
	_, _ = setupLogsTestEnv(t)

	next := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	oldFactory := schedulerFactory
	defer func() { schedulerFactory = oldFactory }()

	schedulerFactory = func(configDir, provider string) (scheduler.Scheduler, error) {
		return &mockSchedulerWithStatus{
			status: &scheduler.Status{
				Active:       true,
				CronExpr:     "0 12 * * *",
				NextEstimate: &next,
			},
		}, nil
	}

	var buf bytes.Buffer
	cmd := newScheduleCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"logs"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil error on active schedule empty state, got: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Status:      Ativo") {
		t.Errorf("expected 'Status: Ativo' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Frequência:  0 12 * * *") {
		t.Errorf("expected frequency in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Próxima:     2026-10-14 12:00:00") {
		t.Errorf("expected next run estimate in output, got:\n%s", out)
	}
}

func TestScheduleLogsCmd_FormatJSON_LinesFlag(t *testing.T) {
	logDir, _ := setupLogsTestEnv(t)

	logPath := filepath.Join(logDir, "schedule.log")
	if err := os.WriteFile(logPath, []byte(sampleLogContent()), 0644); err != nil {
		t.Fatalf("writing sample log: %v", err)
	}

	var buf bytes.Buffer
	cmd := newScheduleCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"logs", "--format", "json", "-n", "1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("schedule logs --format json -n 1 failed: %v", err)
	}

	var runs []scheduler.LogRun
	if err := json.Unmarshal(buf.Bytes(), &runs); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected exactly 1 run with -n 1, got %d", len(runs))
	}
	if runs[0].Status != "FAIL" {
		t.Errorf("expected latest run status FAIL, got: %v", runs[0].Status)
	}
}

func TestScheduleLogsCmd_UnsupportedFormat(t *testing.T) {
	_, _ = setupLogsTestEnv(t)

	cmd := newScheduleCmd()
	cmd.SetArgs([]string{"logs", "--format", "xml"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unsupported format 'xml'")
	}
	if !strings.Contains(err.Error(), "formato não suportado") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestScheduleLogsCmd_FollowError(t *testing.T) {
	logDir, _ := setupLogsTestEnv(t)

	logPath := filepath.Join(logDir, "schedule.log")
	if err := os.WriteFile(logPath, []byte("data\n"), 0644); err != nil {
		t.Fatalf("writing log: %v", err)
	}

	oldStream := streamLogFunc
	defer func() { streamLogFunc = oldStream }()

	streamLogFunc = func(ctx context.Context, p string, out io.Writer) error {
		return errors.New("simulated stream failure")
	}

	cmd := newScheduleCmd()
	cmd.SetArgs([]string{"logs", "-f"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when streamLogFunc fails")
	}
	if !strings.Contains(err.Error(), "simulated stream failure") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestScheduleLogsCmd_UnknownStatusBadge(t *testing.T) {
	logDir, _ := setupLogsTestEnv(t)

	logContent := "=== [START] 2026-10-13 10:00:00 | version=v2.2 | args=[] ===\n" +
		"some log\n" +
		"=== [END] 2026-10-13 10:00:05 | status=CUSTOM_STATUS | targets=1 | freed=10 B | duration=1s ===\n"

	logPath := filepath.Join(logDir, "schedule.log")
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatalf("writing log: %v", err)
	}

	var buf bytes.Buffer
	cmd := newScheduleCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"logs"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("schedule logs failed: %v", err)
	}

	if !strings.Contains(buf.String(), "[CUSTOM_STATUS]") {
		t.Errorf("expected [CUSTOM_STATUS] in output, got:\n%s", buf.String())
	}
}




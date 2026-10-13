package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtkpad/devclean/internal/platform"
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

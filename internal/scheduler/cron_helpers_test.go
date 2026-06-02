package scheduler

import (
	"strings"
	"testing"
)

func TestRemoveCronEntry_RemovesMarkerAndCommand(t *testing.T) {
	content := `# existing cron
0 */2 * * * /usr/bin/backup
# devclean-scheduled
0 3 * * * /usr/local/bin/devclean clean --auto
# another entry
30 12 * * 1 /usr/bin/report`

	result := removeCronEntry(content)

	if strings.Contains(result, cronMarker) {
		t.Error("result should not contain cronMarker")
	}
	if strings.Contains(result, "devclean") {
		t.Error("result should not contain devclean entry")
	}
	if !strings.Contains(result, "/usr/bin/backup") {
		t.Error("should preserve other entries")
	}
	if !strings.Contains(result, "/usr/bin/report") {
		t.Error("should preserve other entries")
	}
}

func TestRemoveCronEntry_NoMarker_ReturnsUnchanged(t *testing.T) {
	content := "0 * * * * /usr/bin/something\n"
	result := removeCronEntry(content)
	if result != content {
		t.Errorf("content changed when no marker present")
	}
}

func TestExtractCronExpr(t *testing.T) {
	content := `# some stuff
# devclean-scheduled
30 2 * * 0 /usr/local/bin/devclean clean --auto`

	got := extractCronExpr(content)
	want := "30 2 * * 0"
	if got != want {
		t.Errorf("extractCronExpr() = %q, want %q", got, want)
	}
}

func TestExtractCronExpr_NoMarker(t *testing.T) {
	content := "0 * * * * /usr/bin/something"
	got := extractCronExpr(content)
	if got != "" {
		t.Errorf("extractCronExpr() = %q, want empty", got)
	}
}

func TestExtractBinaryPath(t *testing.T) {
	content := `# devclean-scheduled
0 3 * * * /opt/bin/devclean clean --auto`

	got := extractBinaryPath(content)
	want := "/opt/bin/devclean"
	if got != want {
		t.Errorf("extractBinaryPath() = %q, want %q", got, want)
	}
}

func TestGenerateCronEntry_Format(t *testing.T) {
	entry := GenerateCronEntry("0 3 * * *", "/usr/local/bin/devclean")

	if !strings.Contains(entry, cronMarker) {
		t.Error("entry should contain cronMarker")
	}
	if !strings.Contains(entry, "0 3 * * * /usr/local/bin/devclean clean --auto") {
		t.Error("entry should contain the full cron line")
	}
}

func TestRemoveCronEntryFromContent_Exported(t *testing.T) {
	content := "# devclean-scheduled\n0 3 * * * /usr/local/bin/devclean clean --auto\n"
	result := RemoveCronEntryFromContent(content)
	if strings.Contains(result, "devclean") {
		t.Error("exported function should remove devclean entries")
	}
}

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtkpad/devclean/internal/tui"
)

func TestAnalyzeCmd_FlagsParsing(t *testing.T) {
	cmd := newAnalyzeCmd()
	args := []string{"--min-size", "10M", "--depth", "2", "--all", "--format", "json", "/tmp"}
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("unexpected flag parsing error: %v", err)
	}

	minSize, err := cmd.Flags().GetString("min-size")
	if err != nil || minSize != "10M" {
		t.Errorf("expected min-size 10M, got %q (err: %v)", minSize, err)
	}

	depth, err := cmd.Flags().GetInt("depth")
	if err != nil || depth != 2 {
		t.Errorf("expected depth 2, got %d (err: %v)", depth, err)
	}

	all, err := cmd.Flags().GetBool("all")
	if err != nil || !all {
		t.Errorf("expected all true, got %v (err: %v)", all, err)
	}

	format, err := cmd.Flags().GetString("format")
	if err != nil || format != "json" {
		t.Errorf("expected format json, got %q (err: %v)", format, err)
	}
}

func TestAnalyzeCmd_InvalidMinSize(t *testing.T) {
	cmd := newAnalyzeCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetArgs([]string{"--min-size", "invalid_size", "."})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid min-size, got nil")
	}
	if !strings.Contains(err.Error(), "invalid --min-size") {
		t.Errorf("expected error message to contain 'invalid --min-size', got: %v", err)
	}
}

func TestAnalyzeCmd_UnknownFormat(t *testing.T) {
	cmd := newAnalyzeCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetArgs([]string{"--format", "unsupported", "."})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unsupported format, got nil")
	}
	if !strings.Contains(err.Error(), "unknown format") {
		t.Errorf("expected error to contain 'unknown format', got: %v", err)
	}
}

func TestAnalyzeCmd_NonInteractiveTable(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "file.txt"), []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	oldIsTerm := analyzeIsTerminal
	analyzeIsTerminal = func(fd int) bool { return false }
	defer func() { analyzeIsTerminal = oldIsTerm }()

	cmd := newAnalyzeCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetArgs([]string{tmpDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}

	out := outBuf.String()
	if !strings.Contains(out, "TAMANHO") || !strings.Contains(out, "NOME") {
		t.Errorf("expected table headers in output, got: %s", out)
	}
	if !strings.Contains(out, "subdir") {
		t.Errorf("expected output to contain 'subdir', got: %s", out)
	}
}

func TestAnalyzeCmd_NonInteractiveJSON(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "data.bin")
	if err := os.WriteFile(testFile, []byte("some binary content here"), 0644); err != nil {
		t.Fatal(err)
	}
	// Also create a symlink to test symlink JSON representation
	_ = os.Symlink(testFile, filepath.Join(tmpDir, "link.bin"))

	cmd := newAnalyzeCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetArgs([]string{"--format", "json", tmpDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}

	out := outBuf.Bytes()
	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v\nOutput was: %s", err, string(out))
	}

	if _, ok := result["path"]; !ok {
		t.Errorf("expected JSON to have 'path' field: %v", result)
	}
	if _, ok := result["size_bytes"]; !ok {
		t.Errorf("expected JSON to have 'size_bytes' field: %v", result)
	}
	if children, ok := result["children"].([]interface{}); !ok || len(children) == 0 {
		t.Errorf("expected JSON to have non-empty 'children' array: %v", result)
	}
}

func TestAnalyzeCmd_Interactive(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	oldIsTerm := analyzeIsTerminal
	analyzeIsTerminal = func(fd int) bool { return true }
	defer func() { analyzeIsTerminal = oldIsTerm }()

	called := false
	oldRunProgram := analyzeRunProgram
	analyzeRunProgram = func(m tui.AnalyzerModel) error {
		called = true
		if m.CurrentDir() == nil {
			t.Errorf("expected non-nil CurrentDir")
		}
		return nil
	}
	defer func() { analyzeRunProgram = oldRunProgram }()

	cmd := newAnalyzeCmd()
	cmd.SetArgs([]string{tmpDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}

	if !called {
		t.Errorf("expected analyzeRunProgram to be invoked in interactive TTY mode")
	}
}

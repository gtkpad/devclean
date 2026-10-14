package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/tui"
)

func TestConfigCmd_NonTTYError(t *testing.T) {
	origIsTerminal := configIsTerminal
	defer func() { configIsTerminal = origIsTerminal }()

	configIsTerminal = func(fd int) bool { return false }

	cmd := newConfigCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when running config without TTY, got nil")
	}
	if !strings.Contains(err.Error(), "requires a terminal TTY") {
		t.Errorf("expected TTY error message, got: %v", err)
	}
}

func TestConfigCmd_InteractiveLaunch(t *testing.T) {
	origIsTerminal := configIsTerminal
	origRunProgram := configRunProgram
	origConfigPath := configPath
	defer func() {
		configIsTerminal = origIsTerminal
		configRunProgram = origRunProgram
		configPath = origConfigPath
	}()

	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte(config.DefaultConfigYAML), 0644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	configPath = cfgFile

	configIsTerminal = func(fd int) bool { return true }
	tuiLaunched := false
	configRunProgram = func(m tui.ConfigModel) error {
		tuiLaunched = true
		if m.ActiveTab() != tui.TabPaths {
			t.Errorf("expected TabPaths initial tab, got %v", m.ActiveTab())
		}
		return nil
	}

	cmd := newConfigCmd()
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error executing config: %v", err)
	}
	if !tuiLaunched {
		t.Errorf("expected TUI program to be launched")
	}
}

func TestConfigCmd_CorruptedConfigExit(t *testing.T) {
	origIsTerminal := configIsTerminal
	origRunProgram := configRunProgram
	origConfigPath := configPath
	defer func() {
		configIsTerminal = origIsTerminal
		configRunProgram = origRunProgram
		configPath = origConfigPath
	}()

	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yaml")
	corruptedYAML := []byte("scan_paths:\n  - [unclosed bracket\n    bad: syntax: :\n")
	if err := os.WriteFile(cfgFile, corruptedYAML, 0644); err != nil {
		t.Fatalf("writing corrupted config: %v", err)
	}
	configPath = cfgFile

	configIsTerminal = func(fd int) bool { return true }
	tuiLaunched := false
	configRunProgram = func(m tui.ConfigModel) error {
		tuiLaunched = true
		return nil
	}

	cmd := newConfigCmd()
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error executing config with corrupted YAML, got nil")
	}
	if tuiLaunched {
		t.Errorf("expected TUI program NOT to be launched on corrupted config")
	}
	if !strings.Contains(err.Error(), "YAML") && !strings.Contains(err.Error(), "syntax") && !strings.Contains(err.Error(), "line") {
		t.Errorf("expected syntax error diagnostic, got: %v", err)
	}
}

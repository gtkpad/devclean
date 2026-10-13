package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/installer"
)

func createInstallerConfigFile(t *testing.T, tmpDir string, installerPaths ...string) string {
	t.Helper()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	var pathsYAML strings.Builder
	for _, p := range installerPaths {
		pathsYAML.WriteString(fmt.Sprintf("    - %q\n", p))
	}
	content := fmt.Sprintf(`installer:
  paths:
%s  max_depth: 2
safety:
  trash_enabled: true
`, pathsYAML.String())
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}

func createDummyInstaller(t *testing.T, dir, filename string, age time.Duration) string {
	t.Helper()
	p := filepath.Join(dir, filename)
	if err := os.WriteFile(p, []byte("fake-installer-content-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	modTime := time.Now().Add(-age)
	if err := os.Chtimes(p, modTime, modTime); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestInstallerCmd_FlagValidation(t *testing.T) {
	origConfigPath := configPath
	origQuiet := quiet
	defer func() {
		configPath = origConfigPath
		quiet = origQuiet
	}()

	tmpDir := t.TempDir()
	configPath = createInstallerConfigFile(t, tmpDir)

	tests := []struct {
		name        string
		args        []string
		errContains string
	}{
		{
			name:        "MutuallyExclusiveTrashAndPermanent",
			args:        []string{"--trash", "--permanent"},
			errContains: "--trash and --permanent are mutually exclusive",
		},
		{
			name:        "InvalidFormat",
			args:        []string{"--format", "yaml"},
			errContains: "unknown format",
		},
		{
			name:        "InvalidMinAge",
			args:        []string{"--min-age", "not-a-duration"},
			errContains: "invalid --min-age",
		},
		{
			name:        "InvalidMinSize",
			args:        []string{"--min-size", "bad-size"},
			errContains: "invalid --min-size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newInstallerCmd()
			var outBuf bytes.Buffer
			cmd.SetOut(&outBuf)
			cmd.SetErr(&outBuf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.errContains)
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("expected error %q to contain %q", err.Error(), tt.errContains)
			}
		})
	}
}

func TestInstallerCmd_DryRun(t *testing.T) {
	origConfigPath := configPath
	origQuiet := quiet
	defer func() {
		configPath = origConfigPath
		quiet = origQuiet
	}()

	tmpDir := t.TempDir()
	configPath = createInstallerConfigFile(t, tmpDir, tmpDir)
	installerFile := createDummyInstaller(t, tmpDir, "sample.dmg", 48*time.Hour)

	cmd := newInstallerCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetArgs([]string{"--dry-run", "--path", tmpDir, "--min-age", "0"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error executing dry-run: %v", err)
	}

	// File must NOT be deleted
	if _, err := os.Stat(installerFile); os.IsNotExist(err) {
		t.Fatalf("expected file %s to still exist in dry-run mode, but it was deleted", installerFile)
	}

	out := outBuf.String()
	if !strings.Contains(out, "sample.dmg") {
		t.Errorf("expected dry-run output to list sample.dmg, got: %s", out)
	}
	if !strings.Contains(out, "Total:") {
		t.Errorf("expected dry-run output to include Total summary, got: %s", out)
	}
}

func TestInstallerCmd_JsonOutput(t *testing.T) {
	origConfigPath := configPath
	origQuiet := quiet
	defer func() {
		configPath = origConfigPath
		quiet = origQuiet
	}()

	tmpDir := t.TempDir()
	configPath = createInstallerConfigFile(t, tmpDir, tmpDir)
	installerFile := createDummyInstaller(t, tmpDir, "test.pkg", 48*time.Hour)

	cmd := newInstallerCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetArgs([]string{"--format", "json", "--dry-run", "--path", tmpDir, "--min-age", "0"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var jsonResult struct {
		Candidates     []installer.Candidate `json:"candidates"`
		TotalFiles     int                   `json:"total_files"`
		TotalSizeBytes int64                 `json:"total_size_bytes"`
	}

	if err := json.Unmarshal(outBuf.Bytes(), &jsonResult); err != nil {
		t.Fatalf("failed to parse JSON output: %v, raw output: %s", err, outBuf.String())
	}

	if jsonResult.TotalFiles != 1 {
		t.Errorf("expected total_files 1, got %d", jsonResult.TotalFiles)
	}
	if len(jsonResult.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(jsonResult.Candidates))
	}
	if jsonResult.Candidates[0].Path != installerFile {
		t.Errorf("expected candidate path %s, got %s", installerFile, jsonResult.Candidates[0].Path)
	}
}

func TestInstallerCmd_InteractiveConfirm(t *testing.T) {
	origConfigPath := configPath
	origQuiet := quiet
	origStdinReader := installerStdinReader
	origIsTerminal := installerIsTerminal
	defer func() {
		configPath = origConfigPath
		quiet = origQuiet
		installerStdinReader = origStdinReader
		installerIsTerminal = origIsTerminal
	}()

	tmpDir := t.TempDir()
	configPath = createInstallerConfigFile(t, tmpDir, tmpDir)
	installerFile := createDummyInstaller(t, tmpDir, "setup.exe", 48*time.Hour)

	// Simulate interactive terminal and "y\n" user input
	installerIsTerminal = func(fd int) bool { return true }
	installerStdinReader = strings.NewReader("y\n")

	cmd := newInstallerCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetArgs([]string{"--path", tmpDir, "--min-age", "0"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := outBuf.String()
	if !strings.Contains(out, "Deseja mover os instaladores selecionados para a lixeira? [y/N]:") {
		t.Errorf("expected confirmation prompt in output, got: %s", out)
	}

	// File should be moved/removed from original location
	if _, err := os.Stat(installerFile); !os.IsNotExist(err) {
		t.Errorf("expected %s to be removed from original location after confirm, but still exists", installerFile)
	}
}

func TestInstallerCmd_InteractiveCancel(t *testing.T) {
	origConfigPath := configPath
	origQuiet := quiet
	origStdinReader := installerStdinReader
	origIsTerminal := installerIsTerminal
	defer func() {
		configPath = origConfigPath
		quiet = origQuiet
		installerStdinReader = origStdinReader
		installerIsTerminal = origIsTerminal
	}()

	tmpDir := t.TempDir()
	configPath = createInstallerConfigFile(t, tmpDir, tmpDir)
	installerFile := createDummyInstaller(t, tmpDir, "app.iso", 48*time.Hour)

	// Simulate interactive terminal and "n\n" user input (cancel)
	installerIsTerminal = func(fd int) bool { return true }
	installerStdinReader = strings.NewReader("n\n")

	cmd := newInstallerCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetArgs([]string{"--path", tmpDir, "--min-age", "0"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := outBuf.String()
	if !strings.Contains(out, "Operação cancelada pelo usuário.") {
		t.Errorf("expected cancellation message, got: %s", out)
	}

	// File must still exist
	if _, err := os.Stat(installerFile); os.IsNotExist(err) {
		t.Errorf("expected file %s to still exist after cancellation, but it was deleted", installerFile)
	}
}

func TestInstallerCmd_NonTTYError(t *testing.T) {
	origConfigPath := configPath
	origQuiet := quiet
	origIsTerminal := installerIsTerminal
	defer func() {
		configPath = origConfigPath
		quiet = origQuiet
		installerIsTerminal = origIsTerminal
	}()

	tmpDir := t.TempDir()
	configPath = createInstallerConfigFile(t, tmpDir, tmpDir)
	installerFile := createDummyInstaller(t, tmpDir, "pkg.deb", 48*time.Hour)

	// Simulate non-TTY environment
	installerIsTerminal = func(fd int) bool { return false }

	cmd := newInstallerCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetArgs([]string{"--path", tmpDir, "--min-age", "0"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error without TTY and without --yes, got nil")
	}

	expectedErr := "confirmação interativa indisponível sem TTY; use --yes para prosseguir"
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}

	// File must still exist
	if _, err := os.Stat(installerFile); os.IsNotExist(err) {
		t.Errorf("expected file %s to still exist after non-TTY abort, but it was deleted", installerFile)
	}
}

func TestInstallerCmd_BlacklistProtection(t *testing.T) {
	origConfigPath := configPath
	origQuiet := quiet
	defer func() {
		configPath = origConfigPath
		quiet = origQuiet
	}()

	tmpDir := t.TempDir()
	configPath = createInstallerConfigFile(t, tmpDir, tmpDir)

	// Place installer inside a .git directory (immutable blacklist)
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	protectedInstaller := createDummyInstaller(t, gitDir, "protected.dmg", 48*time.Hour)

	cmd := newInstallerCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetArgs([]string{"--yes", "--path", tmpDir, "--min-age", "0"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := outBuf.String()
	if !strings.Contains(out, "Nenhum instalador órfão encontrado.") {
		t.Errorf("expected 'Nenhum instalador órfão encontrado.' message, got: %s", out)
	}

	// Protected installer must NOT be deleted
	if _, err := os.Stat(protectedInstaller); os.IsNotExist(err) {
		t.Fatalf("expected protected installer in .git to remain intact, but it was deleted")
	}
}

func TestInstallerCmd_Permanent(t *testing.T) {
	origConfigPath := configPath
	origQuiet := quiet
	defer func() {
		configPath = origConfigPath
		quiet = origQuiet
	}()

	tmpDir := t.TempDir()
	configPath = createInstallerConfigFile(t, tmpDir, tmpDir)
	installerFile := createDummyInstaller(t, tmpDir, "permanent.dmg", 48*time.Hour)

	cmd := newInstallerCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetArgs([]string{"--permanent", "--yes", "--path", tmpDir, "--min-age", "0"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error executing permanent deletion: %v", err)
	}

	if _, err := os.Stat(installerFile); !os.IsNotExist(err) {
		t.Errorf("expected %s to be permanently deleted, but it still exists", installerFile)
	}

	out := outBuf.String()
	if !strings.Contains(out, "removido(s) permanentemente") {
		t.Errorf("expected permanent deletion message in output, got: %s", out)
	}
}

func TestInstallerCmd_ExplicitTrash(t *testing.T) {
	origConfigPath := configPath
	origQuiet := quiet
	defer func() {
		configPath = origConfigPath
		quiet = origQuiet
	}()

	tmpDir := t.TempDir()
	configPath = createInstallerConfigFile(t, tmpDir, tmpDir)
	installerFile := createDummyInstaller(t, tmpDir, "trashme.dmg", 48*time.Hour)

	cmd := newInstallerCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetArgs([]string{"--trash", "--yes", "--path", tmpDir, "--min-age", "0"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error executing explicit trash: %v", err)
	}

	if _, err := os.Stat(installerFile); !os.IsNotExist(err) {
		t.Errorf("expected %s to be moved to trash, but it still exists at source", installerFile)
	}
}

func TestInstallerCmd_NoCandidatesJson(t *testing.T) {
	origConfigPath := configPath
	origQuiet := quiet
	defer func() {
		configPath = origConfigPath
		quiet = origQuiet
	}()

	tmpDir := t.TempDir()
	configPath = createInstallerConfigFile(t, tmpDir, tmpDir)

	cmd := newInstallerCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetArgs([]string{"--format", "json", "--path", tmpDir, "--min-age", "0"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var jsonResult struct {
		Candidates     []installer.Candidate `json:"candidates"`
		TotalFiles     int                   `json:"total_files"`
		TotalSizeBytes int64                 `json:"total_size_bytes"`
	}

	if err := json.Unmarshal(outBuf.Bytes(), &jsonResult); err != nil {
		t.Fatalf("failed to parse empty JSON output: %v, raw: %s", err, outBuf.String())
	}
	if jsonResult.TotalFiles != 0 {
		t.Errorf("expected 0 files, got %d", jsonResult.TotalFiles)
	}
}

func TestInstallerCleanerProvider(t *testing.T) {
	cleaner := installer.NewCleaner(installer.Options{})
	provider := &installerCleanerProvider{cleaner: cleaner}

	if got := provider.GetCleaner("installer"); got != cleaner {
		t.Errorf("expected installer cleaner, got %v", got)
	}
	if got := provider.GetCleaner("other"); got != nil {
		t.Errorf("expected nil for other, got %v", got)
	}
}


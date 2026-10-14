package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gtkpad/devclean/internal/analyzer"
	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/scheduler"
	"github.com/gtkpad/devclean/internal/trash"
	"github.com/gtkpad/devclean/internal/tui"
)

// setupV22GitRepo initializes a git repo for inactive repo testing.
func setupV22GitRepo(t *testing.T, dir string, branch string, daysAgo int) {
	t.Helper()
	runGit := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %s failed: %v\noutput: %s", strings.Join(args, " "), err, string(out))
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test")
	runGit("checkout", "-b", branch)

	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "README.md")

	commitDate := time.Now().Add(-time.Duration(daysAgo) * 24 * time.Hour).Format(time.RFC3339)
	c := exec.Command("git", "commit", "-m", "initial commit")
	c.Dir = dir
	c.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_AUTHOR_DATE="+commitDate,
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_COMMITTER_DATE="+commitDate,
	)
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v\noutput: %s", err, string(out))
	}
}

// Flow 1: User configures custom paths and ecosystems via TUI (Phase 5) -> runs clean with preset (Phase 1)
func TestE2E_Flow1_ConfigTUI_To_CleanPresets(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	configDir := filepath.Join(homeDir, ".config", "devclean")
	workspace := filepath.Join(tmpDir, "workspace")
	trashDir := filepath.Join(homeDir, ".local", "share", "devclean", "trash")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(workspace, 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(homeDir, ".local", "share"))

	// Toolchain indicators so IsInstalled() detects ecosystem presence
	_ = os.MkdirAll(filepath.Join(homeDir, ".pub-cache"), 0755)
	_ = os.MkdirAll(filepath.Join(homeDir, ".conan"), 0755)

	// Create Phase 1 Ecosystem fixtures
	// 1. Flutter fixture (.dart_tool)
	flutterProj := filepath.Join(workspace, "my_flutter_app")
	flutterTool := filepath.Join(flutterProj, ".dart_tool")
	if err := os.MkdirAll(flutterTool, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(flutterProj, "pubspec.yaml"), []byte("name: my_flutter_app\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(flutterTool, "package_config.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. C/C++ CMake fixture
	cppProj := filepath.Join(workspace, "my_cpp_app")
	cmakeBuild := filepath.Join(cppProj, "build")
	if err := os.MkdirAll(cmakeBuild, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cppProj, "CMakeLists.txt"), []byte("cmake_minimum_required(VERSION 3.10)\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmakeBuild, "CMakeCache.txt"), []byte("# CMakeCache\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Terraform fixture
	tfProj := filepath.Join(workspace, "my_terraform")
	tfDir := filepath.Join(tfProj, ".terraform")
	if err := os.MkdirAll(tfDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tfProj, "main.tf"), []byte(`terraform {}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tfDir, "terraform.tfstate"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Initialize config.yaml using AST mutator first run
	cfgPath := filepath.Join(configDir, "config.yaml")
	node, cfg, err := config.LoadWithAST(cfgPath)
	if err != nil {
		t.Fatalf("LoadWithAST failed: %v", err)
	}
	cfg.Safety.TrashEnabled = true
	cfg.Safety.TrashDir = trashDir

	bl := safety.NewBlacklist(cfg.ProtectedPatterns)
	m := tui.NewConfigModel(cfg, node, cfgPath, nil, bl)

	// Simulate user typing workspace path into TUI
	addMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}
	newM, _ := m.Update(addMsg)
	m = newM.(tui.ConfigModel)

	// Now in StateAddingPath, input path
	for _, r := range workspace {
		newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = newM.(tui.ConfigModel)
	}
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(tui.ConfigModel)

	// User saves configuration in TUI
	saveMsg := tea.KeyMsg{Type: tea.KeyCtrlS}
	newM, _ = m.Update(saveMsg)
	m = newM.(tui.ConfigModel)

	// Verify config.yaml was written with workspace in scan_paths
	savedContent, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading saved config.yaml: %v", err)
	}
	if !strings.Contains(string(savedContent), workspace) {
		t.Fatalf("saved config does not contain workspace %s:\n%s", workspace, string(savedContent))
	}

	origConfigPath := configPath
	origQuiet := quiet
	configPath = cfgPath
	quiet = true
	defer func() {
		configPath = origConfigPath
		quiet = origQuiet
	}()

	// Test preset "safe": safe preset must ignore local build directories
	safeCleanCmd := newCleanCmd()
	safeCleanCmd.SetArgs([]string{"--preset", "safe"})
	var safeBuf bytes.Buffer
	safeCleanCmd.SetOut(&safeBuf)
	safeCleanCmd.SetErr(&safeBuf)
	err = safeCleanCmd.Execute()
	if err != nil && !strings.Contains(err.Error(), "nothing to clean") {
		t.Fatalf("unexpected error with --preset safe: %v", err)
	}

	// Verify local build files still exist after --preset safe
	if _, err := os.Stat(filepath.Join(flutterTool, "package_config.json")); os.IsNotExist(err) {
		t.Errorf("expected flutter .dart_tool to be preserved under --preset safe")
	}
	if _, err := os.Stat(filepath.Join(cmakeBuild, "CMakeCache.txt")); os.IsNotExist(err) {
		t.Errorf("expected cmake build to be preserved under --preset safe")
	}

	// Test preset "aggressive": aggressive preset forces TTL=0 and removes local builds to trash
	aggrCleanCmd := newCleanCmd()
	aggrCleanCmd.SetArgs([]string{"--preset", "aggressive", "--trash"})
	var aggrBuf bytes.Buffer
	aggrCleanCmd.SetOut(&aggrBuf)
	aggrCleanCmd.SetErr(&aggrBuf)
	if err := aggrCleanCmd.Execute(); err != nil {
		t.Fatalf("aggressive clean failed: %v", err)
	}

	// Verify local build targets were moved to trash
	if _, err := os.Stat(filepath.Join(flutterTool, "package_config.json")); !os.IsNotExist(err) {
		t.Errorf("expected flutter .dart_tool to be cleaned under --preset aggressive")
	}
	if _, err := os.Stat(filepath.Join(cmakeBuild, "CMakeCache.txt")); !os.IsNotExist(err) {
		t.Errorf("expected cmake build to be cleaned under --preset aggressive")
	}
}

// Flow 2: User finds installer (Phase 2) -> deletes to trash -> runs trash clean with retention (Phase 2)
func TestE2E_Flow2_Installer_To_TrashRetention(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	configDir := filepath.Join(homeDir, ".config", "devclean")
	downloadsDir := filepath.Join(homeDir, "Downloads")
	trashDir := filepath.Join(homeDir, ".local", "share", "devclean", "trash")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(downloadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(homeDir, ".local", "share"))

	// Create dummy old installers
	installer1 := filepath.Join(downloadsDir, "vscode-installer.dmg")
	installer2 := filepath.Join(downloadsDir, "node-v20.pkg")
	installer3 := filepath.Join(downloadsDir, "ubuntu-24.04.iso")
	dummyContent := make([]byte, 50*1024) // 50 KB each

	for _, file := range []string{installer1, installer2, installer3} {
		if err := os.WriteFile(file, dummyContent, 0644); err != nil {
			t.Fatal(err)
		}
		// Set mod time to 15 days ago
		past := time.Now().Add(-15 * 24 * time.Hour)
		_ = os.Chtimes(file, past, past)
	}

	cfgPath := filepath.Join(configDir, "config.yaml")
	cfgContent := fmt.Sprintf(`safety:
  trash_enabled: true
  trash_dir: %q
  retention_days: 7
  max_size_mb: 100
installer:
  paths:
    - %q
  max_depth: 2
`, trashDir, downloadsDir)
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	origConfigPath := configPath
	origQuiet := quiet
	configPath = cfgPath
	quiet = true
	defer func() {
		configPath = origConfigPath
		quiet = origQuiet
	}()

	// 1. Run installer command with --yes and --trash
	instCmd := newInstallerCmd()
	instCmd.SetArgs([]string{"--yes", "--trash", "--path", downloadsDir})
	var instBuf bytes.Buffer
	instCmd.SetOut(&instBuf)
	instCmd.SetErr(&instBuf)
	if err := instCmd.Execute(); err != nil {
		t.Fatalf("installer execution failed: %v", err)
	}

	// Verify installers were moved out of Downloads
	for _, file := range []string{installer1, installer2, installer3} {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Errorf("expected installer file %s to be moved to trash", file)
		}
	}

	// 2. Verify trash manifest has the batch
	trashCfg := config.SafetyConfig{
		TrashEnabled:  true,
		TrashDir:      trashDir,
		RetentionDays: 7,
		MaxSizeMB:     100,
	}
	trashMgr := trash.New(trashCfg)
	batches, err := trashMgr.List(context.Background())
	if err != nil {
		t.Fatalf("trashMgr.List failed: %v", err)
	}
	if len(batches) == 0 {
		t.Fatalf("expected at least 1 trash batch after installer delete, got 0")
	}

	// 3. Test devclean trash list JSON output
	listCmd := newTrashListCmd()
	listCmd.SetArgs([]string{"--format", "json"})
	var listBuf bytes.Buffer
	listCmd.SetOut(&listBuf)
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("trash list failed: %v", err)
	}
	var listResp struct {
		TotalBatches int `json:"total_batches"`
	}
	if err := json.Unmarshal(listBuf.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshaling trash list json: %v", err)
	}
	if listResp.TotalBatches < 1 {
		t.Errorf("expected total_batches >= 1, got %d", listResp.TotalBatches)
	}

	// 4. Manually expire the batch to test retention cleanup
	manifestPath := trashMgr.ManifestPath()
	manifest, err := trash.LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("loading manifest: %v", err)
	}
	for i := range manifest.Batches {
		manifest.Batches[i].CreatedAt = time.Now().Add(-20 * 24 * time.Hour)
		for j := range manifest.Batches[i].Entries {
			manifest.Batches[i].Entries[j].MovedAt = time.Now().Add(-20 * 24 * time.Hour)
		}
	}
	if err := trash.SaveManifest(manifestPath, manifest); err != nil {
		t.Fatalf("saving updated manifest: %v", err)
	}

	// 5. Run devclean trash clean (expired by default)
	cleanTrashCmd := newTrashCleanCmd()
	cleanTrashCmd.SetArgs([]string{"--format", "json"})
	var cleanTrashBuf bytes.Buffer
	cleanTrashCmd.SetOut(&cleanTrashBuf)
	if err := cleanTrashCmd.Execute(); err != nil {
		t.Fatalf("trash clean failed: %v", err)
	}

	var stats trash.CleanStats
	if err := json.Unmarshal(cleanTrashBuf.Bytes(), &stats); err != nil {
		t.Fatalf("unmarshaling trash clean stats: %v", err)
	}
	if stats.BatchesPurged < 1 {
		t.Errorf("expected at least 1 expired batch purged, got %d", stats.BatchesPurged)
	}
}

// Flow 3: User schedules weekly clean via CLI (Phase 3) -> schedule run executes -> schedule logs inspects runs (Phase 3)
func TestE2E_Flow3_SchedulerWeekly_Run_LogsAudit(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	configDir := filepath.Join(homeDir, ".config", "devclean")
	logDir := filepath.Join(homeDir, "Library", "Logs", "devclean")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))

	cfgPath := filepath.Join(configDir, "config.yaml")
	cfgContent := `scheduler:
  args: []
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	origConfigPath := configPath
	configPath = cfgPath
	defer func() {
		configPath = origConfigPath
	}()

	// Mock schedulerFactory with in-memory scheduler
	origFactory := schedulerFactory
	mockSched := &mockScheduler{}
	schedulerFactory = func(cfgDir, provider string) (scheduler.Scheduler, error) {
		return mockSched, nil
	}
	defer func() {
		schedulerFactory = origFactory
	}()

	// 1. devclean schedule --weekly --at 03:00 --preset safe --skip-validation
	schedCmd := newScheduleCmd()
	schedCmd.SetArgs([]string{"--weekly", "--at", "03:00", "--preset", "safe", "--skip-validation"})
	var schedBuf bytes.Buffer
	schedCmd.SetOut(&schedBuf)
	if err := schedCmd.Execute(); err != nil {
		t.Fatalf("schedule command failed: %v", err)
	}

	// Verify schedule was installed with 0 3 * * 0
	if !mockSched.isInstalled {
		t.Errorf("expected scheduler to be installed")
	}
	if mockSched.installed.CronExpr != "0 3 * * 0" {
		t.Errorf("expected cron 0 3 * * 0, got %s", mockSched.installed.CronExpr)
	}

	// Verify config.yaml has --preset safe in scheduler.args
	updatedCfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("loading updated config: %v", err)
	}
	foundPreset := false
	for i, arg := range updatedCfg.Scheduler.Args {
		if arg == "--preset" && i+1 < len(updatedCfg.Scheduler.Args) && updatedCfg.Scheduler.Args[i+1] == "safe" {
			foundPreset = true
			break
		}
	}
	if !foundPreset {
		t.Errorf("expected scheduler.args to contain '--preset safe', got: %v", updatedCfg.Scheduler.Args)
	}

	// 2. Execute devclean schedule run
	origRunCleanSubprocess := runCleanSubprocess
	runCleanSubprocess = func(ctx context.Context, configDir string, args []string, logWriter io.Writer) cleanExecutionResult {
		fmt.Fprintf(logWriter, "Espaço Recuperado: 128 MB\nAlvos Processados: 12\n")
		return cleanExecutionResult{
			ExitCode:     0,
			Output:       "Espaço Recuperado: 128 MB\nAlvos Processados: 12\n",
			TargetsCount: 12,
			BytesFreed:   128 * 1024 * 1024,
			FreedHuman:   "128 MB",
			Status:       "OK",
		}
	}
	defer func() {
		runCleanSubprocess = origRunCleanSubprocess
	}()

	runCmd := newScheduleRunCmd()
	var runBuf bytes.Buffer
	runCmd.SetOut(&runBuf)
	runCmd.SetErr(&runBuf)
	if err := runCmd.Execute(); err != nil {
		t.Fatalf("schedule run failed: %v", err)
	}

	// Verify schedule.log contains START and END markers
	logPath := filepath.Join(logDir, "schedule.log")
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading schedule.log: %v", err)
	}
	logStr := string(logData)
	if !strings.Contains(logStr, "=== [START]") || !strings.Contains(logStr, "=== [END]") {
		t.Errorf("expected schedule.log to have START and END markers, got:\n%s", logStr)
	}
	if !strings.Contains(logStr, "status=OK") {
		t.Errorf("expected status=OK in schedule.log")
	}

	// 3. devclean schedule logs --format json
	logsCmd := newScheduleLogsCmd()
	logsCmd.SetArgs([]string{"--format", "json"})
	var logsBuf bytes.Buffer
	logsCmd.SetOut(&logsBuf)
	if err := logsCmd.Execute(); err != nil {
		t.Fatalf("schedule logs failed: %v", err)
	}

	var parsedRuns []scheduler.LogRun
	if err := json.Unmarshal(logsBuf.Bytes(), &parsedRuns); err != nil {
		t.Fatalf("unmarshaling schedule logs json: %v\nOutput: %s", err, logsBuf.String())
	}
	if len(parsedRuns) == 0 {
		t.Fatalf("expected at least 1 parsed run from schedule logs, got 0")
	}
	if parsedRuns[0].Status != "OK" {
		t.Errorf("expected run status OK, got %s", parsedRuns[0].Status)
	}
}

// Flow 4: User explores disk via analyze (Phase 4) -> attempts to delete protected file (blocked) vs normal file (trashed)
func TestE2E_Flow4_AnalyzeTUI_BlacklistProtection_Vs_Trash(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	trashDir := filepath.Join(homeDir, ".local", "share", "devclean", "trash")
	configDir := filepath.Join(homeDir, ".config", "devclean")

	if err := os.MkdirAll(trashDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_DATA_HOME", filepath.Join(homeDir, ".local", "share"))

	testDir := filepath.Join(tmpDir, "test_explore")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a protected directory (.git)
	gitDir := filepath.Join(testDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("[core]\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a normal deletable file
	normalFile := filepath.Join(testDir, "normal_cache.bin")
	if err := os.WriteFile(normalFile, make([]byte, 1024), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Defaults()
	cfg.Safety.TrashEnabled = true
	cfg.Safety.TrashDir = trashDir
	bl := safety.NewBlacklist(cfg.ProtectedPatterns)

	scanner := analyzer.NewScanner(bl, true, 0, 2)
	rootNode, err := scanner.ScanImmediate(testDir)
	if err != nil {
		t.Fatalf("scanner.ScanImmediate failed: %v", err)
	}

	trashMgr := trash.New(cfg.Safety)
	coord := deleter.New(nil, &cfg, bl, trashMgr, nil, configDir, filepath.Join(homeDir, ".local", "share", "devclean"))

	m := tui.NewAnalyzerModel(rootNode, bl, coord, 100, 30, scanner)

	// Verify .git node is marked protected
	var gitNode *analyzer.Node
	var normalNode *analyzer.Node
	for _, child := range rootNode.Children {
		if child.Name == ".git" {
			gitNode = child
		} else if child.Name == "normal_cache.bin" {
			normalNode = child
		}
	}

	if gitNode == nil {
		t.Fatalf("expected .git child in rootNode")
	}
	if normalNode == nil {
		t.Fatalf("expected normal_cache.bin child in rootNode")
	}

	if !gitNode.IsProtected {
		t.Errorf("expected .git node to be marked IsProtected=true")
	}
	if normalNode.IsProtected {
		t.Errorf("expected normalNode to NOT be protected")
	}

	// Navigate to gitNode and press 'd' -> must be blocked
	for i, c := range rootNode.Children {
		if c.Name == ".git" {
			for k := 0; k < i; k++ {
				newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
				m = newM.(tui.AnalyzerModel)
			}
			break
		}
	}
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = newM.(tui.AnalyzerModel)
	if m.IsConfirmingDelete() {
		t.Errorf("delete confirmation opened for protected .git node, expected immediate block")
	}
	if !strings.Contains(m.StatusMessage(), "protegido pela Blacklist Imutável") {
		t.Errorf("expected status message mentioning protected by blacklist, got: %s", m.StatusMessage())
	}

	// Now move cursor to normalNode and press 'd' -> must ask confirmation
	m2 := tui.NewAnalyzerModel(rootNode, bl, coord, 100, 30, scanner)
	for i, c := range rootNode.Children {
		if c.Name == "normal_cache.bin" {
			for k := 0; k < i; k++ {
				newM2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyDown})
				m2 = newM2.(tui.AnalyzerModel)
			}
			break
		}
	}
	newM2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m2 = newM2.(tui.AnalyzerModel)
	if !m2.IsConfirmingDelete() {
		t.Fatalf("expected state confirming delete for normalNode")
	}

	// Confirm delete with 'y'
	newM2, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m2 = newM2.(tui.AnalyzerModel)
	if cmd != nil {
		msg := cmd()
		newM2, _ = m2.Update(msg)
		m2 = newM2.(tui.AnalyzerModel)
	}

	// Verify normal file was moved to trash
	if _, err := os.Stat(normalFile); !os.IsNotExist(err) {
		t.Errorf("expected normal file to be moved to trash")
	}
}

// Flow 5: User runs purge interactively (Phase 4) -> selects git repos -> confirms purge -> trash handles targets
func TestE2E_Flow5_PurgeTUI_GitRepoDetection_And_Trash(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	configDir := filepath.Join(homeDir, ".config", "devclean")
	workspace := filepath.Join(tmpDir, "workspace")
	trashDir := filepath.Join(homeDir, ".local", "share", "devclean", "trash")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(workspace, 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(homeDir, ".local", "share"))

	// Create inactive git repo with node_modules
	inactiveRepo := filepath.Join(workspace, "legacy-service")
	if err := os.MkdirAll(inactiveRepo, 0755); err != nil {
		t.Fatal(err)
	}
	setupV22GitRepo(t, inactiveRepo, "main", 45)

	nodeModules := filepath.Join(inactiveRepo, "node_modules")
	if err := os.MkdirAll(nodeModules, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nodeModules, "pkg.js"), []byte("console.log('heavy');"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inactiveRepo, "package.json"), []byte(`{"name":"legacy"}`), 0644); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(configDir, "config.yaml")
	cfgContent := fmt.Sprintf(`scan_paths:
  - %q
safety:
  trash_enabled: true
  trash_dir: %q
ecosystems:
  node:
    enabled: true
    max_age_days: 0
`, workspace, trashDir)
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	origConfigPath := configPath
	origQuiet := quiet
	configPath = cfgPath
	quiet = true
	defer func() {
		configPath = origConfigPath
		quiet = origQuiet
	}()

	// Run devclean purge with --yes and --inactive-days 30
	purgeCmd := newPurgeCmd()
	purgeCmd.SetArgs([]string{"--yes", "--inactive-days", "30", "--trash"})
	var purgeBuf bytes.Buffer
	purgeCmd.SetOut(&purgeBuf)
	purgeCmd.SetErr(&purgeBuf)
	if err := purgeCmd.Execute(); err != nil {
		t.Fatalf("purge command failed: %v", err)
	}

	// Verify node_modules was deleted
	if _, err := os.Stat(filepath.Join(nodeModules, "pkg.js")); !os.IsNotExist(err) {
		t.Errorf("expected node_modules to be purged")
	}

	// Verify Git repository state and .git folder are 100% intact
	gitConfig := filepath.Join(inactiveRepo, ".git", "config")
	if _, err := os.Stat(gitConfig); err != nil {
		t.Errorf("expected .git/config to remain untouched, got error: %v", err)
	}

	// Verify trash directory has batch with the purged target
	trashCfg := config.SafetyConfig{
		TrashEnabled: true,
		TrashDir:     trashDir,
	}
	trashMgr := trash.New(trashCfg)
	batches, err := trashMgr.List(context.Background())
	if err != nil {
		t.Fatalf("trash list failed: %v", err)
	}
	if len(batches) == 0 {
		t.Fatalf("expected trash batch for purged repo targets, got 0")
	}
}

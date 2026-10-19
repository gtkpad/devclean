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
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/mcp"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/tui"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

type v23TestRig struct {
	TmpDir        string
	FakeHome      string
	FakeWorkspace string
	ConfigFile    string
	TrashDir      string
	Sentinels     map[string]string // targetPath -> expectedContent
}

func setupV23HermeticRig(t *testing.T) *v23TestRig {
	t.Helper()
	tmpDir := t.TempDir()
	fakeHome := filepath.Join(tmpDir, "home")
	fakeWorkspace := filepath.Join(tmpDir, "workspace")
	configDir := filepath.Join(fakeHome, ".config", "devclean")
	dataDir := filepath.Join(fakeHome, ".local", "share", "devclean")
	trashDir := filepath.Join(dataDir, "trash")
	stateDir := filepath.Join(fakeHome, ".local", "state", "devclean")
	logsDir := filepath.Join(stateDir, "logs")

	for _, d := range []string{configDir, dataDir, trashDir, stateDir, logsDir, fakeWorkspace} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("failed creating rig dir %s: %v", d, err)
		}
	}

	t.Setenv("HOME", fakeHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(fakeHome, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(fakeHome, ".local", "share"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(fakeHome, ".local", "state"))
	t.Setenv("TMPDIR", filepath.Join(tmpDir, "tmp"))

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
  pnpm:
    enabled: true
    max_age_days: 0
  docker:
    enabled: true
    max_age_days: 0
  macos:
    enabled: true
    max_age_days: 0
  browsers:
    enabled: true
    max_age_days: 0
  leftovers:
    enabled: true
    max_age_days: 0
  xcode:
    enabled: true
    max_age_days: 0
`, fakeWorkspace, trashDir)

	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("failed writing test config: %v", err)
	}

	rig := &v23TestRig{
		TmpDir:        tmpDir,
		FakeHome:      fakeHome,
		FakeWorkspace: fakeWorkspace,
		ConfigFile:    cfgPath,
		TrashDir:      trashDir,
		Sentinels:     make(map[string]string),
	}

	// 1. pnpm global store sentinel: Library/pnpm/store/v3/canary.bin
	pnpmStore := filepath.Join(fakeHome, "Library", "pnpm", "store", "v3")
	rig.plantSentinel(t, pnpmStore, "canary.bin", "SENTINEL_INVIOLABLE_PNPM_STORE_V23")

	// 2. OrbStack VM data sentinel: Library/Group Containers/group.dev.orbstack/data/vm.img
	orbData := filepath.Join(fakeHome, "Library", "Group Containers", "group.dev.orbstack", "data")
	rig.plantSentinel(t, orbData, "vm.img", "SENTINEL_INVIOLABLE_ORBSTACK_DATA_V23")

	// 3. LaunchAgent stale sentinel: Library/LaunchAgents/com.corp.stale.plist
	agentDir := filepath.Join(fakeHome, "Library", "LaunchAgents")
	rig.plantSentinel(t, agentDir, "com.corp.stale.plist", "SENTINEL_INVIOLABLE_LAUNCHAGENT_V23")

	// 4. DeadApp leftover sentinel: Library/Application Support/DeadApp/state.db
	deadAppDir := filepath.Join(fakeHome, "Library", "Application Support", "DeadApp")
	rig.plantSentinel(t, deadAppDir, "state.db", "SENTINEL_INVIOLABLE_APP_LEFTOVER_V23")

	// 5. ScriptCache browser sentinel: Library/Application Support/Google/Chrome/Default/Service Worker/ScriptCache/index
	scriptCacheDir := filepath.Join(fakeHome, "Library", "Application Support", "Google", "Chrome", "Default", "Service Worker", "ScriptCache")
	rig.plantSentinel(t, scriptCacheDir, "index", "SENTINEL_INVIOLABLE_SCRIPTCACHE_V23")

	return rig
}

func (r *v23TestRig) plantSentinel(t *testing.T, dir, filename, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed creating sentinel dir %s: %v", dir, err)
	}
	filePath := filepath.Join(dir, filename)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed planting sentinel file %s: %v", filePath, err)
	}
	r.Sentinels[filePath] = content
	return filePath
}

func (r *v23TestRig) assertAllSentinelsIntact(t *testing.T, channelName string) {
	t.Helper()
	for path, expectedContent := range r.Sentinels {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("[%s] INVIOLABLE MUTATION: sentinel file %s was deleted or inaccessible: %v", channelName, path, err)
		}
		if string(data) != expectedContent {
			t.Fatalf("[%s] INVIOLABLE CORRUPTION: sentinel file %s was altered! got %q, want %q", channelName, path, string(data), expectedContent)
		}
	}
}

// fakeReviewCleaner supplies predefined review-only targets for hermetic MCP/Coordinator testing
type fakeReviewCleaner struct {
	name    string
	targets []domain.Target
	cleaned []domain.Target
}

func (f *fakeReviewCleaner) Name() string        { return f.name }
func (f *fakeReviewCleaner) IsInstalled() bool   { return true }
func (f *fakeReviewCleaner) Scan(_ context.Context, _ domain.EcosystemRules) ([]domain.Target, error) {
	return f.targets, nil
}
func (f *fakeReviewCleaner) Clean(_ context.Context, targets []domain.Target) error {
	f.cleaned = append(f.cleaned, targets...)
	return nil
}

// TestE2E_V23_MultiChannel_ReviewOnlyImmunity exercises all 5 channels:
// CLI, Deleter Coordinator, MCP Server, Scheduler, and Bubble Tea TUI.
func TestE2E_V23_MultiChannel_ReviewOnlyImmunity(t *testing.T) {
	rig := setupV23HermeticRig(t)

	// Build domain review targets representing the 5 planted sentinels
	reviewTargets := []domain.Target{
		{
			ID:            "pnpm-store-sentinel",
			Name:          "pnpm store",
			Path:          filepath.Join(rig.FakeHome, "Library", "pnpm", "store", "v3"),
			SizeBytes:     1024 * 1024,
			EligibleBytes: 0,
			ReviewOnly:    true,
			ReviewReason:  domain.ReviewReasonSharedStore,
			Category:      "pnpm-store",
		},
		{
			ID:            "orbstack-data-sentinel",
			Name:          "OrbStack VM data",
			Path:          filepath.Join(rig.FakeHome, "Library", "Group Containers", "group.dev.orbstack", "data"),
			SizeBytes:     50 * 1024 * 1024,
			EligibleBytes: 0,
			ReviewOnly:    true,
			ReviewReason:  domain.ReviewReasonSharedStore,
			Category:      "docker-vm",
		},
		{
			ID:            "launchagent-stale-sentinel",
			Name:          "Stale LaunchAgent",
			Path:          filepath.Join(rig.FakeHome, "Library", "LaunchAgents", "com.corp.stale.plist"),
			SizeBytes:     1024,
			EligibleBytes: 0,
			ReviewOnly:    true,
			ReviewReason:  domain.ReviewReasonStaleLaunchAgent,
			Category:      "launch-agent",
		},
		{
			ID:            "deadapp-leftover-sentinel",
			Name:          "DeadApp leftover",
			Path:          filepath.Join(rig.FakeHome, "Library", "Application Support", "DeadApp"),
			SizeBytes:     4096,
			EligibleBytes: 0,
			ReviewOnly:    true,
			ReviewReason:  domain.ReviewReasonAppLeftover,
			Category:      "app-support",
		},
		{
			ID:            "scriptcache-sentinel",
			Name:          "Chrome ScriptCache",
			Path:          filepath.Join(rig.FakeHome, "Library", "Application Support", "Google", "Chrome", "Default", "Service Worker", "ScriptCache"),
			SizeBytes:     2048,
			EligibleBytes: 0,
			ReviewOnly:    true,
			ReviewReason:  "scriptcache_requires_evidence",
			Category:      "browser-scriptcache",
		},
	}

	origConfigPath := configPath
	origQuiet := quiet
	configPath = rig.ConfigFile
	quiet = true
	defer func() {
		configPath = origConfigPath
		quiet = origQuiet
	}()

	// -------------------------------------------------------------------------
	// Channel 1: CLI Channel (devclean clean --auto --yes)
	// -------------------------------------------------------------------------
	cliCmd := newCleanCmd()
	if cliCmd.Flags().Lookup("yes") == nil {
		cliCmd.Flags().BoolP("yes", "y", false, "bypass confirmation")
	}
	cliCmd.SetArgs([]string{"--auto", "--yes"})
	var cliBuf bytes.Buffer
	cliCmd.SetOut(&cliBuf)
	cliCmd.SetErr(&cliBuf)
	_ = cliCmd.Execute() // "nothing to clean" or clean completed
	rig.assertAllSentinelsIntact(t, "CLI")

	// -------------------------------------------------------------------------
	// Channel 2: Deleter Coordinator Channel (coord.Delete with reviewTargets)
	// -------------------------------------------------------------------------
	cfg, err := config.Load(rig.ConfigFile)
	if err != nil {
		t.Fatalf("loading config failed: %v", err)
	}
	bl := safety.NewBlacklist(cfg.ProtectedPatterns)
	coord := deleter.New(nil, cfg, bl, nil, nil, rig.FakeHome, rig.FakeHome)

	res, err := coord.Delete(context.Background(), map[string][]domain.Target{
		"system": reviewTargets,
	}, deleter.Options{Mode: "force"})
	if err != nil {
		t.Fatalf("coordinator delete failed: %v", err)
	}
	if len(res.SkippedReviewOnly) != len(reviewTargets) {
		t.Fatalf("expected coordinator to skip %d review targets, got %d", len(reviewTargets), len(res.SkippedReviewOnly))
	}
	if res.TotalFreed != 0 {
		t.Fatalf("expected 0 bytes freed for review-only batch, got %d", res.TotalFreed)
	}
	rig.assertAllSentinelsIntact(t, "Coordinator")

	// -------------------------------------------------------------------------
	// Channel 3: MCP Server Channel (propose_cleanup and execute_cleanup)
	// -------------------------------------------------------------------------
	fakeCleaner := &fakeReviewCleaner{
		name:    "system",
		targets: reviewTargets,
	}
	resolver := platform.New()
	eng := engine.New(cfg, resolver)
	eng.Register(fakeCleaner)

	mcpServer := mcp.New(eng, cfg, mcp.WithCoordinator(coord))

	// 3a. Invoke propose_deletion
	ids := []string{
		"pnpm-store-sentinel",
		"orbstack-data-sentinel",
		"launchagent-stale-sentinel",
		"deadapp-leftover-sentinel",
		"scriptcache-sentinel",
	}
	idsBytes, _ := json.Marshal(ids)

	reqPropose := mcpgo.CallToolRequest{}
	reqPropose.Params.Arguments = map[string]interface{}{
		"target_ids": string(idsBytes),
	}
	proposeRes, err := mcpServer.HandlePropose(context.Background(), reqPropose)
	if err != nil {
		t.Fatalf("HandlePropose failed: %v", err)
	}
	var proposeEntries []struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		EligibleBytes int64  `json:"eligible_bytes"`
		ReviewReason  string `json:"review_reason"`
	}
	if len(proposeRes.Content) == 0 {
		t.Fatalf("HandlePropose returned empty content")
	}
	proposeText := proposeRes.Content[0].(mcpgo.TextContent).Text
	if err := json.Unmarshal([]byte(proposeText), &proposeEntries); err != nil {
		t.Fatalf("unmarshaling propose response: %v\nText: %s", err, proposeText)
	}
	for _, entry := range proposeEntries {
		if entry.Status != "review_only" {
			t.Errorf("expected target %s to have status 'review_only', got %q", entry.ID, entry.Status)
		}
		if entry.EligibleBytes != 0 {
			t.Errorf("expected target %s to have eligible_bytes 0, got %d", entry.ID, entry.EligibleBytes)
		}
	}

	// 3b. Invoke execute_approved_deletion
	reqExec := mcpgo.CallToolRequest{}
	reqExec.Params.Arguments = map[string]interface{}{
		"target_ids": string(idsBytes),
	}
	execRes, err := mcpServer.HandleExecute(context.Background(), reqExec)
	if err != nil {
		t.Fatalf("HandleExecute failed: %v", err)
	}
	var execOut map[string]interface{}
	execText := execRes.Content[0].(mcpgo.TextContent).Text
	if err := json.Unmarshal([]byte(execText), &execOut); err != nil {
		t.Fatalf("unmarshaling execute response: %v\nText: %s", err, execText)
	}
	if int(execOut["deleted_count"].(float64)) != 0 {
		t.Errorf("expected deleted_count == 0, got %v", execOut["deleted_count"])
	}
	if int(execOut["freed_bytes"].(float64)) != 0 {
		t.Errorf("expected freed_bytes == 0, got %v", execOut["freed_bytes"])
	}
	if len(fakeCleaner.cleaned) != 0 {
		t.Errorf("expected fakeCleaner.cleaned to be empty, got %d items", len(fakeCleaner.cleaned))
	}
	rig.assertAllSentinelsIntact(t, "MCP")

	// -------------------------------------------------------------------------
	// Channel 4: Scheduler Automated Background Run (schedule run)
	// -------------------------------------------------------------------------
	origRunCleanSubprocess := runCleanSubprocess
	runCleanSubprocess = func(ctx context.Context, configDir string, args []string, logWriter io.Writer) cleanExecutionResult {
		// Run actual clean execution logic on hermetic rig
		subCmd := newCleanCmd()
		if subCmd.Flags().Lookup("yes") == nil {
			subCmd.Flags().BoolP("yes", "y", false, "yes")
		}
		subCmd.SetArgs(args)
		var subBuf bytes.Buffer
		subCmd.SetOut(&subBuf)
		subCmd.SetErr(&subBuf)
		_ = subCmd.Execute()

		return cleanExecutionResult{
			ExitCode:     0,
			Output:       subBuf.String(),
			TargetsCount: 0,
			BytesFreed:   0,
			FreedHuman:   "0 B",
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
	rig.assertAllSentinelsIntact(t, "Scheduler")

	// -------------------------------------------------------------------------
	// Channel 5: TUI Bubble Tea Headless Harness
	// -------------------------------------------------------------------------
	tuiResults := []domain.ScanResult{
		{
			Ecosystem: "system",
			Targets:   reviewTargets,
		},
	}
	m := tui.NewModelWithTargets(tuiResults, coord, bl, cfg)

	// Dispatch selection messages: 'a' (select all), space, enter, and confirm 'y'
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeySpace})
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newM, cmdDel := newM.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if cmdDel != nil {
		msg := cmdDel()
		_, _ = newM.Update(msg)
	}
	rig.assertAllSentinelsIntact(t, "TUI")
}

// TestE2E_V23_FaultInjection_ProcessGuard verifies that active application processes
// (Chrome, OrbStack, Xcode) cause scan targets to receive ReviewOnly: true and
// ReviewReason: active_process.
func TestE2E_V23_FaultInjection_ProcessGuard(t *testing.T) {
	rig := setupV23HermeticRig(t)

	// Create Chrome profile and cache (support Darwin and Linux layout)
	chromeCacheDarwin := filepath.Join(rig.FakeHome, "Library", "Caches", "Google", "Chrome", "Default", "GPUCache")
	_ = os.MkdirAll(chromeCacheDarwin, 0755)
	_ = os.WriteFile(filepath.Join(chromeCacheDarwin, "data_0"), []byte("chrome cache data"), 0644)

	chromeAppLinux := filepath.Join(rig.FakeHome, ".config", "google-chrome", "Default", "GPUCache")
	_ = os.MkdirAll(chromeAppLinux, 0755)
	_ = os.WriteFile(filepath.Join(chromeAppLinux, "data_0"), []byte("chrome cache data"), 0644)

	// Create OrbStack cache (support Darwin and Linux layout)
	orbCache := filepath.Join(rig.FakeHome, "Library", "Caches", "dev.orbstack.OrbStack")
	_ = os.MkdirAll(orbCache, 0755)
	_ = os.WriteFile(filepath.Join(orbCache, "cache.db"), []byte("orbstack cache"), 0644)
	orbLinuxCache := filepath.Join(rig.FakeHome, ".cache", "orbstack")
	_ = os.MkdirAll(orbLinuxCache, 0755)
	_ = os.WriteFile(filepath.Join(orbLinuxCache, "cache.db"), []byte("orbstack cache"), 0644)

	// Create Xcode DerivedData directory (Darwin only)
	derivedData := filepath.Join(rig.FakeHome, "Library", "Developer", "Xcode", "DerivedData", "App-abcdef")
	_ = os.MkdirAll(derivedData, 0755)
	_ = os.WriteFile(filepath.Join(derivedData, "build.db"), []byte("derived data"), 0644)

	mockExec := cleaner.NewMockExecutor()
	// Simulate active PIDs for Chrome, OrbStack, and Xcode
	mockExec.Outputs["pgrep -x Google Chrome"] = []byte("11111\n")
	mockExec.Outputs["pgrep -x chrome"] = []byte("11111\n")
	mockExec.Outputs["pgrep -x google-chrome"] = []byte("11111\n")
	mockExec.Outputs["pgrep -x OrbStack"] = []byte("22222\n")
	mockExec.Outputs["pgrep -x Xcode"] = []byte("33333\n")

	cfg, err := config.Load(rig.ConfigFile)
	if err != nil {
		t.Fatalf("loading config failed: %v", err)
	}

	resolver := platform.New()
	eng := engine.New(cfg, resolver)
	engine.RegisterDefaultCleanersWithExecutor(eng, cfg, resolver, mockExec)

	ctx := context.Background()
	results := engine.Collect(ctx, eng.Scan(ctx))

	foundChrome := false
	foundOrbStack := false
	foundXcode := false

	for _, res := range results {
		for _, tgt := range res.Targets {
			if strings.Contains(tgt.Name, "Chrome") || strings.Contains(tgt.Path, "Chrome") {
				foundChrome = true
				if !tgt.ReviewOnly {
					t.Errorf("expected Chrome target %s to have ReviewOnly=true, got false", tgt.ID)
				}
				if tgt.ReviewReason != domain.ReviewReasonActiveProcess && tgt.ReviewReason != "scriptcache_requires_evidence" {
					t.Errorf("expected Chrome target %s to have ReviewReason active_process, got %q", tgt.ID, tgt.ReviewReason)
				}
			}
			if strings.Contains(tgt.ID, "orbstack") || strings.Contains(tgt.Path, "OrbStack") {
				foundOrbStack = true
				if !tgt.ReviewOnly {
					t.Errorf("expected OrbStack target %s to have ReviewOnly=true, got false", tgt.ID)
				}
				if tgt.ReviewReason != domain.ReviewReasonActiveProcess && tgt.ReviewReason != domain.ReviewReasonSharedStore {
					t.Errorf("expected OrbStack target %s to have ReviewReason active_process, got %q", tgt.ID, tgt.ReviewReason)
				}
			}
			if strings.Contains(tgt.ID, "xcode") || strings.Contains(tgt.Path, "Xcode") {
				foundXcode = true
				if !tgt.ReviewOnly {
					t.Errorf("expected Xcode target %s to have ReviewOnly=true, got false", tgt.ID)
				}
				if tgt.ReviewReason != domain.ReviewReasonActiveProcess {
					t.Errorf("expected Xcode target %s to have ReviewReason active_process, got %q", tgt.ID, tgt.ReviewReason)
				}
			}
		}
	}

	if !foundChrome {
		t.Errorf("expected to find Chrome targets in scan results")
	}
	if !foundOrbStack {
		t.Errorf("expected to find OrbStack targets in scan results")
	}
	if runtime.GOOS == "darwin" && !foundXcode {
		t.Errorf("expected to find Xcode targets in scan results")
	}
}

// TestE2E_V23_FaultInjection_SymlinkGuard verifies that symlinks inside cleanable
// directories pointing to sensitive paths (.ssh, .git) are rejected and sensitive files untouched.
func TestE2E_V23_FaultInjection_SymlinkGuard(t *testing.T) {
	rig := setupV23HermeticRig(t)

	// Plant sensitive sentinels in system blacklisted / sensitive paths
	sshDir := filepath.Join(rig.FakeHome, ".ssh")
	sshSentinel := rig.plantSentinel(t, sshDir, "id_rsa", "PRIVATE_SSH_KEY_DO_NOT_DELETE")

	gitDir := filepath.Join(rig.FakeWorkspace, ".git")
	gitSentinel := rig.plantSentinel(t, gitDir, "config", "GIT_CONFIG_HISTORY_DO_NOT_DELETE")

	// Create cleanable cache directory containing malicious symlinks pointing to sensitive files
	cacheDir := filepath.Join(rig.FakeHome, "Library", "Caches", "test-malicious-app")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("failed creating cache dir: %v", err)
	}
	disposableFile := filepath.Join(cacheDir, "temp.cache")
	if err := os.WriteFile(disposableFile, []byte("disposable data"), 0644); err != nil {
		t.Fatalf("failed writing disposable cache: %v", err)
	}

	symlinkToSSH := filepath.Join(cacheDir, "symlink_ssh")
	if err := os.Symlink(sshDir, symlinkToSSH); err != nil {
		t.Fatalf("failed creating symlink to ssh: %v", err)
	}

	symlinkToGit := filepath.Join(cacheDir, "symlink_git")
	if err := os.Symlink(gitDir, symlinkToGit); err != nil {
		t.Fatalf("failed creating symlink to git: %v", err)
	}

	target := domain.Target{
		ID:         "macos-cache-malicious",
		Name:       "Malicious App Cache",
		Path:       cacheDir,
		Category:   "macos-app-cache",
		ReviewOnly: false,
	}

	cfg, err := config.Load(rig.ConfigFile)
	if err != nil {
		t.Fatalf("failed loading config: %v", err)
	}
	bl := safety.NewBlacklist(cfg.ProtectedPatterns)
	coord := deleter.New(nil, cfg, bl, nil, nil, rig.FakeHome, rig.FakeHome)

	// Execute deletion on the cache target
	_, err = coord.Delete(context.Background(), map[string][]domain.Target{
		"macos": {target},
	}, deleter.Options{Mode: "force"})
	if err != nil {
		t.Fatalf("unexpected coordinator error: %v", err)
	}

	// Verify sentinels in .ssh and .git are 100% untouched
	rig.assertAllSentinelsIntact(t, "SymlinkGuard")

	sshData, err := os.ReadFile(sshSentinel)
	if err != nil || string(sshData) != "PRIVATE_SSH_KEY_DO_NOT_DELETE" {
		t.Fatalf("INVIOLABLE BREACH: .ssh/id_rsa was modified or deleted: %v", err)
	}
	gitData, err := os.ReadFile(gitSentinel)
	if err != nil || string(gitData) != "GIT_CONFIG_HISTORY_DO_NOT_DELETE" {
		t.Fatalf("INVIOLABLE BREACH: .git/config was modified or deleted: %v", err)
	}
}

// TestE2E_V23_FaultInjection_GracefulDegradation verifies that command execution
// failures (e.g. docker daemon offline) complete gracefully without panics,
// set ProviderMetrics.Partial = true, and assign ReviewReason ambiguous_state.
func TestE2E_V23_FaultInjection_GracefulDegradation(t *testing.T) {
	rig := setupV23HermeticRig(t)

	mockExec := cleaner.NewMockExecutor()
	// Inject connection failure for docker commands
	mockExec.Errors["docker system df --format {{json .}}"] = errors.New("cannot connect to Docker daemon")
	mockExec.Errors["docker system df --format json"] = errors.New("cannot connect to Docker daemon")
	mockExec.Errors["docker system df"] = errors.New("cannot connect to Docker daemon")

	cfg, err := config.Load(rig.ConfigFile)
	if err != nil {
		t.Fatalf("failed loading config: %v", err)
	}

	resolver := platform.New()
	eng := engine.New(cfg, resolver)
	engine.RegisterDefaultCleanersWithExecutor(eng, cfg, resolver, mockExec)

	ctx := context.Background()
	results := engine.Collect(ctx, eng.Scan(ctx))

	foundPartial := false
	for _, res := range results {
		for _, tgt := range res.Targets {
			if tgt.ProviderMetrics != nil && tgt.ProviderMetrics.Partial {
				foundPartial = true
				if !tgt.ReviewOnly {
					t.Errorf("expected degraded target %s with Partial=true to have ReviewOnly=true", tgt.ID)
				}
				if tgt.ReviewReason != domain.ReviewReasonAmbiguous && tgt.ReviewReason != domain.ReviewReasonSystemProtected {
					t.Errorf("expected ReviewReason ambiguous_state or system_protected, got %q", tgt.ReviewReason)
				}
			}
		}
	}

	if !foundPartial {
		t.Errorf("expected at least one target to report ProviderMetrics.Partial == true under injected fault")
	}
}

// TestE2E_V23_ZeroDoubleCounting_AcrossEcosystems initializes all default cleaners
// across all 19 ecosystems, executes engine.Scan(), and proves zero duplicate target
// paths and zero parent/child path overlaps across the entire scanned inventory.
func TestE2E_V23_ZeroDoubleCounting_AcrossEcosystems(t *testing.T) {
	rig := setupV23HermeticRig(t)

	// Create sample project directories in fakeWorkspace across multiple ecosystems
	// 1. Node project with node_modules
	nodeProj := filepath.Join(rig.FakeWorkspace, "proj-node")
	_ = os.MkdirAll(filepath.Join(nodeProj, "node_modules", "pkg"), 0755)
	_ = os.WriteFile(filepath.Join(nodeProj, "package.json"), []byte(`{"name":"test"}`), 0644)
	_ = os.WriteFile(filepath.Join(nodeProj, "node_modules", "pkg", "index.js"), []byte(`console.log("hi");`), 0644)

	// 2. Go project with build cache
	goProj := filepath.Join(rig.FakeWorkspace, "proj-go")
	_ = os.MkdirAll(goProj, 0755)
	_ = os.WriteFile(filepath.Join(goProj, "go.mod"), []byte("module testgo\ngo 1.22\n"), 0644)

	// 3. Rust project with target
	rustProj := filepath.Join(rig.FakeWorkspace, "proj-rust")
	_ = os.MkdirAll(filepath.Join(rustProj, "target", "debug"), 0755)
	_ = os.WriteFile(filepath.Join(rustProj, "Cargo.toml"), []byte("[package]\nname = \"testrust\"\nversion = \"0.1.0\"\n"), 0644)
	_ = os.WriteFile(filepath.Join(rustProj, "target", "debug", "binary"), []byte("bin"), 0644)

	// 4. Python project with .venv
	pyProj := filepath.Join(rig.FakeWorkspace, "proj-python")
	_ = os.MkdirAll(filepath.Join(pyProj, ".venv", "bin"), 0755)
	_ = os.WriteFile(filepath.Join(pyProj, "pyproject.toml"), []byte("[project]\nname = 'test'\n"), 0644)
	_ = os.WriteFile(filepath.Join(pyProj, ".venv", "pyvenv.cfg"), []byte("home = /usr/bin\n"), 0644)

	cfg, err := config.Load(rig.ConfigFile)
	if err != nil {
		t.Fatalf("failed loading config: %v", err)
	}
	cfg.ScanPaths = []string{rig.FakeWorkspace}

	resolver := platform.New()
	eng := engine.New(cfg, resolver)
	engine.RegisterDefaultCleaners(eng, cfg, resolver)

	ctx := context.Background()
	results := engine.Collect(ctx, eng.Scan(ctx))

	seenPaths := make(map[string]string) // clean path -> target ID
	for _, res := range results {
		for _, trg := range res.Targets {
			if strings.HasPrefix(trg.Path, "browser://") ||
				strings.HasPrefix(trg.Path, "docker://") ||
				strings.HasPrefix(trg.Path, "pnpm://") ||
				strings.TrimSpace(trg.Path) == "" {
				continue // Skip virtual scheme paths
			}
			cleanPath := filepath.Clean(trg.Path)

			// 1. Exact collision check
			if existingID, exists := seenPaths[cleanPath]; exists {
				t.Fatalf("ZERO DOUBLE-COUNTING VIOLATION: duplicate path %s reported by both %s and %s",
					cleanPath, existingID, trg.ID)
			}

			// 2. Parent / child overlap check
			for registeredPath, existingID := range seenPaths {
				if strings.HasPrefix(cleanPath, registeredPath+string(filepath.Separator)) {
					t.Fatalf("ZERO DOUBLE-COUNTING VIOLATION: nested path %s (%s) is a child of already registered %s (%s)",
						cleanPath, trg.ID, registeredPath, existingID)
				}
				if strings.HasPrefix(registeredPath, cleanPath+string(filepath.Separator)) {
					t.Fatalf("ZERO DOUBLE-COUNTING VIOLATION: registered path %s (%s) is a child of new path %s (%s)",
						registeredPath, existingID, cleanPath, trg.ID)
				}
			}

			seenPaths[cleanPath] = trg.ID
		}
	}
}


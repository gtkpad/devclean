package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/trash"
)

type mockVirtualCleaner struct {
	name    string
	cleaned []domain.Target
}

func (m *mockVirtualCleaner) Name() string      { return m.name }
func (m *mockVirtualCleaner) IsInstalled() bool { return true }
func (m *mockVirtualCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	return []domain.Target{
		{
			ID:        "mock-virt",
			Name:      "Virtual Target",
			Path:      m.name + "://cache/prune",
			SizeBytes: 2048,
			Risk:      domain.RiskLow,
		},
	}, nil
}
func (m *mockVirtualCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	m.cleaned = append(m.cleaned, targets...)
	return nil
}

// TestIntegration_CrossPhase_VirtualCleanersWithDeleter verifies that virtual cleaners
// (Phase 3 pnpm://, brew://, xcode://) route through deleter.Coordinator (Phase 1)
// with trash bypass (BUG-01, CLEAN-02, CLEAN-03).
func TestIntegration_CrossPhase_VirtualCleanersWithDeleter(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	dataDir := filepath.Join(tmpDir, "data")
	trashDir := filepath.Join(tmpDir, "trash")

	cfg := &config.Config{
		Safety: config.SafetyConfig{
			TrashEnabled: true,
			TrashDir:     trashDir,
		},
	}

	resolver := platform.New()
	eng := engine.New(cfg, resolver)

	pnpmMock := &mockVirtualCleaner{name: "pnpm"}
	brewMock := &mockVirtualCleaner{name: "homebrew"}
	eng.Register(pnpmMock)
	eng.Register(brewMock)

	bl := safety.NewBlacklist(nil)
	trashMgr := trash.New(cfg.Safety)
	coord := deleter.New(eng, cfg, bl, trashMgr, nil, configDir, dataDir)

	pnpmTargets, _ := pnpmMock.Scan(context.Background(), domain.EcosystemRules{})
	brewTargets, _ := brewMock.Scan(context.Background(), domain.EcosystemRules{})

	targetsByEco := map[string][]domain.Target{
		"pnpm":     pnpmTargets,
		"homebrew": brewTargets,
	}

	var warnings []string
	var progressEvents []deleter.ProgressEvent
	useTrash := true

	res, err := coord.Delete(context.Background(), targetsByEco, deleter.Options{
		UseTrash: &useTrash,
		OnWarning: func(msg string) {
			warnings = append(warnings, msg)
		},
		OnProgress: func(event deleter.ProgressEvent) {
			progressEvents = append(progressEvents, event)
		},
	})
	if err != nil {
		t.Fatalf("coord.Delete failed: %v", err)
	}

	if res.TotalCount != 2 {
		t.Errorf("expected 2 cleaned targets, got %d", res.TotalCount)
	}
	if len(pnpmMock.cleaned) != 1 {
		t.Errorf("expected pnpmMock to have cleaned 1 virtual target, got %d", len(pnpmMock.cleaned))
	}
	if len(brewMock.cleaned) != 1 {
		t.Errorf("expected brewMock to have cleaned 1 virtual target, got %d", len(brewMock.cleaned))
	}

	// Verify warnings about trash bypass
	if len(warnings) < 2 {
		t.Errorf("expected at least 2 warnings for trash bypass, got %d: %v", len(warnings), warnings)
	}

	// Verify progress actions were ActionVirtualDirect
	for _, pe := range progressEvents {
		if pe.Action != deleter.ActionVirtualDirect {
			t.Errorf("expected action ActionVirtualDirect, got %v", pe.Action)
		}
	}

	// Ensure trash remains empty
	manifest, err := trash.LoadManifest(trashMgr.ManifestPath())
	if err == nil && len(manifest.Batches) > 0 {
		t.Errorf("expected no batches in trash manifest for purely virtual targets, got %d", len(manifest.Batches))
	}
}

// TestIntegration_CrossPhase_E2E_ScanCleanPurgeWhitelist tests the cohesive flow
// connecting Phase 1 (Deleter, Safety), Phase 2 (E2E lifecycle), Phase 3 (Modern Cleaners),
// and Phase 4 (Whitelist, Purge, Report Card).
func TestIntegration_CrossPhase_E2E_ScanCleanPurgeWhitelist(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	configDir := filepath.Join(homeDir, ".config", "devclean")
	workspace := filepath.Join(tmpDir, "workspace")
	trashDir := filepath.Join(tmpDir, "trash")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(workspace, 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(homeDir, ".local", "share"))

	// Create Next.js project fixture (.next/cache)
	nextApp := filepath.Join(workspace, "my-nextjs-app")
	nextCache := filepath.Join(nextApp, ".next", "cache")
	if err := os.MkdirAll(nextCache, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nextApp, "next.config.js"), []byte("module.exports = {};"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nextCache, "artifact.bin"), []byte("cached-build-artifact"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create Inactive Git Repo fixture for Purge
	inactiveRepo := filepath.Join(workspace, "old-repo")
	if err := os.MkdirAll(inactiveRepo, 0755); err != nil {
		t.Fatal(err)
	}
	setupTestRepo(t, inactiveRepo, "main", 75, false)

	// Create Config
	cfgPath := filepath.Join(configDir, "config.yaml")
	cfgContent := fmt.Sprintf(`scan_paths:
  - %q
safety:
  trash_enabled: true
  trash_dir: %q
ecosystems:
  nextjs:
    enabled: true
    max_age_days: 0
  node:
    enabled: true
  docker:
    enabled: false
`, workspace, trashDir)
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	origConfigPath := configPath
	origQuiet := quiet
	origStdinReader := purgeStdinReader
	origIsTerminal := purgeIsTerminal
	configPath = cfgPath
	quiet = false
	defer func() {
		configPath = origConfigPath
		quiet = origQuiet
		purgeStdinReader = origStdinReader
		purgeIsTerminal = origIsTerminal
	}()

	// 1. Whitelist protection check (NFR-03, FEAT-02)
	// Verify attempting to remove immutable blacklist is blocked
	removeCmd := newWhitelistRemoveCmd()
	removeCmd.SetArgs([]string{".git"})
	if err := removeCmd.Execute(); err == nil {
		t.Error("expected error when trying to remove immutable .git from whitelist, got nil")
	}

	// Add an excluded path
	addCmd := newWhitelistAddCmd()
	addCmd.SetArgs([]string{filepath.Join(workspace, "excluded-folder")})
	if err := addCmd.Execute(); err != nil {
		t.Fatalf("whitelist add failed: %v", err)
	}

	// List whitelist
	listCmd := newWhitelistListCmd()
	var listBuf bytes.Buffer
	listCmd.SetOut(&listBuf)
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("whitelist list failed: %v", err)
	}
	if !strings.Contains(listBuf.String(), "Blacklist Imutável do Sistema:") {
		t.Error("whitelist list missing system blacklist section")
	}

	// 2. Scan flow: should find NextJS target
	scanCmd := newScanCmd()
	var scanBuf bytes.Buffer
	scanCmd.SetOut(&scanBuf)
	scanCmd.SetArgs([]string{"--ecosystem", "nextjs"})
	if err := scanCmd.Execute(); err != nil {
		t.Fatalf("scan execution failed: %v", err)
	}
	if !strings.Contains(scanBuf.String(), "my-nextjs-app") {
		t.Errorf("scan expected to find my-nextjs-app, got: %s", scanBuf.String())
	}

	// 3. Clean flow: Clean NextJS cache with clean card report
	cleanCmd := newCleanCmd()
	cleanCmd.SetArgs([]string{"--force", "--trash", "--ecosystem", "nextjs"})
	var cleanBuf bytes.Buffer
	cleanCmd.SetOut(&cleanBuf)
	if err := cleanCmd.Execute(); err != nil {
		t.Fatalf("clean execution failed: %v", err)
	}
	// Verify Next.js cache deleted
	if _, err := os.Stat(filepath.Join(nextCache, "artifact.bin")); !os.IsNotExist(err) {
		t.Errorf("expected Next.js cached artifact to be deleted")
	}

	// 4. Purge flow: Purge inactive Git repo node_modules
	purgeCmd := newPurgeCmd()
	purgeCmd.SetArgs([]string{"--yes", "--inactive-days", "60"})
	var purgeBuf bytes.Buffer
	purgeCmd.SetOut(&purgeBuf)
	if err := purgeCmd.Execute(); err != nil {
		t.Fatalf("purge execution failed: %v", err)
	}
	// Verify old-repo node_modules was purged
	if _, err := os.Stat(filepath.Join(inactiveRepo, "node_modules")); !os.IsNotExist(err) {
		t.Errorf("expected old-repo node_modules to be purged")
	}
}

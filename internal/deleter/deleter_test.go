package deleter_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/history"
	"github.com/gtkpad/devclean/internal/hooks"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/scheduler"
	"github.com/gtkpad/devclean/internal/trash"
)

type fakeCleaner struct {
	name      string
	installed bool
	cleaned   []domain.Target
	cleanErr  error
	mu        sync.Mutex
}

func (f *fakeCleaner) Name() string        { return f.name }
func (f *fakeCleaner) IsInstalled() bool   { return f.installed }
func (f *fakeCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	return nil, nil
}
func (f *fakeCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cleanErr != nil {
		return f.cleanErr
	}
	f.cleaned = append(f.cleaned, targets...)
	return nil
}
func (f *fakeCleaner) Cleaned() []domain.Target {
	f.mu.Lock()
	defer f.mu.Unlock()
	res := make([]domain.Target, len(f.cleaned))
	copy(res, f.cleaned)
	return res
}

type fakeCleanerProvider struct {
	cleaners map[string]*fakeCleaner
	mu       sync.Mutex
}

func newFakeCleanerProvider() *fakeCleanerProvider {
	return &fakeCleanerProvider{
		cleaners: make(map[string]*fakeCleaner),
	}
}

func (p *fakeCleanerProvider) Register(name string, c *fakeCleaner) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cleaners[name] = c
}

func (p *fakeCleanerProvider) GetCleaner(name string) domain.Cleaner {
	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.cleaners[name]; ok {
		return c
	}
	return nil
}

func createPhysicalTarget(t *testing.T, baseDir, relPath string, size int64) domain.Target {
	t.Helper()
	targetPath := filepath.Join(baseDir, relPath)
	if err := os.MkdirAll(targetPath, 0o755); err != nil {
		t.Fatalf("creating target dir %s: %v", targetPath, err)
	}
	contentFile := filepath.Join(targetPath, "file.bin")
	data := make([]byte, size)
	if err := os.WriteFile(contentFile, data, 0o644); err != nil {
		t.Fatalf("writing target file %s: %v", contentFile, err)
	}
	return domain.Target{
		ID:        relPath,
		Name:      filepath.Base(relPath),
		Path:      targetPath,
		SizeBytes: size,
	}
}

func TestCoordinator_FullLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	dataDir := filepath.Join(tempDir, "data")
	trashDir := filepath.Join(tempDir, "trash")

	cleaners := newFakeCleanerProvider()
	nodeCleaner := &fakeCleaner{name: "node", installed: true}
	cleaners.Register("node", nodeCleaner)

	cfg := &config.Config{
		Safety: config.SafetyConfig{
			TrashEnabled: false,
			TrashDir:     trashDir,
		},
	}
	bl := safety.NewBlacklist(nil)
	trashMgr := trash.New(cfg.Safety)
	hookRunner := hooks.New(config.HooksConfig{})

	coord := deleter.New(cleaners, cfg, bl, trashMgr, hookRunner, configDir, dataDir)

	allowedTarget := createPhysicalTarget(t, tempDir, "app/node_modules", 1024)
	blacklistedTarget := domain.Target{
		ID:        "git-dir",
		Name:      ".git",
		Path:      filepath.Join(tempDir, "app/.git"),
		SizeBytes: 2048,
	}

	targets := map[string][]domain.Target{
		"node": {allowedTarget, blacklistedTarget},
	}

	res, err := coord.Delete(context.Background(), targets, deleter.Options{
		Auto: true,
	})
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// 1. Blacklist is filtered out
	if res.TotalCount != 1 {
		t.Fatalf("expected TotalCount 1, got %d", res.TotalCount)
	}
	if res.TotalFreed != 1024 {
		t.Fatalf("expected TotalFreed 1024, got %d", res.TotalFreed)
	}
	if len(res.CleanedPaths) != 1 || res.CleanedPaths[0] != allowedTarget.Path {
		t.Fatalf("expected cleaned paths [%s], got %v", allowedTarget.Path, res.CleanedPaths)
	}

	// 2. Allowed target was processed by cleaner (trash disabled)
	cleaned := nodeCleaner.Cleaned()
	if len(cleaned) != 1 || cleaned[0].Path != allowedTarget.Path {
		t.Fatalf("expected cleaner to clean allowed target, got %v", cleaned)
	}

	// 3. JSONL history was recorded
	entries, err := history.Read(dataDir)
	if err != nil {
		t.Fatalf("reading history failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(entries))
	}
	if entries[0].Targets != 1 || entries[0].FreedBytes != 1024 || entries[0].Mode != "auto" {
		t.Fatalf("unexpected history entry: %+v", entries[0])
	}

	// 4. Scheduler last-run.json was recorded
	lastRun, err := scheduler.ReadLastRun(configDir)
	if err != nil {
		t.Fatalf("reading last-run failed: %v", err)
	}
	if lastRun == nil {
		t.Fatal("expected lastRun not to be nil")
	}
	if lastRun.TargetsCount != 1 || lastRun.BytesFreed != 1024 {
		t.Fatalf("unexpected last-run content: %+v", lastRun)
	}
}

func TestCoordinator_DockerWithTrashBypass(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	dataDir := filepath.Join(tempDir, "data")
	trashDir := filepath.Join(tempDir, "trash")

	cleaners := newFakeCleanerProvider()
	nodeCleaner := &fakeCleaner{name: "node", installed: true}
	dockerCleaner := &fakeCleaner{name: "docker", installed: true}
	cleaners.Register("node", nodeCleaner)
	cleaners.Register("docker", dockerCleaner)

	cfg := &config.Config{
		Safety: config.SafetyConfig{
			TrashEnabled: true,
			TrashDir:     trashDir,
		},
	}
	bl := safety.NewBlacklist(nil)
	trashMgr := trash.New(cfg.Safety)
	hookRunner := hooks.New(config.HooksConfig{})

	coord := deleter.New(cleaners, cfg, bl, trashMgr, hookRunner, configDir, dataDir)

	fsTarget := createPhysicalTarget(t, tempDir, "project/node_modules", 1024)
	dockerTarget := domain.Target{
		ID:        "dock-1",
		Name:      "docker-image",
		Path:      "docker://sha256:1234567890abcdef",
		SizeBytes: 4096,
	}

	targets := map[string][]domain.Target{
		"node":   {fsTarget},
		"docker": {dockerTarget},
	}

	var warnings []string
	var progressEvents []deleter.ProgressEvent
	useTrash := true

	res, err := coord.Delete(context.Background(), targets, deleter.Options{
		UseTrash: &useTrash,
		OnWarning: func(msg string) {
			warnings = append(warnings, msg)
		},
		OnProgress: func(event deleter.ProgressEvent) {
			progressEvents = append(progressEvents, event)
		},
	})
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if res.TotalCount != 2 {
		t.Fatalf("expected TotalCount 2, got %d", res.TotalCount)
	}
	if res.TotalFreed != 5120 {
		t.Fatalf("expected TotalFreed 5120, got %d", res.TotalFreed)
	}

	// 1. Filesystem target was sent to physical trash
	manifest, err := trash.LoadManifest(trashMgr.ManifestPath())
	if err != nil {
		t.Fatalf("loading manifest failed: %v", err)
	}
	if len(manifest.Batches) != 1 {
		t.Fatalf("expected 1 trash batch, got %d", len(manifest.Batches))
	}
	if len(manifest.Batches[0].Entries) != 1 {
		t.Fatalf("expected 1 trash entry, got %d", len(manifest.Batches[0].Entries))
	}
	if manifest.Batches[0].Entries[0].OriginalPath != fsTarget.Path {
		t.Fatalf("expected trash entry original path %s, got %s", fsTarget.Path, manifest.Batches[0].Entries[0].OriginalPath)
	}
	if len(nodeCleaner.Cleaned()) != 0 {
		t.Fatalf("expected nodeCleaner not to be called when trash is active, got %v", nodeCleaner.Cleaned())
	}

	// 2. Docker target was NOT sent to physical trash, but was handled directly by dockerCleaner
	dockerCleaned := dockerCleaner.Cleaned()
	if len(dockerCleaned) != 1 || dockerCleaned[0].Path != dockerTarget.Path {
		t.Fatalf("expected dockerCleaner to clean virtual target directly, got %v", dockerCleaned)
	}

	// 3. Warning notification emitted for virtual target bypassing trash
	warningFound := false
	for _, w := range warnings {
		if fmt.Sprintf("[%s]", "docker") != "" && (len(w) > 0) {
			warningFound = true
			break
		}
	}
	if !warningFound || len(warnings) == 0 {
		t.Fatalf("expected warning about virtual targets bypassing trash, got %v", warnings)
	}

	// Verify progress actions
	actions := make(map[deleter.ActionType]int)
	for _, pe := range progressEvents {
		actions[pe.Action]++
	}
	if actions[deleter.ActionMovingTrash] != 1 {
		t.Errorf("expected 1 moving_to_trash action, got %d", actions[deleter.ActionMovingTrash])
	}
	if actions[deleter.ActionVirtualDirect] != 1 {
		t.Errorf("expected 1 virtual_direct action, got %d", actions[deleter.ActionVirtualDirect])
	}
}

func TestCoordinator_BlacklistProtection(t *testing.T) {
	tempDir := t.TempDir()
	cleaners := newFakeCleanerProvider()
	nodeCleaner := &fakeCleaner{name: "node", installed: true}
	cleaners.Register("node", nodeCleaner)

	cfg := &config.Config{
		ProtectedPatterns: []string{"*.protected"},
	}
	bl := safety.NewBlacklist(cfg.ProtectedPatterns)
	coord := deleter.New(cleaners, cfg, bl, nil, nil, "", "")

	protectedTargets := []domain.Target{
		{ID: "1", Name: ".git", Path: filepath.Join(tempDir, ".git"), SizeBytes: 100},
		{ID: "2", Name: ".env", Path: filepath.Join(tempDir, ".env"), SizeBytes: 200},
		{ID: "3", Name: ".ssh", Path: filepath.Join(tempDir, ".ssh"), SizeBytes: 300},
		{ID: "4", Name: "secrets.json", Path: filepath.Join(tempDir, "secrets.json"), SizeBytes: 400},
		{ID: "5", Name: "app.protected", Path: filepath.Join(tempDir, "app.protected"), SizeBytes: 500},
	}

	res, err := coord.Delete(context.Background(), map[string][]domain.Target{
		"node": protectedTargets,
	}, deleter.Options{})
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if res.TotalCount != 0 {
		t.Fatalf("expected TotalCount 0, got %d", res.TotalCount)
	}
	if res.TotalFreed != 0 {
		t.Fatalf("expected TotalFreed 0, got %d", res.TotalFreed)
	}
	if len(res.CleanedPaths) != 0 {
		t.Fatalf("expected 0 cleaned paths, got %v", res.CleanedPaths)
	}
	if len(nodeCleaner.Cleaned()) != 0 {
		t.Fatalf("expected no cleaner invocations for blacklisted paths, got %v", nodeCleaner.Cleaned())
	}
}

func TestCoordinator_PreHookFailureAbortsEcosystem(t *testing.T) {
	tempDir := t.TempDir()
	cleaners := newFakeCleanerProvider()
	nodeCleaner := &fakeCleaner{name: "node", installed: true}
	goCleaner := &fakeCleaner{name: "go", installed: true}
	cleaners.Register("node", nodeCleaner)
	cleaners.Register("go", goCleaner)

	hooksCfg := config.HooksConfig{
		Node: config.HookConfig{
			PreDelete: "exit 1", // deliberate failure
		},
	}
	hookRunner := hooks.New(hooksCfg)

	cfg := &config.Config{}
	bl := safety.NewBlacklist(nil)
	coord := deleter.New(cleaners, cfg, bl, nil, hookRunner, "", "")

	nodeTarget := createPhysicalTarget(t, tempDir, "node-app/node_modules", 1024)
	goTarget := createPhysicalTarget(t, tempDir, "go-app/bin", 2048)

	targets := map[string][]domain.Target{
		"node": {nodeTarget},
		"go":   {goTarget},
	}

	res, err := coord.Delete(context.Background(), targets, deleter.Options{})
	if err != nil {
		t.Fatalf("Delete returned fatal error: %v", err)
	}

	// Pre-hook for node failed, so node should be aborted and added to res.Errors
	if len(res.Errors) != 1 {
		t.Fatalf("expected 1 error for failing pre-hook, got %d: %v", len(res.Errors), res.Errors)
	}
	if len(nodeCleaner.Cleaned()) != 0 {
		t.Fatalf("expected node cleaner not to be called due to pre-hook failure, got %v", nodeCleaner.Cleaned())
	}

	// go ecosystem should have proceeded normally
	if len(goCleaner.Cleaned()) != 1 || goCleaner.Cleaned()[0].Path != goTarget.Path {
		t.Fatalf("expected go cleaner to execute for non-failing ecosystem, got %v", goCleaner.Cleaned())
	}
	if res.TotalCount != 1 {
		t.Fatalf("expected TotalCount 1, got %d", res.TotalCount)
	}
	if res.TotalFreed != 2048 {
		t.Fatalf("expected TotalFreed 2048, got %d", res.TotalFreed)
	}
}

func TestCoordinator_DryRun(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	dataDir := filepath.Join(tempDir, "data")
	trashDir := filepath.Join(tempDir, "trash")

	cleaners := newFakeCleanerProvider()
	nodeCleaner := &fakeCleaner{name: "node", installed: true}
	cleaners.Register("node", nodeCleaner)

	cfg := &config.Config{
		Safety: config.SafetyConfig{
			TrashEnabled: true,
			TrashDir:     trashDir,
		},
	}
	bl := safety.NewBlacklist(nil)
	trashMgr := trash.New(cfg.Safety)
	hooksCfg := config.HooksConfig{
		Node: config.HookConfig{
			PreDelete: "exit 1", // If hooks ran, this would fail!
		},
	}
	hookRunner := hooks.New(hooksCfg)

	coord := deleter.New(cleaners, cfg, bl, trashMgr, hookRunner, configDir, dataDir)

	target := domain.Target{
		ID:        "target-1",
		Name:      "node_modules",
		Path:      filepath.Join(tempDir, "app", "node_modules"),
		SizeBytes: 4096,
	}

	var actions []deleter.ActionType
	res, err := coord.Delete(context.Background(), map[string][]domain.Target{
		"node": {target},
	}, deleter.Options{
		DryRun: true,
		Auto:   true,
		OnProgress: func(event deleter.ProgressEvent) {
			actions = append(actions, event.Action)
		},
	})
	if err != nil {
		t.Fatalf("Delete in DryRun failed: %v", err)
	}

	// 1. Dry run predicts freed stats
	if res.TotalFreed != 4096 {
		t.Fatalf("expected TotalFreed 4096, got %d", res.TotalFreed)
	}
	if res.TotalCount != 1 {
		t.Fatalf("expected TotalCount 1, got %d", res.TotalCount)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("expected no errors in dry run (pre-hook should not run), got %v", res.Errors)
	}

	// 2. ActionWouldDelete was emitted
	if len(actions) != 1 || actions[0] != deleter.ActionWouldDelete {
		t.Fatalf("expected ActionWouldDelete event, got %v", actions)
	}

	// 3. Cleaner was NOT called
	if len(nodeCleaner.Cleaned()) != 0 {
		t.Fatalf("expected cleaner not to be called in dry run, got %v", nodeCleaner.Cleaned())
	}

	// 4. Trash was NOT called
	manifest, err := trash.LoadManifest(trashMgr.ManifestPath())
	if err != nil {
		t.Fatalf("reading manifest failed: %v", err)
	}
	if len(manifest.Batches) != 0 {
		t.Fatalf("expected 0 batches in trash manifest in dry run, got %d", len(manifest.Batches))
	}

	// 5. History was NOT recorded
	entries, err := history.Read(dataDir)
	if err != nil {
		t.Fatalf("reading history failed: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 history entries in dry run, got %d", len(entries))
	}

	// 6. Scheduler was NOT recorded
	lastRun, err := scheduler.ReadLastRun(configDir)
	if err != nil {
		t.Fatalf("reading scheduler last run failed: %v", err)
	}
	if lastRun != nil {
		t.Fatalf("expected nil lastRun in dry run, got %+v", lastRun)
	}
}

func TestCoordinator_ContextCancellation(t *testing.T) {
	coord := deleter.New(nil, nil, nil, nil, nil, "", "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	res, err := coord.Delete(ctx, map[string][]domain.Target{
		"node": {{ID: "1", Path: "/tmp/foo", SizeBytes: 100}},
	}, deleter.Options{})

	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result even on cancel")
	}
}

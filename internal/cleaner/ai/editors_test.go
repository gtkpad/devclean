package ai

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestEditors_8Editors_Presence(t *testing.T) {
	home := t.TempDir()
	mockExec := newMockExecutor()

	allEditorIDs := []string{
		"code", "code-insiders", "cursor", "antigravity",
		"windsurf", "claude-desktop", "opencode", "zed",
	}

	// 1. Create mock leaves for each editor based on platform
	for _, id := range allEditorIDs {
		switch id {
		case "code":
			setupMockEditor(t, home, "Code", "com.microsoft.VSCode")
		case "code-insiders":
			setupMockEditor(t, home, "Code - Insiders", "com.microsoft.VSCodeInsiders")
		case "cursor":
			setupMockEditor(t, home, "Cursor", "Cursor")
		case "antigravity":
			setupMockEditor(t, home, "Antigravity", "Antigravity")
		case "windsurf":
			setupMockEditor(t, home, "Windsurf", "Windsurf")
		case "claude-desktop":
			setupMockEditor(t, home, "Claude", "com.anthropic.claudefordesktop")
		case "opencode":
			setupMockEditor(t, home, "opencode", "opencode")
		case "zed":
			setupMockZed(t, home)
		}
	}

	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}
	targets, err := scanEditors(context.Background(), mockExec, home, rules)
	if err != nil {
		t.Fatalf("scanEditors failed: %v", err)
	}

	foundChromium := make(map[string]bool)
	foundLogs := make(map[string]bool)

	for _, target := range targets {
		if !target.IsVirtual() {
			t.Errorf("expected virtual target, got %s", target.Path)
		}
		if target.Risk != domain.RiskLow {
			t.Errorf("expected RiskLow for %s, got %v", target.Path, target.Risk)
		}

		parts := strings.Split(strings.TrimPrefix(target.Path, "editor://"), "/")
		if len(parts) != 2 {
			t.Errorf("unexpected target path format: %s", target.Path)
			continue
		}
		edID, cat := parts[0], parts[1]

		if cat == "chromium-cache" {
			foundChromium[edID] = true
			if target.Category != "editor-chromium-cache" {
				t.Errorf("expected category editor-chromium-cache, got %s", target.Category)
			}
			if !strings.Contains(target.Name, "Chromium & GPU Caches") {
				t.Errorf("expected name to contain 'Chromium & GPU Caches', got %s", target.Name)
			}
		}
		if cat == "logs" {
			foundLogs[edID] = true
			if target.Category != "editor-logs" {
				t.Errorf("expected category editor-logs, got %s", target.Category)
			}
			if !strings.Contains(target.Name, "Logs & Crash Reports") {
				t.Errorf("expected name to contain 'Logs & Crash Reports', got %s", target.Name)
			}
		}
	}

	for _, edID := range allEditorIDs {
		if !foundChromium[edID] {
			t.Errorf("missing chromium-cache target for editor %s", edID)
		}
		if !foundLogs[edID] {
			t.Errorf("missing logs target for editor %s", edID)
		}
	}
}

func TestEditors_FunctionalGrouping(t *testing.T) {
	home := t.TempDir()
	mockExec := newMockExecutor()

	// Setup Code with multiple Chromium cache leaves and logs
	var appDir string
	if runtime.GOOS == "darwin" {
		appDir = filepath.Join(home, "Library", "Application Support", "Code")
	} else {
		appDir = filepath.Join(home, ".config", "Code")
	}

	leaves := []string{
		"Cache", "Code Cache", "GPUCache", "DawnCache", "DawnWebGPUCache",
		"DawnGraphiteCache", "CachedData", "webviewCache",
	}
	for _, leaf := range leaves {
		leafDir := filepath.Join(appDir, leaf)
		if err := os.MkdirAll(leafDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(leafDir, "data.bin"), []byte("data-bytes"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Logs leaves
	logLeaves := []string{"logs", "Crashpad", "Sentry"}
	for _, leaf := range logLeaves {
		leafDir := filepath.Join(appDir, leaf)
		if err := os.MkdirAll(leafDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(leafDir, "log.txt"), []byte("log-bytes"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}
	targets, err := scanEditors(context.Background(), mockExec, home, rules)
	if err != nil {
		t.Fatalf("scanEditors failed: %v", err)
	}

	codeTargets := 0
	for _, target := range targets {
		if strings.HasPrefix(target.Path, "editor://code/") {
			codeTargets++
		}
	}

	// Must be exactly 2 consolidated targets (chromium-cache and logs) per D-01
	if codeTargets != 2 {
		t.Fatalf("expected exactly 2 consolidated targets for Code, got %d", codeTargets)
	}
}

func TestEditors_ProtectedPaths_Untouched(t *testing.T) {
	home := t.TempDir()
	mockExec := newMockExecutor()

	var appDir string
	if runtime.GOOS == "darwin" {
		appDir = filepath.Join(home, "Library", "Application Support", "Code")
	} else {
		appDir = filepath.Join(home, ".config", "Code")
	}

	// 1. Create candidate leaf
	cacheDir := filepath.Join(appDir, "Cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cacheFile := filepath.Join(cacheDir, "cache.dat")
	cacheData := []byte("cache-data-bytes-12345")
	if err := os.WriteFile(cacheFile, cacheData, 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. Create protected files and directories that must never be counted or targeted
	protectedFiles := map[string][]byte{
		filepath.Join(appDir, "settings.json"):                   []byte(`{"editor.tabSize": 2}`),
		filepath.Join(appDir, "keybindings.json"):                []byte(`[]`),
		filepath.Join(appDir, "Cookies"):                         []byte(`cookie-binary-secret`),
		filepath.Join(appDir, "Cookies-journal"):                 []byte(`cookie-journal`),
		filepath.Join(appDir, "User", "settings.json"):           []byte(`{"editor.fontSize": 14}`),
		filepath.Join(appDir, "Local Storage", "leveldb", "log"): []byte(`local-storage-state`),
		filepath.Join(appDir, "snippets", "go.json"):             []byte(`{}`),
	}

	for p, content := range protectedFiles {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}
	targets, err := scanEditors(context.Background(), mockExec, home, rules)
	if err != nil {
		t.Fatalf("scanEditors failed: %v", err)
	}

	var foundChromium bool
	for _, target := range targets {
		if target.Path == "editor://code/chromium-cache" {
			foundChromium = true
			if target.SizeBytes != int64(len(cacheData)) {
				t.Errorf("expected SizeBytes to strictly match cache size %d, got %d", len(cacheData), target.SizeBytes)
			}
		}
		// Assert no protected target was emitted
		for p := range protectedFiles {
			if target.Path == p {
				t.Fatalf("SECURITY VIOLATION: protected path %s emitted as target!", p)
			}
		}
	}

	if !foundChromium {
		t.Fatalf("expected editor://code/chromium-cache target to be emitted")
	}
}

func TestEditors_TTL_Decoupling(t *testing.T) {
	home := t.TempDir()
	mockExec := newMockExecutor()

	var appDir string
	if runtime.GOOS == "darwin" {
		appDir = filepath.Join(home, "Library", "Application Support", "Cursor")
	} else {
		appDir = filepath.Join(home, ".config", "Cursor")
	}

	// Old cache leaf (60 days ago)
	oldCacheDir := filepath.Join(appDir, "Cache")
	if err := os.MkdirAll(oldCacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldFile := filepath.Join(oldCacheDir, "data.bin")
	if err := os.WriteFile(oldFile, []byte("old-cache-payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().AddDate(0, 0, -60)
	_ = os.Chtimes(oldFile, oldTime, oldTime)
	_ = os.Chtimes(oldCacheDir, oldTime, oldTime)

	// Recent cache leaf (2 days ago)
	recentCacheDir := filepath.Join(appDir, "GPUCache")
	if err := os.MkdirAll(recentCacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	recentFile := filepath.Join(recentCacheDir, "gpu.bin")
	if err := os.WriteFile(recentFile, []byte("recent-gpu-cache"), 0o644); err != nil {
		t.Fatal(err)
	}
	recentTime := time.Now().AddDate(0, 0, -2)
	_ = os.Chtimes(recentFile, recentTime, recentTime)
	_ = os.Chtimes(recentCacheDir, recentTime, recentTime)

	totalExpectedBytes := int64(len("old-cache-payload") + len("recent-gpu-cache"))
	oldExpectedBytes := int64(len("old-cache-payload"))

	// 1. MaxAgeDays = 0: All bytes are eligible
	rules0 := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}
	targets0, err := scanEditors(context.Background(), mockExec, home, rules0)
	if err != nil {
		t.Fatalf("scanEditors failed: %v", err)
	}

	var found0 bool
	for _, target := range targets0 {
		if target.Path == "editor://cursor/chromium-cache" {
			found0 = true
			if target.SizeBytes != totalExpectedBytes {
				t.Errorf("SizeBytes = %d, want %d", target.SizeBytes, totalExpectedBytes)
			}
			if target.EligibleBytes != totalExpectedBytes {
				t.Errorf("EligibleBytes = %d, want %d", target.EligibleBytes, totalExpectedBytes)
			}
			if !target.TTLEligible {
				t.Errorf("expected TTLEligible = true for MaxAgeDays = 0")
			}
		}
	}
	if !found0 {
		t.Fatalf("target editor://cursor/chromium-cache not found in rules0")
	}

	// 2. MaxAgeDays = 30: 100% bytes reported in SizeBytes, only old leaf in EligibleBytes
	rules30 := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}
	targets30, err := scanEditors(context.Background(), mockExec, home, rules30)
	if err != nil {
		t.Fatalf("scanEditors failed: %v", err)
	}

	var found30 bool
	for _, target := range targets30 {
		if target.Path == "editor://cursor/chromium-cache" {
			found30 = true
			if target.SizeBytes != totalExpectedBytes {
				t.Errorf("SizeBytes = %d, want 100%% of bytes %d", target.SizeBytes, totalExpectedBytes)
			}
			if target.EligibleBytes != oldExpectedBytes {
				t.Errorf("EligibleBytes = %d, want %d", target.EligibleBytes, oldExpectedBytes)
			}
			if !target.TTLEligible {
				t.Errorf("expected TTLEligible = true because older leaf exists")
			}
		}
	}
	if !found30 {
		t.Fatalf("target editor://cursor/chromium-cache not found in rules30")
	}
}

// Helpers for test setup
func setupMockEditor(t *testing.T, home, dirName, cacheBundle string) {
	var appDir, cacheDir string
	if runtime.GOOS == "darwin" {
		appDir = filepath.Join(home, "Library", "Application Support", dirName)
		cacheDir = filepath.Join(home, "Library", "Caches", cacheBundle)
	} else {
		appDir = filepath.Join(home, ".config", dirName)
		cacheDir = filepath.Join(home, ".cache", dirName)
	}

	// Chromium cache leaf
	cLeaf := filepath.Join(appDir, "Cache")
	if err := os.MkdirAll(cLeaf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cLeaf, "cache.bin"), []byte("sample-cache"), 0o644); err != nil {
		t.Fatal(err)
	}

	// OS cache
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "data.bin"), []byte("sample-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Logs leaf
	lLeaf := filepath.Join(appDir, "logs")
	if err := os.MkdirAll(lLeaf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lLeaf, "app.log"), []byte("sample-log"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setupMockZed(t *testing.T, home string) {
	var cacheDir, logDir string
	if runtime.GOOS == "darwin" {
		cacheDir = filepath.Join(home, "Library", "Caches", "dev.zed.Zed")
		logDir = filepath.Join(home, "Library", "Logs", "Zed")
	} else {
		cacheDir = filepath.Join(home, ".cache", "zed")
		logDir = filepath.Join(home, ".local", "state", "zed", "logs")
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "cache.bin"), []byte("zed-cache"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "zed.log"), []byte("zed-log"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEditors_WorkspaceStorage_SurgicalBackup(t *testing.T) {
	home := t.TempDir()
	mockExec := newMockExecutor()

	var appDir string
	if runtime.GOOS == "darwin" {
		appDir = filepath.Join(home, "Library", "Application Support", "Code")
	} else {
		appDir = filepath.Join(home, ".config", "Code")
	}

	wsDir1 := filepath.Join(appDir, "User", "workspaceStorage", "ws-001")
	wsDir2 := filepath.Join(appDir, "User", "workspaceStorage", "ws-002")

	if err := os.MkdirAll(wsDir1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wsDir2, 0o755); err != nil {
		t.Fatal(err)
	}

	// Workspace 1: state.vscdb (8192 bytes), workspace.json (512 bytes), state.vscdb.backup (4096 bytes)
	if err := os.WriteFile(filepath.Join(wsDir1, "state.vscdb"), make([]byte, 8192), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsDir1, "workspace.json"), make([]byte, 512), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsDir1, "state.vscdb.backup"), make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}

	// Workspace 2: state.vscdb (16384 bytes), workspace.json (512 bytes), no backup
	if err := os.WriteFile(filepath.Join(wsDir2, "state.vscdb"), make([]byte, 16384), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsDir2, "workspace.json"), make([]byte, 512), 0o644); err != nil {
		t.Fatal(err)
	}

	// Also setup a mock editor with hasWorkspaces=false (e.g. Zed) to verify it never emits workspace-backups
	setupMockZed(t, home)

	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}
	targets, err := scanEditors(context.Background(), mockExec, home, rules)
	if err != nil {
		t.Fatalf("scanEditors failed: %v", err)
	}

	var foundBackupTarget bool
	for _, target := range targets {
		if strings.Contains(target.Name, "state.vscdb") || strings.Contains(target.Path, "state.vscdb") {
			t.Fatalf("SECURITY VIOLATION: primary state database state.vscdb exposed in target %s (%s)", target.Name, target.Path)
		}
		if strings.Contains(target.Name, "workspace.json") || strings.Contains(target.Path, "workspace.json") {
			t.Fatalf("SECURITY VIOLATION: workspace.json exposed in target %s (%s)", target.Name, target.Path)
		}

		if target.Path == "editor://zed/workspace-backups" {
			t.Fatalf("editor without workspaces emitted workspace-backups target: %s", target.Path)
		}

		if target.Path == "editor://code/workspace-backups" {
			foundBackupTarget = true
			if target.Category != "editor-workspace-backups" {
				t.Errorf("expected category editor-workspace-backups, got %s", target.Category)
			}
			if target.Risk != domain.RiskMedium {
				t.Errorf("expected RiskMedium, got %v", target.Risk)
			}
			if target.SizeBytes != 4096 {
				t.Errorf("expected SizeBytes == 4096, got %d", target.SizeBytes)
			}
			if target.EligibleBytes != 4096 {
				t.Errorf("expected EligibleBytes == 4096, got %d", target.EligibleBytes)
			}
			if !target.TTLEligible {
				t.Errorf("expected TTLEligible == true for MaxAgeDays = 0")
			}
			if !strings.Contains(target.Name, "Workspace State Backups") {
				t.Errorf("expected Name to contain 'Workspace State Backups', got %s", target.Name)
			}
		}
	}

	if !foundBackupTarget {
		t.Fatalf("expected editor://code/workspace-backups target was not found")
	}
}


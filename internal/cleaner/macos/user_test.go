package macos

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestUserCaches_DeduplicationExclusions(t *testing.T) {
	// Table of folders that MUST be excluded
	excludedDirs := []string{
		"pnpm",
		"uv",
		"pip",
		"Homebrew",
		"CocoaPods",
		"vcpkg",
		"mise",
		"gh",
		"ms-playwright",
		"Cypress",
		"node",
		"Yarn",
		"dev.orbstack.OrbStack",
		"com.microsoft.VSCode",
		"com.microsoft.VSCodeInsiders",
		"Cursor",
		"Antigravity",
		"com.google.antigravity",
		"com.google.antigravity.editor",
		"Windsurf",
		"com.exafunction.windsurf",
		"com.anthropic.claudefordesktop",
		"opencode",
		"dev.zed.Zed",
		"Google",
		"BraveSoftware",
		"company.thebrowser.Arc",
		"Microsoft Edge",
		"Firefox",
		"com.apple.Safari",
	}

	for _, dir := range excludedDirs {
		if !isExcludedByDedicatedCleaner(dir) {
			t.Errorf("expected dir %q to be excluded by dedicated cleaner, but was not", dir)
		}
	}

	// Table of items that must NOT be excluded
	retainedDirs := []string{
		"com.spotify.client",
		"com.apple.helpd",
		"com.google.Chrome",
		"Slack",
		"random-custom-app",
	}

	for _, dir := range retainedDirs {
		if isExcludedByDedicatedCleaner(dir) {
			t.Errorf("expected dir %q NOT to be excluded by dedicated cleaner, but was", dir)
		}
	}

	// Hermetic filesystem test
	tempHome := t.TempDir()
	cachesDir := filepath.Join(tempHome, "Library", "Caches")
	if err := os.MkdirAll(cachesDir, 0755); err != nil {
		t.Fatalf("failed to create mock caches dir: %v", err)
	}

	// Populate excluded dirs
	for _, dir := range []string{"pnpm", "uv", "Homebrew", "Cursor", "com.microsoft.VSCode"} {
		dirPath := filepath.Join(cachesDir, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dirPath, "data.bin"), []byte("some-data"), 0644); err != nil {
			t.Fatalf("failed to write dummy data: %v", err)
		}
	}

	ctx := context.Background()
	mock := newMockProcessExecutor()
	targets, err := scanUserCaches(ctx, mock, tempHome)
	if err != nil {
		t.Fatalf("scanUserCaches failed: %v", err)
	}

	for _, target := range targets {
		for _, excluded := range []string{"pnpm", "uv", "Homebrew", "Cursor", "com.microsoft.VSCode"} {
			if target.ID == "macos-cache-"+excluded || target.Path == filepath.Join(cachesDir, excluded) {
				t.Errorf("found target %v for excluded dir %s", target, excluded)
			}
		}
	}
	if len(targets) != 0 {
		t.Errorf("expected 0 targets when all caches are excluded, got %d targets: %+v", len(targets), targets)
	}
}

func TestUserCaches_ConsolidationThreshold(t *testing.T) {
	tempHome := t.TempDir()
	cachesDir := filepath.Join(tempHome, "Library", "Caches")
	if err := os.MkdirAll(cachesDir, 0755); err != nil {
		t.Fatalf("failed to create mock caches dir: %v", err)
	}

	// Small app <= 50MB (e.g. 10KB)
	smallDir := filepath.Join(cachesDir, "com.small.app")
	if err := os.MkdirAll(smallDir, 0755); err != nil {
		t.Fatalf("failed to create small app dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(smallDir, "cache.db"), make([]byte, 10240), 0644); err != nil {
		t.Fatalf("failed to write small cache file: %v", err)
	}

	// Another small app <= 50MB (e.g. 20KB)
	anotherSmall := filepath.Join(cachesDir, "com.another.small")
	if err := os.MkdirAll(anotherSmall, 0755); err != nil {
		t.Fatalf("failed to create another small dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(anotherSmall, "blob.dat"), make([]byte, 20480), 0644); err != nil {
		t.Fatalf("failed to write blob file: %v", err)
	}

	// Large app > 50MB (create sparse file of 60MB)
	largeDir := filepath.Join(cachesDir, "com.large.app")
	if err := os.MkdirAll(largeDir, 0755); err != nil {
		t.Fatalf("failed to create large dir: %v", err)
	}
	largeFile, err := os.Create(filepath.Join(largeDir, "huge.data"))
	if err != nil {
		t.Fatalf("failed to create large file: %v", err)
	}
	const sixtyMB = 60 * 1024 * 1024
	if err := largeFile.Truncate(sixtyMB); err != nil {
		_ = largeFile.Close()
		t.Fatalf("failed to truncate large file: %v", err)
	}
	_ = largeFile.Close()

	ctx := context.Background()
	mock := newMockProcessExecutor()
	targets, err := scanUserCaches(ctx, mock, tempHome)
	if err != nil {
		t.Fatalf("scanUserCaches failed: %v", err)
	}

	var foundConsolidated bool
	var foundLargeIndividual bool

	for _, target := range targets {
		if target.ID == "macos-user-app-caches-consolidated" {
			foundConsolidated = true
			if target.Category != "macos-app-caches" {
				t.Errorf("expected Category macos-app-caches, got %s", target.Category)
			}
			if target.ReviewOnly {
				t.Errorf("expected consolidated target to not be ReviewOnly")
			}
			if !target.TTLEligible {
				t.Errorf("expected consolidated target to be TTLEligible")
			}
			if target.SizeBytes < 30720 {
				t.Errorf("expected consolidated SizeBytes >= 30720, got %d", target.SizeBytes)
			}
		}
		if target.ID == "macos-cache-com.large.app" {
			foundLargeIndividual = true
			if target.Category != "macos-app-caches" {
				t.Errorf("expected Category macos-app-caches, got %s", target.Category)
			}
			if target.SizeBytes < sixtyMB {
				t.Errorf("expected large app SizeBytes >= %d, got %d", sixtyMB, target.SizeBytes)
			}
			if target.EligibleBytes < sixtyMB {
				t.Errorf("expected large app EligibleBytes >= %d, got %d", sixtyMB, target.EligibleBytes)
			}
		}
		if target.ID == "macos-cache-com.small.app" || target.ID == "macos-cache-com.another.small" {
			t.Errorf("small bundle %s should have been consolidated, but was surfaced individually", target.ID)
		}
	}

	if !foundConsolidated {
		t.Errorf("expected consolidated target macos-user-app-caches-consolidated, but not found in targets: %+v", targets)
	}
	if !foundLargeIndividual {
		t.Errorf("expected individual target macos-cache-com.large.app, but not found in targets: %+v", targets)
	}
}

func TestUserCaches_ActiveProcessSurfacedIndividually(t *testing.T) {
	tempHome := t.TempDir()
	cachesDir := filepath.Join(tempHome, "Library", "Caches")
	if err := os.MkdirAll(cachesDir, 0755); err != nil {
		t.Fatalf("failed to create mock caches dir: %v", err)
	}

	// Active small app <= 50MB (e.g. 5KB)
	activeDir := filepath.Join(cachesDir, "com.active.app")
	if err := os.MkdirAll(activeDir, 0755); err != nil {
		t.Fatalf("failed to create active app dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(activeDir, "cache.db"), make([]byte, 5120), 0644); err != nil {
		t.Fatalf("failed to write active app cache: %v", err)
	}

	// Inactive small app <= 50MB (e.g. 5KB)
	inactiveDir := filepath.Join(cachesDir, "com.inactive.app")
	if err := os.MkdirAll(inactiveDir, 0755); err != nil {
		t.Fatalf("failed to create inactive app dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inactiveDir, "cache.db"), make([]byte, 5120), 0644); err != nil {
		t.Fatalf("failed to write inactive app cache: %v", err)
	}

	ctx := context.Background()
	mock := newMockProcessExecutor()
	// Fallback process name for com.active.app will query "active.app"
	mock.runningProcesses["active.app"] = true

	targets, err := scanUserCaches(ctx, mock, tempHome)
	if err != nil {
		t.Fatalf("scanUserCaches failed: %v", err)
	}

	var foundActiveTarget bool
	for _, target := range targets {
		if target.ID == "macos-cache-com.active.app" {
			foundActiveTarget = true
			if !target.ReviewOnly {
				t.Errorf("expected active app target to be ReviewOnly: true, got false")
			}
			if target.ReviewReason != domain.ReviewReasonActiveProcess {
				t.Errorf("expected ReviewReason %q, got %q", domain.ReviewReasonActiveProcess, target.ReviewReason)
			}
			if target.EligibleBytes != 0 {
				t.Errorf("expected EligibleBytes == 0 for active app, got %d", target.EligibleBytes)
			}
			if target.TTLEligible {
				t.Errorf("expected TTLEligible == false for active app, got true")
			}
		}
		if target.ID == "macos-cache-com.inactive.app" {
			t.Errorf("inactive small bundle should have been consolidated, not surfaced individually")
		}
	}

	if !foundActiveTarget {
		t.Fatalf("expected individual target for active app com.active.app, but not found in %+v", targets)
	}
}

func TestUserCaches_WhitelistedSkipped(t *testing.T) {
	tempHome := t.TempDir()
	cachesDir := filepath.Join(tempHome, "Library", "Caches")
	if err := os.MkdirAll(cachesDir, 0755); err != nil {
		t.Fatalf("failed to create mock caches dir: %v", err)
	}

	// Create Safari and Keychain caches
	safariDir := filepath.Join(cachesDir, "com.apple.Safari")
	if err := os.MkdirAll(safariDir, 0755); err != nil {
		t.Fatalf("failed to create safari dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(safariDir, "cache.db"), []byte("safari-data"), 0644); err != nil {
		t.Fatalf("failed to write safari cache: %v", err)
	}

	keychainDir := filepath.Join(cachesDir, "com.apple.keychainaccess")
	if err := os.MkdirAll(keychainDir, 0755); err != nil {
		t.Fatalf("failed to create keychain dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(keychainDir, "data.bin"), []byte("keychain-data"), 0644); err != nil {
		t.Fatalf("failed to write keychain cache: %v", err)
	}

	ctx := context.Background()
	mock := newMockProcessExecutor()
	targets, err := scanUserCaches(ctx, mock, tempHome)
	if err != nil {
		t.Fatalf("scanUserCaches failed: %v", err)
	}

	if len(targets) != 0 {
		t.Errorf("expected whitelisted caches to be completely skipped (0 targets), got %d: %+v", len(targets), targets)
	}
}

func TestUserCaches_SymlinkEscape(t *testing.T) {
	tempHome := t.TempDir()
	cachesDir := filepath.Join(tempHome, "Library", "Caches")
	if err := os.MkdirAll(cachesDir, 0755); err != nil {
		t.Fatalf("failed to create mock caches dir: %v", err)
	}

	outsideDir := t.TempDir()
	outsideTarget := filepath.Join(outsideDir, "evil-escape")
	if err := os.MkdirAll(outsideTarget, 0755); err != nil {
		t.Fatalf("failed to create outside target dir: %v", err)
	}

	linkPath := filepath.Join(cachesDir, "com.escaping.link")
	if err := os.Symlink(outsideTarget, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	ctx := context.Background()
	mock := newMockProcessExecutor()
	targets, err := scanUserCaches(ctx, mock, tempHome)
	if err != nil {
		t.Fatalf("scanUserCaches failed: %v", err)
	}

	var foundEscapeTarget bool
	for _, target := range targets {
		if target.ID == "macos-cache-symlink-com.escaping.link" {
			foundEscapeTarget = true
			if !target.ReviewOnly {
				t.Errorf("expected escaping symlink to be ReviewOnly: true")
			}
			if target.ReviewReason != domain.ReviewReasonAmbiguous {
				t.Errorf("expected ReviewReasonAmbiguous, got %s", target.ReviewReason)
			}
			if target.Risk != domain.RiskMedium {
				t.Errorf("expected RiskMedium, got %s", target.Risk)
			}
		}
	}

	if !foundEscapeTarget {
		t.Fatalf("expected symlink escape target, but not found in %+v", targets)
	}
}

func TestUserLogs_Consolidation(t *testing.T) {
	tempHome := t.TempDir()
	logsDir := filepath.Join(tempHome, "Library", "Logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("failed to create mock logs dir: %v", err)
	}

	// Small log <= 50MB (10KB)
	smallLog := filepath.Join(logsDir, "app.log")
	if err := os.WriteFile(smallLog, make([]byte, 10240), 0644); err != nil {
		t.Fatalf("failed to write small log: %v", err)
	}

	// DiagnosticReports folder (must be excluded from scanUserLogs)
	diagDir := filepath.Join(logsDir, "DiagnosticReports")
	if err := os.MkdirAll(diagDir, 0755); err != nil {
		t.Fatalf("failed to create DiagnosticReports dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(diagDir, "crash.ips"), []byte("crash-report"), 0644); err != nil {
		t.Fatalf("failed to write crash report: %v", err)
	}

	// Large log > 50MB (create sparse file of 60MB)
	largeLog := filepath.Join(logsDir, "com.huge.log")
	f, err := os.Create(largeLog)
	if err != nil {
		t.Fatalf("failed to create large log file: %v", err)
	}
	const sixtyMB = 60 * 1024 * 1024
	if err := f.Truncate(sixtyMB); err != nil {
		_ = f.Close()
		t.Fatalf("failed to truncate large log file: %v", err)
	}
	_ = f.Close()

	ctx := context.Background()
	targets, err := scanUserLogs(ctx, tempHome)
	if err != nil {
		t.Fatalf("scanUserLogs failed: %v", err)
	}

	var foundConsolidated bool
	var foundLargeIndividual bool

	for _, target := range targets {
		if target.ID == "macos-user-app-logs-consolidated" {
			foundConsolidated = true
			if target.Category != "macos-app-logs" {
				t.Errorf("expected Category macos-app-logs, got %s", target.Category)
			}
			if target.ReviewOnly {
				t.Errorf("expected consolidated logs to not be ReviewOnly")
			}
			if target.SizeBytes < 10240 {
				t.Errorf("expected consolidated SizeBytes >= 10240, got %d", target.SizeBytes)
			}
		}
		if target.ID == "macos-log-com.huge.log" {
			foundLargeIndividual = true
			if target.Category != "macos-app-logs" {
				t.Errorf("expected Category macos-app-logs, got %s", target.Category)
			}
			if target.SizeBytes < sixtyMB {
				t.Errorf("expected large log SizeBytes >= %d, got %d", sixtyMB, target.SizeBytes)
			}
		}
		if target.ID == "macos-log-DiagnosticReports" || target.Path == diagDir {
			t.Errorf("DiagnosticReports must be excluded from scanUserLogs, but was found: %+v", target)
		}
	}

	if !foundConsolidated {
		t.Errorf("expected consolidated logs target macos-user-app-logs-consolidated, but not found in %+v", targets)
	}
	if !foundLargeIndividual {
		t.Errorf("expected individual target macos-log-com.huge.log, but not found in %+v", targets)
	}
}

func TestUserCachesAndLogs_NonExistentDirs(t *testing.T) {
	tempHome := t.TempDir() // Library doesn't even exist
	ctx := context.Background()
	mock := newMockProcessExecutor()

	caches, err := scanUserCaches(ctx, mock, tempHome)
	if err != nil {
		t.Errorf("expected nil error on non-existent caches dir, got %v", err)
	}
	if len(caches) != 0 {
		t.Errorf("expected empty targets on non-existent caches dir, got %d", len(caches))
	}

	logs, err := scanUserLogs(ctx, tempHome)
	if err != nil {
		t.Errorf("expected nil error on non-existent logs dir, got %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("expected empty targets on non-existent logs dir, got %d", len(logs))
	}
}

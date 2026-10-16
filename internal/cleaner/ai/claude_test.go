package ai

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)

func TestClaude_ActiveVersion_Preserved(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Setup ~/.local/share/claude/versions/ with versions 2.1.299 and 2.1.300
	versionsDir := filepath.Join(tmpDir, ".local", "share", "claude", "versions")
	v299 := filepath.Join(versionsDir, "2.1.299")
	v300 := filepath.Join(versionsDir, "2.1.300")

	if err := os.MkdirAll(v299, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v299, "claude"), []byte("bin-2.1.299"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(v300, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v300, "claude"), []byte("bin-2.1.300"), 0o755); err != nil {
		t.Fatal(err)
	}

	// 2. Setup active symlink in ~/.local/bin/claude pointing to versions/2.1.300
	binDir := filepath.Join(tmpDir, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkPath := filepath.Join(binDir, "claude")
	if err := os.Symlink(v300, symlinkPath); err != nil {
		t.Fatal(err)
	}

	// 3. Scan Claude with MaxAgeDays = 0
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}
	targets, err := scanClaude(context.Background(), &cleaner.DefaultExecutor{}, tmpDir, rules)
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}

	// 4. Assert active version 2.1.300 is strictly PRESERVED (NOT in targets)
	var found299, found300 bool
	for _, target := range targets {
		if target.Path == v300 {
			found300 = true
		}
		if target.Path == v299 {
			found299 = true
			if target.Risk != domain.RiskLow {
				t.Errorf("expected RiskLow for inactive Claude version, got %v", target.Risk)
			}
			if target.Category != "ai-claude-versions" {
				t.Errorf("expected category 'ai-claude-versions', got '%s'", target.Category)
			}
			if !target.TTLEligible {
				t.Errorf("expected TTLEligible=true when MaxAgeDays=0")
			}
			if target.SizeBytes <= 0 {
				t.Errorf("expected SizeBytes > 0, got %d", target.SizeBytes)
			}
			if target.EligibleBytes != target.SizeBytes {
				t.Errorf("expected EligibleBytes == SizeBytes (%d), got %d", target.SizeBytes, target.EligibleBytes)
			}
			if target.ReviewOnly {
				t.Errorf("expected ReviewOnly=false for confirmed inactive version with valid active symlink")
			}
		}
	}

	if found300 {
		t.Fatalf("SECURITY VIOLATION: Active version %s was returned as deletion candidate!", v300)
	}
	if !found299 {
		t.Fatalf("Expected inactive version %s to be detected as target", v299)
	}
}

func TestClaude_ProtectedFiles_Untouched(t *testing.T) {
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	protectedFiles := []string{
		"config.json",
		"history.jsonl",
		"settings.json",
		"auth.json",
		"credentials.json",
	}

	for _, pf := range protectedFiles {
		if err := os.WriteFile(filepath.Join(claudeDir, pf), []byte(`{"protected": true}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// Create valid active version to avoid empty scan
	versionsDir := filepath.Join(tmpDir, ".local", "share", "claude", "versions")
	v100 := filepath.Join(versionsDir, "1.0.0")
	if err := os.MkdirAll(v100, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v100, "claude"), []byte("bin-1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(tmpDir, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(v100, filepath.Join(binDir, "claude")); err != nil {
		t.Fatal(err)
	}

	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}
	targets, err := scanClaude(context.Background(), &cleaner.DefaultExecutor{}, tmpDir, rules)
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}

	for _, target := range targets {
		base := filepath.Base(target.Path)
		for _, pf := range protectedFiles {
			if base == pf {
				t.Fatalf("SECURITY VIOLATION: Protected file %s was returned as target!", pf)
			}
		}
	}
}

func TestClaude_MissingSymlink_FailClosed(t *testing.T) {
	tmpDir := t.TempDir()
	versionsDir := filepath.Join(tmpDir, ".local", "share", "claude", "versions")
	v100 := filepath.Join(versionsDir, "1.0.0")
	v200 := filepath.Join(versionsDir, "2.0.0")

	if err := os.MkdirAll(v100, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v100, "claude"), []byte("bin-1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(v200, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v200, "claude"), []byte("bin-2.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Do NOT create any invocation symlink!
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}
	targets, err := scanClaude(context.Background(), &cleaner.DefaultExecutor{}, tmpDir, rules)
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}

	foundMap := make(map[string]domain.Target)
	for _, target := range targets {
		foundMap[target.Path] = target
	}

	for _, v := range []string{v100, v200} {
		tgt, ok := foundMap[v]
		if !ok {
			t.Fatalf("expected version %s to be returned as target", v)
		}
		if !tgt.ReviewOnly {
			t.Errorf("expected ReviewOnly=true for missing symlink on %s", v)
		}
		if tgt.ReviewReason != domain.ReviewReasonActiveVersion {
			t.Errorf("expected ReviewReason '%s', got '%s'", domain.ReviewReasonActiveVersion, tgt.ReviewReason)
		}
		if tgt.EligibleBytes != 0 {
			t.Errorf("expected EligibleBytes=0, got %d", tgt.EligibleBytes)
		}
		if tgt.TTLEligible {
			t.Errorf("expected TTLEligible=false")
		}
		if tgt.SizeBytes <= 0 {
			t.Errorf("expected SizeBytes > 0, got %d", tgt.SizeBytes)
		}
		if tgt.Category != "ai-claude-versions" {
			t.Errorf("expected category 'ai-claude-versions', got '%s'", tgt.Category)
		}
		if tgt.Risk != domain.RiskLow {
			t.Errorf("expected RiskLow, got %v", tgt.Risk)
		}
	}
}

func TestClaude_BrokenSymlink_FailClosed(t *testing.T) {
	tmpDir := t.TempDir()
	versionsDir := filepath.Join(tmpDir, ".local", "share", "claude", "versions")
	v100 := filepath.Join(versionsDir, "1.0.0")
	if err := os.MkdirAll(v100, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v100, "claude"), []byte("bin-1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create symlink ~/.local/bin/claude pointing to non-existent ~/.local/share/claude/versions/9.9.9
	binDir := filepath.Join(tmpDir, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	nonExistentTarget := filepath.Join(versionsDir, "9.9.9")
	if err := os.Symlink(nonExistentTarget, filepath.Join(binDir, "claude")); err != nil {
		t.Fatal(err)
	}

	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}
	targets, err := scanClaude(context.Background(), &cleaner.DefaultExecutor{}, tmpDir, rules)
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}

	if len(targets) == 0 {
		t.Fatalf("expected discovered versions to be reported even when symlink is broken")
	}

	for _, tgt := range targets {
		if !tgt.ReviewOnly {
			t.Errorf("expected ReviewOnly=true for broken symlink on %s", tgt.Path)
		}
		if tgt.ReviewReason != domain.ReviewReasonAmbiguous {
			t.Errorf("expected ReviewReason '%s', got '%s'", domain.ReviewReasonAmbiguous, tgt.ReviewReason)
		}
		if tgt.EligibleBytes != 0 {
			t.Errorf("expected EligibleBytes=0 for broken symlink, got %d", tgt.EligibleBytes)
		}
		if tgt.TTLEligible {
			t.Errorf("expected TTLEligible=false for broken symlink")
		}
		if tgt.SizeBytes <= 0 {
			t.Errorf("expected SizeBytes > 0, got %d", tgt.SizeBytes)
		}
	}
}

func TestClaude_AmbiguousSymlink_FailClosed(t *testing.T) {
	tmpDir := t.TempDir()
	versionsDir := filepath.Join(tmpDir, ".local", "share", "claude", "versions")
	v100 := filepath.Join(versionsDir, "1.0.0")
	v200 := filepath.Join(versionsDir, "2.0.0")

	if err := os.MkdirAll(v100, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v100, "claude"), []byte("bin-1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(v200, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v200, "claude"), []byte("bin-2.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create symlink 1: ~/.local/bin/claude -> v100
	binDir := filepath.Join(tmpDir, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(v100, filepath.Join(binDir, "claude")); err != nil {
		t.Fatal(err)
	}

	// Create symlink 2: ~/.local/share/claude/current -> v200
	claudeShareDir := filepath.Join(tmpDir, ".local", "share", "claude")
	if err := os.MkdirAll(claudeShareDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(v200, filepath.Join(claudeShareDir, "current")); err != nil {
		t.Fatal(err)
	}

	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}
	targets, err := scanClaude(context.Background(), &cleaner.DefaultExecutor{}, tmpDir, rules)
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}

	if len(targets) != 2 {
		t.Fatalf("expected 2 discovered versions reported, got %d", len(targets))
	}

	for _, tgt := range targets {
		if !tgt.ReviewOnly {
			t.Errorf("expected ReviewOnly=true for ambiguous symlinks on %s", tgt.Path)
		}
		if tgt.ReviewReason != domain.ReviewReasonAmbiguous {
			t.Errorf("expected ReviewReason '%s', got '%s'", domain.ReviewReasonAmbiguous, tgt.ReviewReason)
		}
		if tgt.EligibleBytes != 0 {
			t.Errorf("expected EligibleBytes=0 for ambiguous symlinks, got %d", tgt.EligibleBytes)
		}
		if tgt.TTLEligible {
			t.Errorf("expected TTLEligible=false for ambiguous symlinks")
		}
		if tgt.SizeBytes <= 0 {
			t.Errorf("expected SizeBytes > 0, got %d", tgt.SizeBytes)
		}
	}
}

func TestClaude_ProcessGuard_ActiveCLI(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Setup version in ~/.local/share/claude/versions/1.0.0
	versionsDir := filepath.Join(tmpDir, ".local", "share", "claude", "versions")
	v100 := filepath.Join(versionsDir, "1.0.0")
	if err := os.MkdirAll(v100, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v100, "claude"), []byte("bin-1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Active symlink pointing to 2.0.0
	v200 := filepath.Join(versionsDir, "2.0.0")
	if err := os.MkdirAll(v200, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v200, "claude"), []byte("bin-2.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(tmpDir, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(v200, filepath.Join(binDir, "claude")); err != nil {
		t.Fatal(err)
	}

	// Setup cache dir in ~/.claude/cache
	cacheDir := filepath.Join(tmpDir, ".claude", "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "temp.bin"), []byte("some-cache-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. Mock executor with claude running
	mockExec := newMockExecutor()
	mockExec.runningProcesses["claude"] = true

	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}
	targets, err := scanClaude(context.Background(), mockExec, tmpDir, rules)
	if err != nil {
		t.Fatalf("scanClaude failed: %v", err)
	}

	if len(targets) == 0 {
		t.Fatal("expected Claude targets to be emitted")
	}

	for _, target := range targets {
		if !target.ReviewOnly {
			t.Errorf("expected Claude target %s to have ReviewOnly=true when CLI is active", target.Path)
		}
		if target.ReviewReason != domain.ReviewReasonActiveProcess {
			t.Errorf("expected Claude target %s to have ReviewReason=%s, got %s", target.Path, domain.ReviewReasonActiveProcess, target.ReviewReason)
		}
		if target.EligibleBytes != 0 {
			t.Errorf("expected Claude target %s to have EligibleBytes=0, got %d", target.Path, target.EligibleBytes)
		}
		if target.TTLEligible {
			t.Errorf("expected Claude target %s to have TTLEligible=false", target.Path)
		}
		if target.SizeBytes <= 0 {
			t.Errorf("expected Claude target %s to have SizeBytes > 0, got %d", target.Path, target.SizeBytes)
		}
	}
}



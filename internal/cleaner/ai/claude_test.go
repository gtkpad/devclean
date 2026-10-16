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

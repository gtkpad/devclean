package installer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestCleaner_Name(t *testing.T) {
	c := NewCleaner(Options{})
	if c.Name() != "installer" {
		t.Fatalf("expected Name() to be %q, got %q", "installer", c.Name())
	}
}

func TestCleaner_IsInstalled(t *testing.T) {
	c := NewCleaner(Options{})
	if !c.IsInstalled() {
		t.Fatalf("expected IsInstalled() to be true, got false")
	}
}

func TestCleaner_Scan(t *testing.T) {
	tempDir := t.TempDir()

	// Create a dummy installer file
	installerPath := filepath.Join(tempDir, "sample.dmg")
	content := []byte("dummy installer bytes")
	if err := os.WriteFile(installerPath, content, 0o644); err != nil {
		t.Fatalf("failed to create dummy installer: %v", err)
	}

	past := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(installerPath, past, past); err != nil {
		t.Fatalf("failed to change times: %v", err)
	}

	opts := Options{
		Paths:    []string{tempDir},
		MaxDepth: 2,
		MinAge:   24 * time.Hour,
		MinSize:  0,
	}

	c := NewCleaner(opts)
	ctx := context.Background()

	targets, err := c.Scan(ctx, domain.EcosystemRules{})
	if err != nil {
		t.Fatalf("Scan returned unexpected error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}

	tgt := targets[0]
	if tgt.ID != "installer:"+installerPath {
		t.Errorf("expected ID %q, got %q", "installer:"+installerPath, tgt.ID)
	}
	if tgt.Path != installerPath {
		t.Errorf("expected Path %q, got %q", installerPath, tgt.Path)
	}
	if tgt.Category != "installer" {
		t.Errorf("expected Category %q, got %q", "installer", tgt.Category)
	}
	if tgt.Risk != domain.RiskLow {
		t.Errorf("expected Risk %q, got %q", domain.RiskLow, tgt.Risk)
	}
	if tgt.SizeBytes != int64(len(content)) {
		t.Errorf("expected SizeBytes %d, got %d", len(content), tgt.SizeBytes)
	}
}

func TestCleaner_Scan_ContextCanceled(t *testing.T) {
	tempDir := t.TempDir()
	opts := Options{
		Paths: []string{tempDir},
	}
	c := NewCleaner(opts)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Scan(ctx, domain.EcosystemRules{})
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestCleaner_Clean_DirectDeletionWithChmod(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Create a read-only file
	roFilePath := filepath.Join(tempDir, "locked.pkg")
	if err := os.WriteFile(roFilePath, []byte("pkg bytes"), 0o444); err != nil {
		t.Fatalf("failed to create read-only file: %v", err)
	}

	// 2. Create a directory containing a read-only file and make the directory read-only
	lockedDir := filepath.Join(tempDir, "LockedApp.app")
	if err := os.Mkdir(lockedDir, 0o755); err != nil {
		t.Fatalf("failed to create locked dir: %v", err)
	}
	innerFile := filepath.Join(lockedDir, "inner")
	if err := os.WriteFile(innerFile, []byte("inner"), 0o444); err != nil {
		t.Fatalf("failed to create inner file: %v", err)
	}
	if err := os.Chmod(lockedDir, 0o555); err != nil {
		t.Fatalf("failed to set read-only on locked dir: %v", err)
	}

	c := NewCleaner(Options{})
	ctx := context.Background()

	targets := []domain.Target{
		{
			ID:       "installer:" + roFilePath,
			Path:     roFilePath,
			Category: "installer",
		},
		{
			ID:       "installer:" + lockedDir,
			Path:     lockedDir,
			Category: "installer",
		},
	}

	if err := c.Clean(ctx, targets); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// Verify paths are removed
	if _, err := os.Stat(roFilePath); !os.IsNotExist(err) {
		t.Errorf("expected %s to be deleted, but still exists", roFilePath)
	}
	if _, err := os.Stat(lockedDir); !os.IsNotExist(err) {
		t.Errorf("expected %s to be deleted, but still exists", lockedDir)
	}
}

func TestCleaner_Clean_ContextCanceled(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "cancel_test.dmg")
	_ = os.WriteFile(filePath, []byte("test"), 0o644)

	c := NewCleaner(Options{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	targets := []domain.Target{
		{
			ID:       "installer:" + filePath,
			Path:     filePath,
			Category: "installer",
		},
	}

	err := c.Clean(ctx, targets)
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

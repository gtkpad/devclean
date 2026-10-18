//go:build acceptance

package acceptance

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/platform"
)

func TestHostDiagnostics(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Acceptance suite requires a live macOS host (runtime.GOOS == 'darwin')")
	}

	diag := ProbeHost(t)
	t.Logf("Host Diagnostics: Darwin %s (%s), APFS=%v (type=%s), FDA=%v (%s), ActiveProtectedApps=%v",
		diag.OSVersion, diag.Architecture, diag.APFSSupported, diag.FilesystemType,
		diag.FullDiskAccess, diag.FDANote, diag.ActiveProtectedApps)

	if diag.OSVersion == "" || diag.OSVersion == "unknown" {
		t.Errorf("expected valid non-empty OS version, got %q", diag.OSVersion)
	}
	if diag.Architecture == "" {
		t.Errorf("expected non-empty Architecture")
	}
	if !diag.APFSSupported {
		t.Errorf("expected APFSSupported=true on live modern macOS host, got false (type: %s)", diag.FilesystemType)
	}
}

// AssertSafetyInvariants enforces the 4 hard failure criteria (D-14):
// 1. ReviewOnly physical mutation / eligibility invariant: EligibleBytes must be 0.
// 2. Target duplication & zero double-counting: no duplicate canonical paths.
// 3. No unjustified omission: in-scope categories present on modern macOS must be reported.
// 4. Protected file integrity: sensitive/gated targets (ScriptCache, LaunchAgents) must be ReviewOnly.
func AssertSafetyInvariants(t *testing.T, scanResults []domain.Target) {
	t.Helper()

	if len(scanResults) == 0 {
		t.Fatalf("HARD FAILURE (VAL-03 / D-14 Criterion 3): DevClean scan returned 0 targets on live host")
	}

	seenPaths := make(map[string]string)
	categoryCounts := make(map[string]int)

	for _, trg := range scanResults {
		// Criterion 1: ReviewOnly targets must report 0 eligible bytes
		if trg.ReviewOnly && trg.EligibleBytes != 0 {
			t.Fatalf("HARD FAILURE (VAL-03 / D-14 Criterion 1): Target %s (%s) is ReviewOnly=true but reports EligibleBytes=%d (must be 0)",
				trg.ID, trg.Path, trg.EligibleBytes)
		}

		cat := MapTargetToBenchmarkCategory(trg)
		categoryCounts[cat]++

		// Skip virtual scheme paths from filesystem duplicate check
		if strings.Contains(trg.Path, "://") || strings.TrimSpace(trg.Path) == "" {
			continue
		}

		cleanPath := filepath.Clean(trg.Path)

		// Criterion 2: Zero duplicate paths across the entire scan
		if existingID, exists := seenPaths[cleanPath]; exists {
			t.Fatalf("HARD FAILURE (VAL-03 / D-14 Criterion 2): Target duplication detected for path %s (targets %s and %s)",
				cleanPath, existingID, trg.ID)
		}
		seenPaths[cleanPath] = trg.ID

		// Criterion 4: Gated & protected files must be ReviewOnly
		if strings.Contains(cleanPath, "Service Worker/ScriptCache") && !trg.ReviewOnly {
			t.Fatalf("HARD FAILURE (VAL-03 / D-14 Criterion 4): ScriptCache target %s must be ReviewOnly per BROW-04", trg.ID)
		}
		if (strings.Contains(cleanPath, "/LaunchAgents") || strings.Contains(cleanPath, "group.dev.orbstack/data")) && !trg.ReviewOnly {
			t.Fatalf("HARD FAILURE (VAL-03 / D-14 Criterion 4): Protected system target %s must be ReviewOnly", trg.ID)
		}
	}

	// Criterion 3: Verify fundamental in-scope categories on modern macOS
	essentialCategories := []string{"User essentials", "App caches", "Browsers", "Developer tools"}
	for _, cat := range essentialCategories {
		if categoryCounts[cat] == 0 {
			t.Fatalf("HARD FAILURE (VAL-03 / D-14 Criterion 3): Unjustified omission of in-scope category %q from DevClean scan", cat)
		}
	}
}

// TestMacOSAcceptance_ParityAndReceipt executes the full read-only acceptance inspection
// on a live macOS machine, validates all safety invariants, and outputs dual receipts (VAL-03).
func TestMacOSAcceptance_ParityAndReceipt(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Acceptance suite requires a live macOS host (runtime.GOOS == 'darwin')")
	}

	ctx := context.Background()

	// 1. Probe pure Go host diagnostics (D-03)
	diag := ProbeHost(t)
	t.Logf("Host Diagnostics: Darwin %s (%s), APFS=%v, FDA=%v (%s)",
		diag.OSVersion, diag.Architecture, diag.APFSSupported, diag.FullDiskAccess, diag.FDANote)

	// 2. Run or fallback baseline benchmark
	baselineResult := RunBaselineOrFallback(t)
	t.Logf("Acceptance Baseline: %s (%d categories, %d items, %s)",
		baselineResult.Source, len(baselineResult.Categories), baselineResult.TotalItems, baselineResult.TotalBytesFormatted)

	// 3. Run DevClean real host scan in strictly read-only mode (T-14-05, D-14)
	cfg := config.Defaults()
	resolver := platform.New()
	eng := engine.New(&cfg, resolver)
	engine.RegisterDefaultCleaners(eng, &cfg, resolver)

	scanResults := engine.Collect(ctx, eng.Scan(ctx))
	var allTargets []domain.Target
	for _, sr := range scanResults {
		allTargets = append(allTargets, sr.Targets...)
	}
	t.Logf("DevClean scan returned %d targets across %d ecosystems on host", len(allTargets), len(scanResults))

	// 4. Invariant assertion checks (D-14 Hard Failure Conditions)
	AssertSafetyInvariants(t, allTargets)

	// 5. Generate dual receipts (D-01, D-13, D-15, D-16)
	repoRoot := findRepoRoot()
	outDir := filepath.Join(repoRoot, "reports", "acceptance")
	GenerateReceipts(t, outDir, diag, baselineResult, allTargets)

	// Verify both receipt files were generated
	mdPath := filepath.Join(outDir, "MACOS-ACCEPTANCE.md")
	jsonPath := filepath.Join(outDir, "MACOS-ACCEPTANCE.json")
	if _, err := os.Stat(mdPath); err != nil {
		t.Fatalf("expected generated receipt %s, got err: %v", mdPath, err)
	}
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("expected generated receipt %s, got err: %v", jsonPath, err)
	}
}

func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "."
}

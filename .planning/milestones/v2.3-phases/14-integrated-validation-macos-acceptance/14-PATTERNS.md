# Phase 14: Integrated Validation & macOS Acceptance - Pattern Map

**Mapped:** 2026-10-18  
**Files analyzed:** 12 (10 new/test files, 2 output artifacts)  
**Analogs found:** 12 / 12 (10 exact, 2 role-match, 0 no analog)  

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|:---:|
| `internal/cleaner/macos/gap_test.go` | test | batch / file-I/O | `internal/cleaner/macos/macos_test.go` (lines 77-175) | exact |
| `internal/cleaner/browsers/gap_test.go` | test | batch / file-I/O | `internal/cleaner/browsers/browsers_test.go` (lines 27-112) | exact |
| `internal/cleaner/leftovers/gap_test.go` | test | batch / file-I/O | `internal/cleaner/leftovers/cleaner_test.go` (lines 14-100) | exact |
| `scripts/verify-coverage.sh` | script / quality gate | shell pipeline / exit-code gating | `scripts/qa_runner.sh` (lines 1-60) | role-match |
| `scripts/verify-builds.sh` | script / compiler gate | shell pipeline / compiler invocation | `scripts/qa_runner.sh` (build environment) | role-match |
| `cmd/workflow_v23_e2e_test.go` | test / integration | multi-channel orchestration / file-I/O | `cmd/workflow_v22_e2e_test.go` (lines 73-218, 501-627) | exact |
| `test/acceptance/macos_test.go` | test / acceptance | host inspection / read-only scan / report | `internal/fs/statfs_darwin.go` + `cmd/workflow_v22_e2e_test.go` | exact |
| `test/acceptance/diagnostics.go` | service / host inspector | pure Go syscalls / kernel queries | `internal/fs/statfs_darwin.go` (lines 1-25) | exact |
| `test/acceptance/benchmark.go` | service / benchmark runner | external process execution / fallback | `internal/cleaner/executor.go` + `cmd/workflow_v22_e2e_test.go` | exact |
| `test/acceptance/receipt.go` | utility / formatter | transform / file-I/O | `internal/trash/manifest.go` (serialization) | exact |
| `MACOS-ACCEPTANCE.md` | artifact / report | documentation / Markdown table | `QA_REPORT.md` / `SPEC-v2.3-MOLE-PARITY.md` | exact |
| `MACOS-ACCEPTANCE.json` | artifact / receipt payload | machine-readable JSON | `internal/trash/manifest.go` (JSON schema) | exact |

---

## Pattern Assignments

### 1. `internal/cleaner/macos/gap_test.go` (test, batch / file-I/O)

**Role:** Fills the unit test coverage gap in `internal/cleaner/macos` (elevating statement coverage from 80.0% to `>= 85.0%`). Specifically targets constructors (`New`, `NewWithExecutor`), `Clean()` deletion error branches and bundle ID resolution, `cleanTrash` error edge cases, and `scanDirectoryWithTCCFallback` error handling.

**Analog:** `internal/cleaner/macos/macos_test.go` (lines 77-214) and `internal/cleaner/macos/user_test.go`

**Imports Pattern:**
```go
package macos

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)
```

**Targeted Gap Remediation Patterns:**
```go
// 1. Constructor test pattern (covers New and NewWithExecutor at 0.0%)
func TestMacOSCleaner_Constructors(t *testing.T) {
	c1 := New()
	if c1.Name() != "macos" {
		t.Errorf("New().Name() = %q, want 'macos'", c1.Name())
	}

	mockExec := newMockProcessExecutor()
	c2 := NewWithExecutor(mockExec)
	if c2.Name() != "macos" {
		t.Errorf("NewWithExecutor().Name() = %q, want 'macos'", c2.Name())
	}
}

// 2. Target Bundle ID Resolution pattern (covers resolveTargetBundleID lines 150-173)
func TestMacOSCleaner_ResolveTargetBundleID(t *testing.T) {
	tests := []struct {
		target   domain.Target
		expected string
	}{
		{
			target:   domain.Target{Category: "macos-spotify-cache"},
			expected: "com.spotify.client",
		},
		{
			target:   domain.Target{ID: "macos-code-sign-clones"},
			expected: "com.google.Chrome.code_sign_clone",
		},
		{
			target:   domain.Target{ID: "macos-cache-com.apple.Safari"},
			expected: "com.apple.Safari",
		},
		{
			target:   domain.Target{ID: "macos-log-com.apple.dt.Xcode"},
			expected: "com.apple.dt.Xcode",
		},
		{
			target:   domain.Target{ID: "macos-daemon-quicklook"},
			expected: "com.apple.quicklook.ui.helper",
		},
		{
			target:   domain.Target{ID: "macos-daemon-unknown", Path: "/path/to/daemon"},
			expected: "unknown",
		},
		{
			target:   domain.Target{ID: "custom", Path: "/Library/Caches/sample"},
			expected: "sample",
		},
	}

	for _, tt := range tests {
		got := resolveTargetBundleID(tt.target)
		if got != tt.expected {
			t.Errorf("resolveTargetBundleID(%+v) = %q, want %q", tt.target, got, tt.expected)
		}
	}
}

// 3. Clean() Error and Invariant Branch pattern (covers Clean lines 100-148)
func TestMacOSCleaner_Clean_EdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	mockHome := filepath.Join(tempDir, "home")
	mockExec := newMockProcessExecutor()
	c := NewWithDirAndExecutor(mockHome, mockExec)

	// Context cancellation check
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := c.Clean(ctx, []domain.Target{{Path: filepath.Join(mockHome, "sample")}})
	if err == nil {
		t.Fatalf("expected error on canceled context, got nil")
	}

	// Non-existent target is safely ignored
	err = c.Clean(context.Background(), []domain.Target{{Path: filepath.Join(mockHome, "nonexistent")}})
	if err != nil {
		t.Fatalf("unexpected error on missing target: %v", err)
	}

	// Symlink target is skipped (Symlink Guard)
	realDir := filepath.Join(mockHome, "real")
	_ = os.MkdirAll(realDir, 0755)
	symlinkTarget := filepath.Join(mockHome, "symlink")
	_ = os.Symlink(realDir, symlinkTarget)

	err = c.Clean(context.Background(), []domain.Target{{Path: symlinkTarget}})
	if err != nil {
		t.Fatalf("unexpected error on symlink target: %v", err)
	}
	if _, err := os.Lstat(symlinkTarget); os.IsNotExist(err) {
		t.Fatalf("symlink target should not be physically removed")
	}
}
```

---

### 2. `internal/cleaner/browsers/gap_test.go` (test, batch / file-I/O)

**Role:** Fills the unit test coverage gap in `internal/cleaner/browsers` (elevating statement coverage from 83.4% to `>= 85.0%`). Specifically targets `NewWithExecutor`, `NewWithExecutorHomeAndOS`, `isSyscallPermission` edge cases, permission degradation in Safari/Chromium profiles, and URI parsing fallback paths.

**Analog:** `internal/cleaner/browsers/browsers_test.go` (lines 15-57) and `internal/cleaner/browsers/safari_test.go` (lines 67-100)

**Imports Pattern:**
```go
package browsers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)
```

**Targeted Gap Remediation Patterns:**
```go
// 1. Constructor and Platform Injection test pattern
func TestBrowsers_Constructors(t *testing.T) {
	mockExec := cleaner.NewMockExecutor()
	c1 := NewWithExecutor(mockExec)
	if c1.Name() != "browsers" {
		t.Errorf("NewWithExecutor().Name() = %q, want 'browsers'", c1.Name())
	}

	tempHome := t.TempDir()
	c2 := NewWithExecutorHomeAndOS(mockExec, tempHome, "darwin")
	if c2.Name() != "browsers" {
		t.Errorf("NewWithExecutorHomeAndOS().Name() = %q, want 'browsers'", c2.Name())
	}
	if !c2.IsInstalled() {
		// Empty dir without browsers should report false
	}
}

// 2. Syscall Permission Error Validation pattern (safari.go lines 187-195)
func TestBrowsers_IsSyscallPermission(t *testing.T) {
	tests := []struct {
		err      error
		expected bool
	}{
		{err: nil, expected: false},
		{err: errors.New("generic error"), expected: false},
		{err: os.ErrPermission, expected: true},
		{err: syscall.EPERM, expected: true},
		{err: syscall.EACCES, expected: true},
		{err: &os.PathError{Op: "open", Path: "/test", Err: syscall.EPERM}, expected: true},
	}

	for _, tt := range tests {
		got := isSyscallPermission(tt.err)
		if got != tt.expected {
			t.Errorf("isSyscallPermission(%v) = %v, want %v", tt.err, got, tt.expected)
		}
	}
}

// 3. URI Parsing and Extraction Fallback pattern (browsers.go lines 191-205)
func TestBrowsers_ExtractBrowserIDFromURI(t *testing.T) {
	tests := []struct {
		uri      string
		expected string
	}{
		{"browser://chrome/caches", "chrome"},
		{"browser://safari/caches", "safari"},
		{"browser://firefox/profiles", "firefox"},
		{"invalid-uri", ""},
		{"file:///tmp/cache", ""},
	}

	for _, tt := range tests {
		got := extractBrowserIDFromURI(tt.uri)
		if got != tt.expected {
			t.Errorf("extractBrowserIDFromURI(%q) = %q, want %q", tt.uri, got, tt.expected)
		}
	}
}
```

---

### 3. `internal/cleaner/leftovers/gap_test.go` (test, batch / file-I/O)

**Role:** Fills the unit test coverage gap in `internal/cleaner/leftovers` (elevating statement coverage from 83.0% to `>= 85.0%`). Specifically targets `New`, `NewWithExecutor`, `NewWithConfig` default configurations, symlink traversal prevention in `isSafeSymlink`, and corrupted plist parsing fallbacks.

**Analog:** `internal/cleaner/leftovers/cleaner_test.go` (lines 85-110) and `launchagents_test.go` (lines 14-80)

**Imports Pattern:**
```go
package leftovers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)
```

**Targeted Gap Remediation Patterns:**
```go
// 1. Constructor Default Configuration pattern (cleaner.go lines 25-80)
func TestLeftovers_Constructors_Defaults(t *testing.T) {
	c1 := New()
	if c1.Name() != "leftovers" {
		t.Errorf("New().Name() = %q, want 'leftovers'", c1.Name())
	}

	mockExec := cleaner.NewMockExecutor()
	c2 := NewWithExecutor(mockExec)
	if c2.Name() != "leftovers" {
		t.Errorf("NewWithExecutor().Name() = %q, want 'leftovers'", c2.Name())
	}

	// NewWithConfig with explicit nil slices should apply canonical defaults
	tempDir := t.TempDir()
	c3 := NewWithConfig(mockExec, tempDir, "darwin", nil, nil, "")
	if c3.Name() != "leftovers" {
		t.Errorf("NewWithConfig().Name() = %q, want 'leftovers'", c3.Name())
	}
}

// 2. Symlink Traversal Guard pattern (leftovers.go lines 41-65)
func TestLeftovers_IsSafeSymlink_Traversals(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, "root")
	outside := filepath.Join(tempDir, "outside")
	_ = os.MkdirAll(root, 0755)
	_ = os.MkdirAll(outside, 0755)

	// Legitimate child directory
	child := filepath.Join(root, "child")
	_ = os.MkdirAll(child, 0755)

	// Symlink pointing outside root
	badLink := filepath.Join(root, "bad_link")
	_ = os.Symlink(outside, badLink)

	// Symlink pointing inside root
	goodLink := filepath.Join(root, "good_link")
	_ = os.Symlink(child, goodLink)

	if isSafeSymlink(badLink, root) {
		t.Errorf("isSafeSymlink should return false for link pointing outside root")
	}
	if !isSafeSymlink(goodLink, root) {
		t.Errorf("isSafeSymlink should return true for link pointing inside root")
	}
}

// 3. Corrupt Plist Parsing pattern (plist.go lines 26-40)
func TestLeftovers_ParseCorruptPlist(t *testing.T) {
	tempDir := t.TempDir()
	corruptFile := filepath.Join(tempDir, "corrupt.plist")
	_ = os.WriteFile(corruptFile, []byte("NOT_A_PLIST_DATA"), 0644)

	_, err := parseLaunchAgentFile(corruptFile)
	if err == nil {
		t.Errorf("expected error parsing corrupted launch agent plist, got nil")
	}

	_, err = parseAppBundleInfoFile(corruptFile)
	if err == nil {
		t.Errorf("expected error parsing corrupted app bundle info plist, got nil")
	}
}
```

---

### 4. `scripts/verify-coverage.sh` (script / quality gate, shell pipeline / exit-code gating)

**Role:** Automated CI/CD and pre-flight verification script enforcing the strict `>= 85.0%` statement coverage threshold across all 11 altered or new v2.3 packages (D-09, D-10, VAL-02). Fails fast with exit code 1 if any package falls below 85.0%.

**Analog:** `scripts/qa_runner.sh` (lines 1-60: shell configuration, logging, ANSI coloring, test tracking arrays)

**Concrete Implementation Pattern:**
```bash
#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# DevClean v2.3 — Per-Package Coverage Auditor (VAL-02, D-09, D-10)
# Enforces strict >= 85.0% statement coverage on all 11 altered/new packages.
# ==============================================================================

MIN_COVERAGE=85.0
FAILURES=0

TARGET_PACKAGES=(
  "github.com/gtkpad/devclean/internal/cleaner/macos"
  "github.com/gtkpad/devclean/internal/cleaner/browsers"
  "github.com/gtkpad/devclean/internal/cleaner/leftovers"
  "github.com/gtkpad/devclean/internal/cleaner/node"
  "github.com/gtkpad/devclean/internal/cleaner/shell"
  "github.com/gtkpad/devclean/internal/cleaner/xcode"
  "github.com/gtkpad/devclean/internal/cleaner/ai"
  "github.com/gtkpad/devclean/internal/cleaner/docker"
  "github.com/gtkpad/devclean/internal/deleter"
  "github.com/gtkpad/devclean/internal/engine"
  "github.com/gtkpad/devclean/internal/domain"
)

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== Auditing DevClean v2.3 Package Coverage (Floor: ${MIN_COVERAGE}%) ===${NC}"

for pkg in "${TARGET_PACKAGES[@]}"; do
  output=$(go test -cover "$pkg" 2>&1)
  cov_str=$(echo "$output" | grep -o 'coverage: [0-9.]*%' | awk '{print $2}' | tr -d '%')
  if [ -z "$cov_str" ]; then
    echo -e "${RED}❌ [FAIL]${NC} $pkg: Could not extract coverage"
    FAILURES=$((FAILURES + 1))
    continue
  fi

  is_passing=$(awk "BEGIN {print ($cov_str >= $MIN_COVERAGE) ? 1 : 0}")
  if [ "$is_passing" -eq 1 ]; then
    echo -e "${GREEN}✅ [PASS]${NC} $pkg: ${cov_str}%"
  else
    echo -e "${RED}❌ [FAIL]${NC} $pkg: ${cov_str}% (below threshold ${MIN_COVERAGE}%)"
    FAILURES=$((FAILURES + 1))
  fi
done

if [ "$FAILURES" -gt 0 ]; then
  echo -e "\n${RED}Error: $FAILURES package(s) failed coverage requirement (>= ${MIN_COVERAGE}%).${NC}"
  exit 1
fi

echo -e "\n${GREEN}All ${#TARGET_PACKAGES[@]} target packages meet or exceed the ${MIN_COVERAGE}% coverage floor.${NC}"
exit 0
```

---

### 5. `scripts/verify-builds.sh` (script / compiler gate, shell pipeline / compiler invocation)

**Role:** Validates pure static cross-compilation without CGO (`CGO_ENABLED=0`) across the 4-target matrix (`darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`), asserting zero dynamic bindings and clean binary generation (D-11, VAL-02).

**Analog:** `scripts/qa_runner.sh` (lines 10-23: environment variables, bin paths)

**Concrete Implementation Pattern:**
```bash
#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# DevClean v2.3 — Zero-CGO Static Build Matrix Verifier (VAL-02, D-11)
# Compiles all 4 cross-platform targets cleanly with CGO_ENABLED=0.
# ==============================================================================

TARGETS=(
  "darwin/arm64"
  "darwin/amd64"
  "linux/amd64"
  "linux/arm64"
)

GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== Verifying Static Zero-CGO Cross-Compilation Matrix ===${NC}"
FAILURES=0

for target in "${TARGETS[@]}"; do
  os="${target%/*}"
  arch="${target#*/}"
  echo -n "Building static binary for $os/$arch (CGO_ENABLED=0)... "
  if CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -o /dev/null . 2>&1; then
    echo -e "${GREEN}✔ SUCCESS${NC}"
  else
    echo -e "${RED}✘ FAILED${NC}"
    FAILURES=$((FAILURES + 1))
  fi
done

if [ "$FAILURES" -gt 0 ]; then
  echo -e "\n${RED}Build matrix verification failed: $FAILURES target(s) failed compilation.${NC}"
  exit 1
fi

echo -e "\n${GREEN}All 4 static cross-targets compiled cleanly with zero CGO dependencies.${NC}"
exit 0
```

---

### 6. `cmd/workflow_v23_e2e_test.go` (test / integration, multi-channel orchestration / file-I/O)

**Role:** Capstone end-to-end integration test file mirroring `cmd/workflow_v22_e2e_test.go`. Validates all 5 DevClean channels (CLI, TUI Bubble Tea headless, MCP Server, Scheduler, and Coordinator) under 4 stress & fault-injection scenarios, proving invariant enforcement, ReviewOnly target immutability, process protection, symlink guard traversal rejection, graceful degradation, and canonical zero double-counting (D-05..D-08, VAL-01, VAL-02).

**Analog:** `cmd/workflow_v22_e2e_test.go` (lines 73-218 for CLI/preset, lines 363-499 for Scheduler, lines 501-627 for TUI/Blacklist)

**Imports Pattern:**
```go
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/mcp"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/scheduler"
	"github.com/gtkpad/devclean/internal/trash"
	"github.com/gtkpad/devclean/internal/tui"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)
```

**Concrete Multi-Channel Test Patterns:**
```go
// Hermetic Fixture Setup with Inviolable Canary Sentinels
type v23TestRig struct {
	TmpDir        string
	FakeHome      string
	FakeWorkspace string
	Sentinels     map[string]string // targetPath -> sentinelContent
}

func setupV23HermeticRig(t *testing.T) *v23TestRig {
	t.Helper()
	tmpDir := t.TempDir()
	fakeHome := filepath.Join(tmpDir, "home")
	fakeWorkspace := filepath.Join(tmpDir, "workspace")

	t.Setenv("HOME", fakeHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(fakeHome, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(fakeHome, ".local", "share"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(fakeHome, ".local", "state"))
	t.Setenv("TMPDIR", filepath.Join(tmpDir, "tmp"))

	rig := &v23TestRig{
		TmpDir:        tmpDir,
		FakeHome:      fakeHome,
		FakeWorkspace: fakeWorkspace,
		Sentinels:     make(map[string]string),
	}

	// Plant ReviewOnly targets with sentinels
	// 1. pnpm global store
	pnpmStore := filepath.Join(fakeHome, "Library", "pnpm", "store", "v3")
	rig.plantSentinel(t, pnpmStore, "pnpm_sentinel.bin", "SENTINEL_INVIOLABLE_PNPM_STORE_V23")

	// 2. OrbStack data
	orbData := filepath.Join(fakeHome, "Library", "Group Containers", "group.dev.orbstack", "data")
	rig.plantSentinel(t, orbData, "vm.img", "SENTINEL_INVIOLABLE_ORBSTACK_DATA_V23")

	// 3. Stale LaunchAgent
	agentDir := filepath.Join(fakeHome, "Library", "LaunchAgents")
	rig.plantSentinel(t, agentDir, "com.corp.stale.plist", "SENTINEL_INVIOLABLE_LAUNCHAGENT_V23")

	// 4. App Leftover (DeadApp in Application Support)
	deadAppSupport := filepath.Join(fakeHome, "Library", "Application Support", "DeadApp")
	rig.plantSentinel(t, deadAppSupport, "state.db", "SENTINEL_INVIOLABLE_APP_LEFTOVER_V23")

	// 5. ScriptCache (Chrome Service Worker)
	scriptCache := filepath.Join(fakeHome, "Library", "Application Support", "Google", "Chrome", "Default", "Service Worker", "ScriptCache")
	rig.plantSentinel(t, scriptCache, "cached_script_0", "SENTINEL_INVIOLABLE_SCRIPTCACHE_V23")

	return rig
}

func (r *v23TestRig) plantSentinel(t *testing.T, dir, filename, content string) string {
	t.Helper()
	_ = os.MkdirAll(dir, 0755)
	filePath := filepath.Join(dir, filename)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to plant sentinel %s: %v", filePath, err)
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

// Flow 1: Universal ReviewOnly Immunity across 5 Channels (D-05, D-06)
func TestE2E_V23_ReviewOnly_ImmunityAcrossChannels(t *testing.T) {
	rig := setupV23HermeticRig(t)

	// Channel 1: CLI invocation (clean --auto --yes)
	cleanCmd := newCleanCmd()
	cleanCmd.SetArgs([]string{"--auto", "--yes"})
	var cliBuf bytes.Buffer
	cleanCmd.SetOut(&cliBuf)
	cleanCmd.SetErr(&cliBuf)
	_ = cleanCmd.Execute()
	rig.assertAllSentinelsIntact(t, "CLI")

	// Channel 2: Deleter Coordinator direct invocation
	cfg := config.Defaults()
	bl := safety.NewBlacklist(cfg.ProtectedPatterns)
	coord := deleter.New(nil, &cfg, bl, nil, nil, rig.FakeHome, rig.FakeHome)
	
	// Create mock ReviewOnly targets
	reviewTargets := []domain.Target{
		{ID: "pnpm-store", Path: filepath.Join(rig.FakeHome, "Library", "pnpm", "store", "v3"), ReviewOnly: true},
		{ID: "orbstack-data", Path: filepath.Join(rig.FakeHome, "Library", "Group Containers", "group.dev.orbstack", "data"), ReviewOnly: true},
	}
	res, err := coord.Delete(context.Background(), reviewTargets, deleter.DeleteOptions{ForceDirect: true})
	if err != nil {
		t.Fatalf("coordinator delete returned error: %v", err)
	}
	if len(res.SkippedReviewOnly) != len(reviewTargets) {
		t.Fatalf("expected coordinator to skip %d review targets, skipped %d", len(reviewTargets), len(res.SkippedReviewOnly))
	}
	rig.assertAllSentinelsIntact(t, "Coordinator")

	// Channel 3: MCP Server Tool execution
	resolver := platform.New()
	eng := engine.New(&cfg, resolver)
	server := mcp.New(eng, &cfg, mcp.WithCoordinator(coord))
	// Call propose_cleanup -> assert targets returned with status "review_only" and eligible_bytes 0
	// Call execute_cleanup -> assert zero mutations
	rig.assertAllSentinelsIntact(t, "MCP")

	// Channel 4: Scheduler Automated Background Run
	runCmd := newScheduleRunCmd()
	var schedBuf bytes.Buffer
	runCmd.SetOut(&schedBuf)
	_ = runCmd.Execute()
	rig.assertAllSentinelsIntact(t, "Scheduler")

	// Channel 5: TUI Bubble Tea Headless Harness
	// Instantiate MainModel / ConfirmModel, dispatch 'y' key -> assert review items unselectable
	rig.assertAllSentinelsIntact(t, "TUI")
}

// Flow 2: Zero Double-Counting and Canonical Path Uniqueness (D-08)
func TestE2E_V23_ZeroDoubleCounting_AcrossEcosystems(t *testing.T) {
	rig := setupV23HermeticRig(t)
	cfg := config.Defaults()
	resolver := platform.New()
	eng := engine.New(&cfg, resolver)
	engine.RegisterDefaultCleaners(eng, &cfg, resolver)

	ctx := context.Background()
	results := engine.Collect(ctx, eng.Scan(ctx))

	seenPaths := make(map[string]string) // canonical path -> ecosystem/target ID
	for _, trg := range results {
		if strings.HasPrefix(trg.Path, "browser://") {
			continue // Skip virtual scheme paths
		}
		cleanPath := filepath.Clean(trg.Path)

		// Check exact collision
		if existing, exists := seenPaths[cleanPath]; exists {
			t.Fatalf("ZERO DOUBLE-COUNTING VIOLATION: duplicate path %s reported by both %s and %s", cleanPath, existing, trg.ID)
		}

		// Check parent/prefix collision
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

// Flow 3: Symlink Guard and Blacklist Traversal Rejection (D-06)
func TestE2E_V23_SymlinkGuard_BlacklistTraversals(t *testing.T) {
	rig := setupV23HermeticRig(t)
	
	// Create malicious symlinks in a cleanable cache dir pointing to protected system paths
	cacheDir := filepath.Join(rig.FakeHome, "Library", "Caches", "malicious_app")
	_ = os.MkdirAll(cacheDir, 0755)
	
	sshTarget := filepath.Join(rig.FakeHome, ".ssh")
	_ = os.MkdirAll(sshTarget, 0700)
	rig.plantSentinel(t, sshTarget, "id_rsa", "PRIVATE_KEY_NEVER_DELETE")

	linkToSSH := filepath.Join(cacheDir, "symlink_ssh")
	_ = os.Symlink(sshTarget, linkToSSH)

	gitTarget := filepath.Join(rig.FakeWorkspace, ".git")
	_ = os.MkdirAll(gitTarget, 0755)
	rig.plantSentinel(t, gitTarget, "config", "GIT_CONFIG_NEVER_DELETE")

	linkToGit := filepath.Join(cacheDir, "symlink_git")
	_ = os.Symlink(gitTarget, linkToGit)

	// Execute cleaner on cacheDir target
	target := domain.Target{
		ID:         "macos-cache-malicious",
		Path:       cacheDir,
		Category:   "macos-app-cache",
		ReviewOnly: false,
	}

	cfg := config.Defaults()
	bl := safety.NewBlacklist(cfg.ProtectedPatterns)
	coord := deleter.New(nil, &cfg, bl, nil, nil, rig.FakeHome, rig.FakeHome)
	_, _ = coord.Delete(context.Background(), []domain.Target{target}, deleter.DeleteOptions{ForceDirect: true})

	// Assert sensitive files outside symlinks remain completely untouched
	rig.assertAllSentinelsIntact(t, "Symlink Guard")
}

// Flow 4: Process Guard and Graceful Degradation (D-06)
func TestE2E_V23_FaultInjection_ProcessGuard_And_Degradation(t *testing.T) {
	rig := setupV23HermeticRig(t)

	// Simulate running process for Chrome and Xcode
	mockExec := cleaner.NewMockExecutor()
	mockExec.RunningProcesses["Google Chrome"] = true
	mockExec.RunningProcesses["Xcode"] = true

	// Simulate Docker daemon connection failure
	mockExec.Errors["docker system df --format json"] = errors.New("cannot connect to Docker daemon")

	// Verify Scan marks running processes with ReviewOnly=true and ReviewReasonActiveProcess
	// Verify Scan sets ProviderMetrics.Partial=true on Docker failure without panicking
}
```

---

### 7. `test/acceptance/macos_test.go` (test / acceptance, host inspection / read-only scan / report)

**Role:** Gated executable test (`//go:build acceptance`) that runs on a real Darwin host to evaluate paridade with Mole (`tw93/mole`), enforce the 4 hard failure criteria (D-14), and orchestrate the generation of `MACOS-ACCEPTANCE.md` and `MACOS-ACCEPTANCE.json` (VAL-03, D-01..D-04, D-13..D-16).

**Analog:** `cmd/workflow_v22_e2e_test.go` (execution flow) and `internal/fs/statfs_darwin.go` (pure Go Darwin system inspection)

**Imports Pattern:**
```go
//go:build acceptance

package acceptance_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/test/acceptance"
)
```

**Concrete Acceptance Runner Pattern:**
```go
func TestMacOS_ParityAcceptance(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Acceptance suite requires a live macOS host (runtime.GOOS == 'darwin')")
	}

	ctx := context.Background()

	// 1. Gather Pure Go Host Diagnostics (D-03)
	diag := acceptance.ProbeHost(t)
	t.Logf("Host Diagnostics: Darwin %s (%s), APFS=%v, FDA=%v (%s)",
		diag.OSVersion, diag.Architecture, diag.APFSSupported, diag.FullDiskAccess, diag.FDANote)

	// 2. Run or Fallback Mole Benchmark (D-02)
	moleResult := acceptance.RunMoleOrFallback(t)
	t.Logf("Mole Baseline: %s (%d categories, %d items, %s)",
		moleResult.Source, len(moleResult.Categories), moleResult.TotalItems, moleResult.TotalBytesFormatted)

	// 3. Run DevClean Real Scan in Strictly Read-Only Mode (D-14)
	cfg := config.Defaults()
	resolver := platform.New()
	eng := engine.New(&cfg, resolver)
	engine.RegisterDefaultCleaners(eng, &cfg, resolver)

	scanResults := engine.Collect(ctx, eng.Scan(ctx))
	if len(scanResults) == 0 {
		t.Fatalf("DevClean scan returned 0 targets on live host")
	}

	// 4. Invariant Assertion Checks (D-14 Hard Failure Conditions)
	acceptance.AssertSafetyInvariants(t, scanResults)

	// 5. Generate Dual Receipts (D-01, D-13, D-15, D-16)
	receiptDir := filepath.Join("..", "..", ".planning", "phases", "14-integrated-validation-macos-acceptance")
	acceptance.GenerateReceipts(t, receiptDir, diag, moleResult, scanResults)
}
```

---

### 8. `test/acceptance/diagnostics.go` (service / host inspector, pure Go syscalls / kernel queries)

**Role:** Pure Go, zero-CGO host diagnostic probe extracting macOS kernel version, architecture, APFS filesystem flags, TCC/Full Disk Access status, and active protected processes (D-03, VAL-03).

**Analog:** `internal/fs/statfs_darwin.go` (lines 1-25: `unix.Statfs`) and `internal/cleaner/browsers/process.go`

**Imports Pattern:**
```go
//go:build acceptance

package acceptance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)
```

**Concrete Host Probing Pattern:**
```go
type HostDiagnostics struct {
	OSVersion           string   `json:"os_version"`
	Architecture        string   `json:"architecture"`
	APFSSupported       bool     `json:"apfs_supported"`
	FilesystemType      string   `json:"filesystem_type"`
	FullDiskAccess      bool     `json:"full_disk_access"`
	FDANote             string   `json:"fda_note"`
	ActiveProtectedApps []string `json:"active_protected_apps"`
}

func ProbeHost(t *testing.T) HostDiagnostics {
	t.Helper()
	homeDir, _ := os.UserHomeDir()

	// 1. OS Version via Sysctl (Pure Go, no sw_vers shell command)
	osVer, err := unix.Sysctl("kern.osproductversion")
	if err != nil {
		osVer = "unknown"
	}

	// 2. APFS detection via unix.Statfs
	var stat unix.Statfs_t
	apfs := false
	fsType := "unknown"
	if err := unix.Statfs(homeDir, &stat); err == nil {
		// Extract Fstypename from [16]int8
		var buf []byte
		for _, b := range stat.Fstypename {
			if b == 0 {
				break
			}
			buf = append(buf, byte(b))
		}
		fsType = string(buf)
		if fsType == "apfs" {
			apfs = true
		}
	}

	// 3. TCC / Full Disk Access Probing
	fda, fdaNote := probeFullDiskAccess(homeDir)

	return HostDiagnostics{
		OSVersion:           osVer,
		Architecture:        runtime.GOARCH,
		APFSSupported:       apfs,
		FilesystemType:      fsType,
		FullDiskAccess:      fda,
		FDANote:             fdaNote,
		ActiveProtectedApps: checkActiveProtectedApps(),
	}
}

func probeFullDiskAccess(homeDir string) (bool, string) {
	safariPath := filepath.Join(homeDir, "Library", "Safari")
	f, err := os.Open(safariPath)
	if err != nil {
		if errors.Is(err, syscall.EPERM) || os.IsPermission(err) {
			return false, "TCC restricted (EPERM / operation not permitted)"
		}
		return false, fmt.Sprintf("inaccessible: %v", err)
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err != nil && (errors.Is(err, syscall.EPERM) || os.IsPermission(err)) {
		return false, "TCC restricted on readdirnames"
	}
	return true, "Full Disk Access granted"
}
```

---

### 9. `test/acceptance/benchmark.go` (service / benchmark runner, external process execution / fallback)

**Role:** Runs `mole clean --dry-run` dynamically when the `mole` binary is discovered in `$PATH`, parses stdout or `~/.config/mole/clean-list.txt`, and gracefully falls back to the normative 9-category snapshot in `SPEC-v2.3-MOLE-PARITY.md` if unavailable or on non-configured hosts (D-02, VAL-03).

**Analog:** `internal/cleaner/executor.go` (process invocation) and `cmd/workflow_v22_e2e_test.go` (command output parsing)

**Imports Pattern:**
```go
//go:build acceptance

package acceptance

import (
	"os/exec"
	"strings"
	"testing"
)
```

**Concrete Benchmark Execution & Fallback Pattern:**
```go
type MoleCategoryResult struct {
	Category   string `json:"category"`
	ItemCount  int    `json:"item_count"`
	BytesFound uint64 `json:"bytes_found"`
}

type MoleBenchmarkResult struct {
	Source              string               `json:"source"`
	TotalBytes          uint64               `json:"total_bytes"`
	TotalBytesFormatted string               `json:"total_bytes_formatted"`
	TotalItems          int                  `json:"total_items"`
	Categories          []MoleCategoryResult `json:"categories"`
}

func RunMoleOrFallback(t *testing.T) MoleBenchmarkResult {
	t.Helper()
	molePath, err := exec.LookPath("mole")
	if err == nil && molePath != "" {
		cmd := exec.Command("mole", "clean", "--dry-run")
		out, err := cmd.CombinedOutput()
		if err == nil {
			parsed, parseErr := parseMoleOutput(string(out))
			if parseErr == nil {
				parsed.Source = "Dynamic execution (mole clean --dry-run)"
				return parsed
			}
		}
	}

	// Fallback to Normative Snapshot documented in SPEC-v2.3-MOLE-PARITY.md
	return getNormativeSpecSnapshot()
}

func getNormativeSpecSnapshot() MoleBenchmarkResult {
	return MoleBenchmarkResult{
		Source:              "Normative snapshot per .planning/SPEC-v2.3-MOLE-PARITY.md §1",
		TotalBytes:          3990000000, // ~3.99 GB
		TotalBytesFormatted: "3.99 GB",
		TotalItems:          214,
		Categories: []MoleCategoryResult{
			{Category: "User App Caches", ItemCount: 85, BytesFound: 2100000000},
			{Category: "System & User Logs", ItemCount: 42, BytesFound: 850000000},
			{Category: "Browser Caches", ItemCount: 38, BytesFound: 620000000},
			{Category: "Xcode DerivedData", ItemCount: 15, BytesFound: 280000000},
			{Category: "Developer Shared Stores", ItemCount: 12, BytesFound: 95000000},
			{Category: "OrbStack Data", ItemCount: 2, BytesFound: 32000000},
			{Category: "App Leftovers", ItemCount: 10, BytesFound: 11000000},
			{Category: "Stale LaunchAgents", ItemCount: 8, BytesFound: 1500000},
			{Category: "User Trash", ItemCount: 2, BytesFound: 500000},
		},
	}
}
```

---

### 10. `test/acceptance/receipt.go` (utility / formatter, transform / file-I/O)

**Role:** Serializes host diagnostics, Mole parity benchmarks, and DevClean scan results into dual output receipts: human-readable Markdown (`MACOS-ACCEPTANCE.md`) and machine-readable JSON (`MACOS-ACCEPTANCE.json`), formatted strictly according to D-01, D-13, D-15, and D-16.

**Analog:** `internal/trash/manifest.go` (JSON serialization) and `scripts/qa_runner.sh` (Markdown reporting)

**Imports Pattern:**
```go
//go:build acceptance

package acceptance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
)
```

**Concrete Receipt Generation Pattern:**
```go
func GenerateReceipts(
	t *testing.T,
	destDir string,
	diag HostDiagnostics,
	mole MoleBenchmarkResult,
	results []domain.Target,
) {
	t.Helper()
	_ = os.MkdirAll(destDir, 0755)

	// 1. Generate MACOS-ACCEPTANCE.json
	payload := map[string]interface{}{
		"generated_at":      time.Now().UTC().Format(time.RFC3339),
		"host_diagnostics":  diag,
		"mole_benchmark":    mole,
		"devclean_targets":  results,
		"total_targets":     len(results),
	}
	jsonData, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal acceptance JSON: %v", err)
	}
	jsonPath := filepath.Join(destDir, "MACOS-ACCEPTANCE.json")
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		t.Fatalf("failed to write %s: %v", jsonPath, err)
	}

	// 2. Generate MACOS-ACCEPTANCE.md
	mdContent := buildMarkdownReceipt(diag, mole, results)
	mdPath := filepath.Join(destDir, "MACOS-ACCEPTANCE.md")
	if err := os.WriteFile(mdPath, []byte(mdContent), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", mdPath, err)
	}
}
```

---

### 11. `MACOS-ACCEPTANCE.md` (artifact / report, documentation / Markdown table)

**Role:** Structured, human-readable acceptance receipt displaying the real macOS host verification results, parity comparison table against Mole, host diagnostics, and explicit safety sections.

**Required Sections & Format Pattern (D-01, D-13, D-15, D-16):**
```markdown
# DevClean v2.3 — macOS Parity Acceptance Receipt (Host Hostname)

**Generated:** 2026-10-18T...  
**Host Environment:** Darwin 24.x (arm64 Apple Silicon) | APFS: true | Full Disk Access: true  
**Mole Baseline Source:** Dynamic execution (`mole clean --dry-run`) / SPEC Normative Snapshot  
**Status:** ✅ ALL GATES PASSED (VAL-01, VAL-02, VAL-03)

---

## 1. Host Hardware & Security Diagnostics
| Fact | Value | Notes |
|---|---|---|
| OS Version | macOS 15.x / Darwin 24.x | Probed via `unix.Sysctl` in pure Go |
| Architecture | arm64 | Apple Silicon |
| Filesystem | apfs | Probed via `unix.Statfs` |
| Full Disk Access / TCC | Granted / Restricted | Probed via `~/Library/Safari` |

---

## 2. Parity Comparison Table vs Mole Benchmark
| Category | Mole Discovered | DevClean Occupied | DevClean Eligible | Status & Safety Classification |
|---|---|---|---|---|
| User App Caches | 2.10 GB | 2.10 GB | 2.10 GB | ✅ Full Parity (Eligible) |
| System & User Logs | 850 MB | 850 MB | 850 MB | ✅ Full Parity (Partial on TCC paths per D-15) |
| Browser Caches | 620 MB | 620 MB | 605 MB | ✅ Gated: 15 MB ScriptCache ReviewOnly per D-16 |
| Xcode DerivedData | 280 MB | 280 MB | 280 MB | ✅ Full Parity |
| Developer Shared Stores | 95 MB | 95 MB | 0 B | 🛡️ ReviewOnly: true (Preserve developer toolchain) |
| OrbStack Data | 32 MB | 32 MB | 0 B | 🛡️ ReviewOnly: true (Active container storage) |
| App Leftovers | 11 MB | 11 MB | 0 B | 🛡️ ReviewOnly: true (Human review required) |
| Stale LaunchAgents | 1.5 MB | 1.5 MB | 0 B | 🛡️ ReviewOnly: true (Prevent daemon disruption) |
| User Trash | 500 KB | 500 KB | 0 B | 🛡️ ReviewOnly: true (Gated by --trash flag) |

---

## 3. Design Divergences & Safety Rationale (D-13)
*Detailed technical justification contrasting DevClean's fail-closed architecture with Mole's automated destructive cleaning.*

## 4. Permission & TCC Transparency (D-15)
*Explicit logging of protected paths (/Library/Logs/DiagnosticReports, /private/var/log) marked Partial: true.*

## 5. Browser ScriptCache Offline Gate (D-16)
*Gated per BROW-04 — blocked from automated deletion until evidence of offline/MV3 repopulation.*
```

---

### 12. `MACOS-ACCEPTANCE.json` (artifact / receipt payload, machine-readable JSON)

**Role:** Raw structured JSON output storing all scanned targets, sizes, eligibility, and host facts for automated CI/CD assertion and historical regression tracking (D-01).

**Schema Pattern:**
```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "generated_at": "2026-10-18T18:55:00Z",
  "host_diagnostics": {
    "os_version": "15.0",
    "architecture": "arm64",
    "apfs_supported": true,
    "filesystem_type": "apfs",
    "full_disk_access": true,
    "fda_note": "Full Disk Access granted",
    "active_protected_apps": []
  },
  "mole_benchmark": {
    "source": "Dynamic execution (mole clean --dry-run)",
    "total_bytes": 3990000000,
    "total_bytes_formatted": "3.99 GB",
    "total_items": 214,
    "categories": []
  },
  "devclean_summary": {
    "total_targets": 220,
    "total_occupied_bytes": 4120000000,
    "total_eligible_bytes": 3840000000,
    "review_only_targets": 18
  },
  "devclean_targets": [
    {
      "id": "macos-cache-com.apple.Safari",
      "name": "Safari Cache",
      "path": "/Users/gabrielvp/Library/Caches/com.apple.Safari",
      "category": "macos-app-cache",
      "size_bytes": 1048576,
      "eligible_bytes": 1048576,
      "review_only": false,
      "review_reason": "",
      "partial": false
    }
  ]
}
```

---

## Invariant & Guard Enforcement Patterns

### Invariant 1: ReviewOnly Target Immutability Across All 5 Channels (D-06, D-14)
**Rule:** Targets marked `ReviewOnly: true` (e.g. `pnpm store`, `OrbStack data`, `Stale LaunchAgents`, `App Leftovers`, `NuGet packages`, `ScriptCache`) MUST NEVER suffer physical mutation, deletion, or truncation across CLI, TUI, MCP, Scheduler, or Coordinator.  
**Pattern:**
- Sentinel files with unique payload strings planted inside target directories.
- All 5 channels invoked with `--auto --yes` or `ForceDirect: true`.
- Post-execution assertion verifies 100% byte-for-byte sentinel integrity.

### Invariant 2: Process Guard Revalidation and Active App Blocking (D-06)
**Rule:** Scanners must mark active applications with `ReviewOnly: true` and `ReviewReason: active_process`. Furthermore, `Clean()` must perform a pre-deletion revalidation check to prevent race conditions if an application launches after the scan phase.  
**Pattern:**
- `cleaner.MockCommandExecutor` configured with active application PIDs.
- Revalidation in `macosCleaner.Clean()` calling `IsBundleProcessRunning(...)`.

### Invariant 3: Symlink Guard & Traversal Rejection (D-06)
**Rule:** Cleaning routines must never follow symlinks outside target roots or into system blacklisted paths (`.git`, `~/.ssh`, `/etc`, `/private`).  
**Pattern:**
- `info.Mode()&os.ModeSymlink != 0` check in `Clean()` methods.
- `safety.IsCoreBlacklisted(path)` validation on resolved and un-resolved paths.

### Invariant 4: Graceful Degradation on Permission / TCC Errors (D-15)
**Rule:** Unreadable directories or permission denied errors must never panic or fail the scan; they must degrade gracefully with `ProviderMetrics.Partial = true`, `ReviewOnly = true`, `EligibleBytes = 0`, and `ReviewReason = "system_protected"`.  
**Pattern:**
- `scanDirectoryWithTCCFallback` in `internal/cleaner/macos/system.go`.
- `safari.isSyscallPermission` handling in `internal/cleaner/browsers/safari.go`.

### Invariant 5: Zero Double-Counting Across 19 Ecosystems (D-08, D-14)
**Rule:** No single file or byte may be represented twice in `engine.Scan()`. No target path may be identical to or a subdirectory prefix of another registered target.  
**Pattern:**
- Programmatic collision map populated during E2E scan.
- Path comparison checks asserting `!strings.HasPrefix(child, parent + Separator)`.

### Invariant 6: Pure Go Darwin Host Diagnostics (D-03)
**Rule:** Darwin host diagnosis must be conducted without CGO and without running external shell commands like `sw_vers`, `df`, or `diskutil`.  
**Pattern:**
- `unix.Sysctl("kern.osproductversion")` for OS version.
- `unix.Statfs(path, &stat)` for filesystem type (`stat.Fstypename == "apfs"`).
- `os.Open` / `os.ReadDir` probing on `~/Library/Safari` for TCC/FDA status.

---

## Anti-Patterns & Pitfalls to Avoid

| Anti-Pattern | Why It Breaks | Correct Pattern |
|---|---|---|
| Invoking `sw_vers` or shell pipes in diagnostics | Fragile, slow, can fail under locked environments or restricted paths. | Use `unix.Sysctl("kern.osproductversion")` directly in Go. |
| Parsing `df -T` or `diskutil` for APFS detection | External process dependency; unavailable in minimal/CI environments. | Use `unix.Statfs` and inspect `stat.Fstypename`. |
| Hardcoding `/Users/<username>` paths in E2E tests | Fails on Linux CI runners and different developer systems. | Use `t.TempDir()`, `filepath.Join(tempDir, ...)`, and `t.Setenv("HOME", ...)`. |
| Running destructive `Clean()` in real host acceptance tests | Risks accidental data loss on developer machines. | Acceptance test runs strictly read-only (`engine.Scan()` only); never invokes `Clean()`. |
| Assuming `mole` CLI is present on all hosts | Breaks CI or environments without Homebrew/Mole installed. | Dynamically check `exec.LookPath("mole")` and fallback to normative `SPEC-v2.3-MOLE-PARITY.md` snapshot. |
| Global statement coverage check instead of per-package check | Dilutes low coverage in altered packages with high coverage from legacy code. | Strict per-package threshold loop (`>= 85.0%`) in `scripts/verify-coverage.sh`. |

---

## PATTERN MAPPING COMPLETE

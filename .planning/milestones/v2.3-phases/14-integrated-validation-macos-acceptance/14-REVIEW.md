---
phase: "14"
status: passed
critical_count: 0
warning_count: 0
info_count: 2
coverage_status: passed
---

# Phase 14: Integrated Validation & macOS Acceptance — Code Review Report

**Phase:** 14-integrated-validation-macos-acceptance  
**Depth:** standard  
**Files Reviewed:** 12  
**Status:** passed (1 warning remediated and verified with `go test -race`)  
**Coverage Status:** passed (all 11 target packages meet or exceed >= 85.0%)  

---

## 1. Summary

An in-depth standard code review was conducted across the 12 files delivered during **Phase 14 (Integrated Validation & macOS Acceptance)**.

The deliverable achieves the requirements outlined in the Phase 14 plans (`14-01-PLAN.md`, `14-02-PLAN.md`, `14-03-PLAN.md`) and validates the complete DevClean v2.3 architecture:

1. **Multi-Channel Review-Only Invariant Enforcement (`cmd/workflow_v23_e2e_test.go`):**
   - End-to-end hermetic testing validates that the 5 inviolable review-only sentinels (`pnpm store`, `OrbStack VM data`, `LaunchAgent stale plist`, `DeadApp leftover`, `Chrome ScriptCache`) remain 100% intact across all 5 user and automated interaction channels: CLI (`clean --auto --yes`), Deleter Coordinator (`coord.Delete`), MCP Server (`propose_cleanup` / `execute_cleanup`), Scheduler (`schedule run`), and TUI Bubble Tea headless harness.
   - Fault injection tests prove active process protection (Chrome, OrbStack, Xcode), symlink directory escape containment (`.ssh` / `.git` isolation), zero double-counting across ecosystems, and graceful degradation on daemon communication failures.

2. **Zero-CGO Static Cross-Compilation (`scripts/verify-builds.sh`):**
   - Validates that DevClean compiles statically with `CGO_ENABLED=0` across all 4 target architectures: `darwin/arm64`, `darwin/amd64`, `linux/amd64`, and `linux/arm64`.

3. **Per-Package Statement Coverage Floor (`scripts/verify-coverage.sh`):**
   - All 11 altered or new packages surpass the required 85.0% floor (ranging from 85.9% in `internal/cleaner/macos` to 100.0% in `internal/domain`).

4. **Live macOS Parity & Acceptance Suite (`test/acceptance/`):**
   - Pure Go host diagnostics (`unix.Sysctl` for OS version, `unix.Statfs` for APFS detection, direct non-prompting `os.Open` of `~/Library/Safari` for Full Disk Access probing, and `pgrep` for active process guards) execute with zero shell dependencies and zero CGO.
   - Dual acceptance receipts (`MACOS-ACCEPTANCE.md` and `MACOS-ACCEPTANCE.json`) are generated in the phase directory, verifying zero target duplication, zero eligible bytes on review-only targets, zero unjustified category omissions, and strict ScriptCache / LaunchAgent gating.

The review identified **1 Warning** (a data race in `cleaner.MockCommandExecutor` exposed when running `go test -race ./cmd -run TestE2E_V23`) and **2 Info** items (output capturing in `verify-builds.sh` and `os.UserHomeDir()` resolution in `benchmark.go`).

---

## 2. Scope of Review

The following 12 source, test, and verification files were reviewed:

- `cmd/workflow_v23_e2e_test.go`: Multi-channel hermetic E2E tests, fault-injection harnesses, and zero double-counting verification.
- `internal/cleaner/browsers/gap_test.go`: Unit tests for browser cleaner constructors, permission helpers, URI parsing, symlink defenses, and context cancellation.
- `internal/cleaner/leftovers/gap_test.go`: Unit tests for leftovers cleaner defaults, safe symlink traversals, corrupt plists, and unconditional deletion refusal.
- `internal/cleaner/macos/gap_test.go`: Unit tests for macOS cleaner bundle ID resolution, TCC fallback, trash errors, and defensive edge cases.
- `scripts/verify-builds.sh`: Bash script verifying 4-platform static cross-compilation with `CGO_ENABLED=0`.
- `scripts/verify-coverage.sh`: Bash script auditing per-package statement coverage across 11 target packages.
- `test/acceptance/benchmark.go`: Mole parity benchmark extraction and fallback normative snapshot mapping.
- `test/acceptance/diagnostics.go`: Host diagnostics structures and pure Go TCC / Full Disk Access probing.
- `test/acceptance/diagnostics_darwin.go`: Darwin-specific syscall diagnostics via `unix.Sysctl` and `unix.Statfs`.
- `test/acceptance/diagnostics_other.go`: Non-darwin fallback diagnostics.
- `test/acceptance/macos_test.go`: Live acceptance tests asserting the 4 safety invariants and orchestrating receipt generation.
- `test/acceptance/receipt.go`: Dual receipt generation logic (`MACOS-ACCEPTANCE.md` and `MACOS-ACCEPTANCE.json`).

---

## 3. Findings by Severity

### Critical (0)

No critical issues, security vulnerabilities, or data loss bugs were detected.

---

### Warning (0 open, 1 remediated)

#### WR-01: Data Race in `cleaner.MockCommandExecutor` During Concurrent `engine.Scan(ctx)` [RESOLVED]

- **Files:** `internal/cleaner/executor_mock.go:43-50` (invoked via `cmd/workflow_v23_e2e_test.go:449, 588`)
- **Severity:** Warning (Remediated in commit `25c8796`)
- **Status:** Resolved & Verified with `go test -race ./cmd -run TestE2E_V23` (PASS)
- **Description:**
  In `cmd/workflow_v23_e2e_test.go`, the fault injection tests `TestE2E_V23_FaultInjection_ProcessGuard` and `TestE2E_V23_FaultInjection_GracefulDegradation` register a shared mock executor across all default cleaners:
  ```go
  mockExec := cleaner.NewMockExecutor()
  // ... configure outputs / errors ...
  engine.RegisterDefaultCleanersWithExecutor(eng, cfg, resolver, mockExec)
  results := engine.Collect(ctx, eng.Scan(ctx))
  ```
  `eng.Scan(ctx)` launches concurrent goroutines (via `errgroup`) for each registered cleaner (`docker`, `xcode`, `pnpm`, `homebrew`, `ai`, etc.). As multiple cleaners concurrently invoke `mockExec.Output(ctx, name, args...)`, concurrent unsynchronized writes occurred on `m.Calls` (`m.Calls = append(...)`) and concurrent unsynchronized reads/writes occurred on `m.Errors` and `m.Outputs`.

- **Remediation Applied:**
  Protected `MockCommandExecutor` internal state with `sync.RWMutex`:
  Acquired `m.mu.Lock()` / `defer m.mu.Unlock()` in `Output()`, and `m.mu.RLock()` in `LookPath()`.
  Verified zero data races with `go test -race ./cmd -run TestE2E_V23`.


---

### Info (2)

#### IN-01: In-Line Error Output Interleaving in `scripts/verify-builds.sh`

- **File:** `scripts/verify-builds.sh:33`
- **Severity:** Info
- **Description:**
  Line 33 tests static builds with `if CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -o /dev/null . 2>&1; then`. Because standard error is redirected to standard output without buffering, any compilation error or diagnostic from `go build` is printed directly after `Building static binary for $os/$arch... ` before the `✖ FAILED` label.
- **Remediation:**
  Capture the output in a local variable:
  ```bash
  if build_out=$(CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -o /dev/null . 2>&1); then
    echo -e "${GREEN}✔ SUCCESS${NC}"
  else
    echo -e "${RED}✖ FAILED${NC}"
    echo "$build_out"
    FAILURES=$((FAILURES + 1))
  fi
  ```

#### IN-02: Use `os.UserHomeDir()` for Mole Config Path Resolution

- **File:** `test/acceptance/benchmark.go:45`
- **Severity:** Info
- **Description:**
  `cleanListPath := filepath.Join(os.Getenv("HOME"), ".config", "mole", "clean-list.txt")` assumes `$HOME` is set. In headless or daemon environments where `$HOME` might be unset, this evaluates to a relative path `".config/mole/clean-list.txt"`.
- **Remediation:**
  Use `home, err := os.UserHomeDir()` with fallback to `os.Getenv("HOME")`, matching the pattern used in `test/acceptance/diagnostics.go:36`.

---

## 4. Verification Checks

| Check | Command | Result | Notes |
|---|---|---|---|
| Static Zero-CGO Matrix | `./scripts/verify-builds.sh` | **PASS** | `darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64` |
| Statement Coverage Floor | `./scripts/verify-coverage.sh` | **PASS** | All 11 target packages >= 85.0% |
| E2E Multi-Channel Tests | `go test -v ./cmd -run TestE2E_V23` | **PASS** | 5 channels + 4 fault injections pass (7.87s) |
| macOS Acceptance Suite | `go test -tags=acceptance ./test/acceptance/...` | **PASS** | 4 safety invariants verified, dual receipts written (101.47s) |
| Acceptance Race Detector | `go test -tags=acceptance -race ./test/acceptance/...` | **PASS** | Clean execution with 0 races (108.98s) |
| Cleaner Race Detector | `go test -race ./internal/cleaner/browsers/... ./internal/cleaner/leftovers/... ./internal/cleaner/macos/...` | **PASS** | All cleaner gap tests race-free |
| Codebase Diagnostics | `go vet ./... && go vet -tags=acceptance ./test/acceptance/...` | **PASS** | 0 compiler or vet warnings |

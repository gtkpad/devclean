---
phase: 13-app-leftovers-launchagents
plan: 03
subsystem: leftovers
tags: [cleaner-lifecycle, zero-mutation, engine-registry, coordinator-policy, hermetic-testing, zero-cgo, macos]

# Dependency graph
requires:
  - phase: 13-app-leftovers-launchagents
    provides: LaunchAgent scanner (13-01), App indexer, exclusions shield, and leftovers scanner (13-02)
provides:
  - leftoversCleaner lifecycle orchestrator implementing domain.Cleaner ("leftovers") with injectable directory roots
  - Inviolable Clean() defensive refusal guard unconditionally returning nil to guarantee zero mutations (LEFT-04, D-13)
  - Engine registration for 22 ecosystems sorted alphabetically ("ai" through "xcode")
  - CLI and configuration wiring for leftovers ecosystem and hook definitions
  - Coordinator zero-mutation integration canary test (TestCoordinator_DryRun_ZeroMutation_Phase13)
affects: [13-app-leftovers-launchagents, cleaner, engine, deleter, config, cmd]

# Actuals
actuals:
  tokens: 16000
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns: [defensive-refusal-guard, root-directory-injection, hermetic-lifecycle-testing, zero-mutation-canary, 22-ecosystem-sorting]

key-files:
  created:
    - internal/cleaner/leftovers/cleaner.go
    - internal/cleaner/leftovers/cleaner_test.go
  modified:
    - internal/config/config.go
    - internal/config/defaults.go
    - internal/config/validate.go
    - internal/engine/register.go
    - internal/engine/engine.go
    - internal/engine/register_test.go
    - cmd/clean.go
    - cmd/clean_test.go
    - cmd/scan_test.go
    - internal/deleter/policy_test.go

key-decisions:
  - "D-13: Inviolable defensive refusal guard in Clean() returning nil unconditionally, guaranteeing zero mutations even under test or coordinator bypass"
  - "D-14: Engine registration of leftovers cleaner with executor and full configuration integration"
  - "D-15: Injectable root paths (appRoots, launchAgentsDirs, libraryDir) enabling 100% hermetic testing in t.TempDir() without CGO across Darwin and Linux"
  - "D-16: Alphabetical registry ordering maintaining 22 ecosystems ('ai' through 'xcode') with 'leftovers' positioned between 'java' and 'macos'"

patterns-established:
  - "Defensive refusal guard: Unconditionally returning nil in Clean() ensures physical deletion is impossible even if called directly or bypassed by upstream logic"
  - "Custom root injection: Constructors NewWithConfig accept custom slice paths for appRoots, launchAgentsDirs, and libraryDir, decoupling the cleaner from host filesystem paths for hermetic tests"
  - "Canary zero-mutation testing: Hash, size, and modtime validation before and after DryRun and live Delete verifying zero disk mutations"

requirements-completed:
  - LEFT-04
  - LEFT-01
  - LEFT-02
  - LEFT-03
  - VAL-01
  - VAL-02
  - POL-01
  - POL-02
  - POL-04
  - SCAN-02
  - SCAN-04

coverage:
  - id: D1
    description: "Leftovers cleaner lifecycle orchestrator implementing domain.Cleaner with Name() == 'leftovers' and injectable roots"
    requirement: "LEFT-01"
    verification:
      - kind: unit
        ref: "internal/cleaner/leftovers/cleaner_test.go#TestLeftovers_FullLifecycle_Hermetic"
        status: pass
    human_judgment: false
  - id: D2
    description: "Inviolable Clean() defensive refusal guard preserving 100% of sentinels on disk"
    requirement: "LEFT-04"
    verification:
      - kind: unit
        ref: "internal/cleaner/leftovers/cleaner_test.go#TestLeftovers_CleanRefusesDeletion"
        status: pass
      - kind: unit
        ref: "internal/cleaner/leftovers/cleaner_test.go#TestLeftovers_DryRun"
        status: pass
    human_judgment: false
  - id: D3
    description: "Engine registration and config wiring for 22 alphabetically sorted ecosystems"
    requirement: "VAL-02"
    verification:
      - kind: unit
        ref: "internal/engine/register_test.go#TestRegisterDefaultCleaners"
        status: pass
      - kind: unit
        ref: "internal/engine/register_test.go#TestRegisterDefaultCleanersWithExecutor"
        status: pass
    human_judgment: false
  - id: D4
    description: "Coordinator zero-mutation canary test asserting zero bytes freed and zero mutations on ReviewOnly leftover targets"
    requirement: "POL-04"
    verification:
      - kind: unit
        ref: "internal/deleter/policy_test.go#TestCoordinator_DryRun_ZeroMutation_Phase13"
        status: pass
    human_judgment: false

duration: 18min
completed: 2026-10-18
status: complete
---

# Phase 13 Plan 03: Cleaner Lifecycle, Inviolable Clean() Guard, Engine Registration & Canary Integration Summary

**Leftovers cleaner lifecycle orchestrator (`cleaner.go`) with dependency injection and inviolable defensive refusal guard (`Clean()`), engine registry expansion to 22 ecosystems, config/CLI wiring, and Coordinator zero-mutation canary testing.**

## Performance

- **Duration:** 18 min
- **Started:** 2026-10-18T12:32:00-03:00
- **Completed:** 2026-10-18T12:54:00-03:00
- **Tasks:** 2
- **Files modified:** 12

## Accomplishments
- Implemented `leftoversCleaner` lifecycle orchestrator (`cleaner.go`) implementing `domain.Cleaner` under ecosystem `"leftovers"` with constructors (`New()`, `NewWithExecutor(exec)`, `NewWithConfig(...)`) allowing custom roots injection (`appRoots`, `launchAgentsDirs`, `libraryDir`) for 100% hermetic testing across Darwin and Linux without CGO.
- Enforced the inviolable non-deletion contract (`LEFT-04`, `D-13`) in `Clean()`, which unconditionally returns `nil` without deleting or unlinking any file, verified by sentinel hash/size/modtime preservation tests.
- Wired `leftovers` ecosystem and hooks into configuration (`config.go`, `defaults.go`, `validate.go`) with `Enabled: true, MaxAgeDays: 0, FailMode: "continue"`.
- Registered `leftovers.NewWithExecutor(exec)` in `internal/engine/register.go` expanding the engine registry to 22 ecosystems.
- Updated `internal/engine/register_test.go` asserting all 22 ecosystems sorted alphabetically (`ai`, `browsers`, `bun`, `cocoapods`, `cpp`, `docker`, `dotnet`, `elixir`, `flutter`, `go`, `homebrew`, `java`, `leftovers`, `macos`, `nextjs`, `node`, `pnpm`, `python`, `rust`, `shell`, `terraform`, `xcode`).
- Added `case "leftovers"` to `cmd/clean.go` `getMaxAgeDays`.
- Created integration canary test `TestCoordinator_DryRun_ZeroMutation_Phase13` in `internal/deleter/policy_test.go` proving stale LaunchAgents and app leftovers are rejected by Coordinator Stage 1 as `ReviewOnly`, guaranteeing 0 bytes freed and zero mutations on disk under both dry-run and live runs.
- Verified that the entire project test suite passes with `CGO_ENABLED=0 go test ./...`.

## Task Commits

Each task was committed atomically:

1. **Task 13-03-01: Leftovers cleaner lifecycle orchestrator with dependency injection and inviolable Clean() defensive refusal guard (LEFT-04, LEFT-01, LEFT-02, LEFT-03, D-13, D-14, D-15, VAL-01)** - `d12fa97` (feat)
2. **Task 13-03-02: Engine registration for 22 ecosystems, config wiring, clean CLI wiring, and Coordinator zero-mutation integration test (LEFT-01..04, VAL-01, VAL-02, POL-01, POL-04, D-14, D-16)** - `1d4339d` (feat)

## Files Created/Modified
- `internal/cleaner/leftovers/cleaner.go` - `leftoversCleaner` struct, constructors with root injection, and inviolable `Clean()` refusal guard.
- `internal/cleaner/leftovers/cleaner_test.go` - Lifecycle hermetic test, sentinel preservation test, and dry-run test.
- `internal/config/config.go` - Added `Leftovers` to `EcosystemsConfig` and `HooksConfig`, and viper defaults in `Load()`.
- `internal/config/defaults.go` - Added `Leftovers` defaults (`Enabled: true, MaxAgeDays: 0`).
- `internal/config/validate.go` - Added `macos`, `browsers`, and `leftovers` to allowed clean values and hooks validation.
- `internal/engine/register.go` - Registered `leftovers.NewWithExecutor(exec)` in engine.
- `internal/engine/engine.go` - Mapped `"leftovers"` to `e.config.Ecosystems.Leftovers`.
- `cmd/clean.go` - Mapped `"leftovers"` to `cfg.Ecosystems.Leftovers.MaxAgeDays` in `getMaxAgeDays`.
- `internal/engine/register_test.go` - Verified 22 alphabetically sorted ecosystems and leftovers configuration mapping.
- `internal/deleter/policy_test.go` - Added `TestCoordinator_DryRun_ZeroMutation_Phase13` canary test.
- `cmd/clean_test.go` & `cmd/scan_test.go` - Updated test config helpers to explicitly disable `leftovers` where empty configs are tested on host systems.

## Decisions Made
- Followed decisions D-13, D-14, D-15, D-16 as specified in the plan.
- Added `leftovers: enabled: false` to test config generators in `cmd/clean_test.go` and `cmd/scan_test.go` so host macOS LaunchAgents are not scanned during empty-config CLI tests.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Disabled leftovers in mock CLI test configs**
- **Found during:** Task 13-03-02 (`go test ./cmd`)
- **Issue:** Mock test configs in `cmd/clean_test.go` and `cmd/scan_test.go` explicitly disabled prior 21 ecosystems to verify behavior when nothing exists to clean. Because `leftovers` was enabled by default, the cleaner scanned real host `/Library/LaunchAgents` paths on macOS during test execution, causing `TestCleanCmd_NothingToClean_ReturnsError` to find host targets.
- **Fix:** Added `leftovers: enabled: false` alongside `macos` and `browsers` in `cmd/scan_test.go` and `cmd/clean_test.go` test fixtures.
- **Files modified:** `cmd/scan_test.go`, `cmd/clean_test.go`
- **Verification:** `go test -v ./cmd` passed cleanly across all test cases.
- **Committed in:** `1d4339d` (Task 13-03-02 commit)

---

**Total deviations:** 1 auto-fixed (test fixture parity across 22 ecosystems)
**Impact on plan:** Essential for test isolation on macOS developer environments. No scope creep.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 13 (App Leftovers & LaunchAgents) is complete across all 3 plans (13-01, 13-02, 13-03).
- All requirements (LEFT-01, LEFT-02, LEFT-03, LEFT-04, VAL-01, VAL-02, POL-01, POL-02, POL-04, SCAN-02, SCAN-04) are fully implemented and verified.
- Full test suite passes under `CGO_ENABLED=0 go test ./...`.

---
*Phase: 13-app-leftovers-launchagents*
*Completed: 2026-10-18*

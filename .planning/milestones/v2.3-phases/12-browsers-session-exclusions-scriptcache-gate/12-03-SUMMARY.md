---
phase: 12-browsers-session-exclusions-scriptcache-gate
plan: 03
subsystem: cleaner/browsers
tags: [browsers, lifecycle, orchestrator, engine, config, macos-deduplication, dry-run, canary, tdd]

# Dependency graph
requires:
  - phase: 12-01
    provides: Browser definitions, Inviolable Session Guard, Process Guard, test fixtures
  - phase: 12-02
    provides: Chromium multi-profile scanner, Firefox scanner, Safari scanner, ScriptCache gating
provides:
  - BrowserCleaner orchestrator implementing domain.Cleaner ("browsers") with multi-browser dispatching (BROW-01, BROW-02)
  - Active defensive refusal guard in Clean() skipping ReviewOnly and ScriptCache targets (BROW-04, D-05, D-07)
  - Atomic pre-clean process revalidation in Clean() failing closed on concurrent process start (D-10)
  - macOS generic user cache deduplication exclusions for 6 browsers in dedicatedEcosystemCaches (SCAN-04)
  - Engine registration, CLI clean MaxAgeDays routing, and config defaults (Enabled: true, MaxAgeDays: 14) (BROW-01)
  - TestCoordinator_DryRun_ZeroMutation_Phase12 verifying zero bytes mutated and canary SHA256 preservation (POL-04, VAL-01, VAL-02)
affects: [phase-13, phase-14]

# Actuals
actuals:
  tokens: 28000
  tasks: 2
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Browser cleaner orchestrator dispatching to Chromium, Gecko, and WebKit sub-scanners
    - Dual-layer Clean() defensive refusal guard against ReviewOnly and ScriptCache mutation
    - Pre-clean atomic process revalidation immediately prior to unlinking preventing user races
    - Cache folder deduplication via dedicatedEcosystemCaches table preventing double-counting with macos cleaner
    - Dry-run coordinator canary asserting 100% SHA256, byte size, and mtime invariance

key-files:
  created:
    - internal/cleaner/browsers/browsers.go
    - internal/cleaner/browsers/browsers_test.go
  modified:
    - internal/cleaner/macos/user.go
    - internal/cleaner/macos/user_test.go
    - internal/config/config.go
    - internal/config/config_test.go
    - internal/config/defaults.go
    - internal/engine/register.go
    - internal/engine/register_test.go
    - internal/engine/engine.go
    - cmd/clean.go
    - cmd/clean_test.go
    - cmd/scan_test.go
    - internal/deleter/policy_test.go

key-decisions:
  - "D-01 & D-02: Orchestrator presents consolidated browser://<id>/caches targets across Chromium, Gecko, and WebKit engines"
  - "D-05 & D-07: Active defensive refusal guard directly in Clean() rejecting any ReviewOnly or Category browser-scriptcache targets"
  - "D-10: Atomic pre-clean process revalidation immediately before file unlinking, failing closed if a browser or helper was launched post-scan"
  - "SCAN-04: Added Google, BraveSoftware, company.thebrowser.Arc, Microsoft Edge, Firefox, and com.apple.Safari to dedicatedEcosystemCaches to prevent double-counting"
  - "POL-04 & VAL-01: Coordinator dry-run canary TestCoordinator_DryRun_ZeroMutation_Phase12 proves zero mutation across browser caches and session sentinels"
  - "VAL-02: 100% pure Go compilation with CGO_ENABLED=0 across the entire repository test suite"

patterns-established:
  - "browsers.New, browsers.NewWithExecutor, browsers.NewWithHome, browsers.NewWithExecutorAndHome: Canonical cleaner constructor pattern"
  - "browsers.extractBrowserIDFromURI: Deterministic URI schema parser for browser://<id>/caches virtual targets"
  - "Clean() 4-stage pipeline: Context check -> Defensive refusal -> Pre-clean process revalidation -> Surgical unlinking"

requirements-completed: [BROW-01, BROW-02, BROW-03, BROW-04, VAL-01, VAL-02, POL-04, SCAN-04]

# Coverage metadata
coverage:
  - id: D1
    description: "Browser cleaner orchestrator implementing domain.Cleaner interface with multi-browser dispatch"
    requirement: "BROW-01, BROW-02"
    verification:
      - kind: unit
        ref: "internal/cleaner/browsers/browsers_test.go#TestBrowsers_Lifecycle"
        status: pass
    human_judgment: false
  - id: D2
    description: "Defensive refusal guard in Clean() skipping ReviewOnly and ScriptCache targets"
    requirement: "BROW-04"
    verification:
      - kind: unit
        ref: "internal/cleaner/browsers/browsers_test.go#TestBrowsers_DefensiveRefusal"
        status: pass
    human_judgment: false
  - id: D3
    description: "Atomic pre-clean process revalidation failing closed on process startup race"
    requirement: "BROW-01, BROW-04"
    verification:
      - kind: unit
        ref: "internal/cleaner/browsers/browsers_test.go#TestBrowsers_PreCleanRevalidationRace"
        status: pass
    human_judgment: false
  - id: D4
    description: "Inviolable session shield and surgical unlinking of disposable cache entries"
    requirement: "BROW-03, VAL-01"
    verification:
      - kind: unit
        ref: "internal/cleaner/browsers/browsers_test.go#TestBrowsers_InviolableSessionShield_Integration"
        status: pass
    human_judgment: false
  - id: D5
    description: "macOS user cache deduplication exclusions preventing double-counting"
    requirement: "SCAN-04"
    verification:
      - kind: unit
        ref: "internal/cleaner/macos/user_test.go#TestUserCaches_DeduplicationExclusions"
        status: pass
    human_judgment: false
  - id: D6
    description: "Engine registration, config defaults, and CLI routing with 14-day default TTL"
    requirement: "BROW-01"
    verification:
      - kind: unit
        ref: "internal/engine/register_test.go#TestRegisterDefaultCleaners"
        status: pass
      - kind: unit
        ref: "internal/config/config_test.go#TestLoad_NoFile_ReturnsDefaults"
        status: pass
    human_judgment: false
  - id: D7
    description: "Coordinator dry-run zero-mutation canary preserving 100% of canary SHA256 hashes"
    requirement: "POL-04, VAL-01, VAL-02"
    verification:
      - kind: unit
        ref: "internal/deleter/policy_test.go#TestCoordinator_DryRun_ZeroMutation_Phase12"
        status: pass
    human_judgment: false

# Metrics
duration: 20 min
completed: 2026-10-17
status: complete
---

# Phase 12 Plan 03: Browser Cleaner Orchestrator, Engine Integration & Safety Canary Summary

**Primary browser cleaner orchestrator (`browsers.go`), fail-closed pre-clean process revalidation, defensive refusal against ScriptCache mutation, macOS user cache deduplication exclusions (`SCAN-04`), engine/config wiring, and Phase 12 zero-mutation dry-run canary verification (BROW-01..04, VAL-01, VAL-02, POL-04, SCAN-04).**

## Performance

- **Duration:** 20 min
- **Started:** 2026-10-17T11:15:20Z
- **Completed:** 2026-10-17T11:38:00Z
- **Tasks:** 2
- **Files modified/created:** 15

## Accomplishments

- Implemented `internal/cleaner/browsers/browsers.go` implementing `domain.Cleaner` for ecosystem `"browsers"`, providing `New`, `NewWithExecutor`, `NewWithHome`, and `NewWithExecutorAndHome` constructors, on-disk installation detection across all 6 browsers, unified `Scan` dispatching to Chromium, Firefox, and Safari sub-scanners, and a 4-stage surgical `Clean` pipeline.
- Implemented dual-layer defensive refusal in `Clean()` immediately skipping any target with `ReviewOnly: true`, `Category == "browser-scriptcache"`, or URI ending in `"/scriptcache"` without deleting files or erroring out (D-05, D-07).
- Implemented atomic pre-clean process revalidation in `Clean()` querying `CheckBrowserProcessStatus` immediately prior to unlinking, aborting fail-closed if a browser was launched after the scan (D-10).
- Updated `dedicatedEcosystemCaches` in `internal/cleaner/macos/user.go` to exclude `"Google"`, `"BraveSoftware"`, `"company.thebrowser.Arc"`, `"Microsoft Edge"`, `"Firefox"`, and `"com.apple.Safari"`, preventing the generic macOS user cleaner from double-counting browser caches (SCAN-04).
- Registered `browsers` in `internal/config/config.go` (`EcosystemsConfig.Browsers`, `HooksConfig.Browsers`, `Load`), `internal/config/defaults.go` (`Enabled: true, MaxAgeDays: 14`), `internal/engine/register.go` (`browsers.NewWithExecutor(exec)`), `internal/engine/engine.go` (`ecosystemConfig`), and `cmd/clean.go` (`getMaxAgeDays`).
- Implemented `TestCoordinator_DryRun_ZeroMutation_Phase12` in `internal/deleter/policy_test.go` proving zero bytes mutated, zero files deleted, zero trash entries created, and 100% SHA256 invariance on browser cache and session canaries under Coordinator dry-run (POL-04, VAL-01).
- Verified full test suite pass with `CGO_ENABLED=0 go test ./...` across all 43 packages with zero failures (VAL-02).

## Task Commits

Each task was committed atomically following strict TDD discipline (RED → GREEN):

1. **Task 12-03-01: BrowserCleaner implementation with atomic pre-clean process revalidation and defensive refusal**
   - RED test: `3073e96` (`test(12-03): add failing unit tests for browser cleaner orchestrator and pre-clean revalidation`)
   - GREEN feat: `5ae47cf` (`feat(12-03): implement browser cleaner orchestrator with defensive refusal and process revalidation`)
2. **Task 12-03-02: Engine registration, Config defaults, macOS user cache deduplication, and zero-mutation canary**
   - RED test: `9d40195` (`test(12-03): add failing tests for engine registration, deduplication, and zero-mutation canary`)
   - GREEN feat: `082edc6` (`feat(12-03): wire browsers cleaner into engine, config defaults, macos exclusions, and canary test`)

## Files Created/Modified

- `internal/cleaner/browsers/browsers.go` - Primary browser cleaner orchestrator implementing `domain.Cleaner`
- `internal/cleaner/browsers/browsers_test.go` - Lifecycle, defensive refusal, pre-clean process race, and session shield integration tests
- `internal/cleaner/macos/user.go` - Browser cache directory exclusions in `dedicatedEcosystemCaches` (SCAN-04)
- `internal/cleaner/macos/user_test.go` - Deduplication exclusion tests for browser cache folders
- `internal/config/config.go` - Added `Browsers` to `EcosystemsConfig` and `HooksConfig` with mapstructure tags and defaults
- `internal/config/config_test.go` - Default configuration assertions for `Browsers`
- `internal/config/defaults.go` - Default `Browsers` configuration (`Enabled: true, MaxAgeDays: 14, Timeout: "30s"`)
- `internal/engine/register.go` - Registered `browsers.NewWithExecutor(exec)` in `RegisterDefaultCleanersWithExecutor`
- `internal/engine/register_test.go` - Verified 21 registered ecosystems and `browsers` configuration routing
- `internal/engine/engine.go` - Routed `"browsers"` in `ecosystemConfig`
- `cmd/clean.go` - Routed `"browsers"` in `getMaxAgeDays`
- `cmd/clean_test.go` - Updated hermetic test configs to explicitly isolate from unconfigured browser scans
- `cmd/scan_test.go` - Updated hermetic test configs to explicitly isolate from unconfigured browser scans
- `internal/deleter/policy_test.go` - Added `TestCoordinator_DryRun_ZeroMutation_Phase12` canary test

## Decisions Made

- Confirmed `MaxAgeDays: 14` as default TTL for browsers ecosystem in accordance with specification and plan.
- Retained fail-closed refusal: any process in running or uncertain state aborts deletion fail-closed without touching files.
- Maintained absolute inviolability of `~/Library/Safari` and session files (`Cookies`, `History`, `Login Data`, `places.sqlite`, etc.).
- Gated ScriptCache targets with `ReviewOnly: true` and `EligibleBytes: 0`, and additionally guarded `Clean()` against any accidental mutation attempts.

## Deviations from Plan

- `cmd/clean_test.go` and `cmd/scan_test.go` were updated to explicitly include `macos: enabled: false` and `browsers: enabled: false` in test fixture configs (`createEmptyConfig`, `createTestWorkspace`, `setupCleanFixture`, `setupMultiEcoCleanFixture`) to preserve test isolation when running on developer machines with real user browser caches.

## Issues Encountered

None.

## User Setup Required

None - standard Go library and internal primitives only.

## Next Phase Readiness

- Phase 12 (Browsers, Session Exclusions & ScriptCache Gate) is 100% complete across all 3 plans.
- Core browser engine, multi-profile traversal, consolidated presentation, ScriptCache gating, session shields, process guards, engine registration, deduplication, and zero-mutation canary are fully verified with zero CGO.
- Ready for milestone progression or Phase 13.

---
*Phase: 12-browsers-session-exclusions-scriptcache-gate*
*Completed: 2026-10-17*

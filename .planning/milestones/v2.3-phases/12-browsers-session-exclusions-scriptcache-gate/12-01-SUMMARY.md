---
phase: 12-browsers-session-exclusions-scriptcache-gate
plan: 01
subsystem: cleaner/browsers
tags: [browsers, security, session-guard, process-guard, chromium, firefox, safari, fixtures]

# Dependency graph
requires:
  - phase: 06-11
    provides: Unified core cleaner, safety blacklist, and cleaner.CommandExecutor abstractions
provides:
  - Core browser contracts, data types, and declarative definitions for 6 major browsers (BROW-01, D-04)
  - Inviolable Dual-Layer Session Shield rejecting Cookies, History, Sessions, Login Data, and ~/Library/Safari (BROW-03, D-13, D-15)
  - Deterministic Process Guard querying pgrep -x for main executables and active helpers with fail-closed semantics (BROW-01, BROW-04, D-09, D-11, D-12)
  - Hermetic double-sentinel test fixture helpers for macOS and Linux in t.TempDir() without CGO (VAL-01, D-16)
affects: [12-02, 12-03]

# Actuals
actuals:
  tokens: 22000
  tasks: 2
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Inviolable positive allowlist + negative invariant assertion guard rejecting session sentinels
    - Fail-closed process status inspection via cleaner.CommandExecutor without shell interpreters
    - Hermetic test fixture generator with double sentinels (disposable cache vs protected session sentinels)

key-files:
  created:
    - internal/cleaner/browsers/types.go
    - internal/cleaner/browsers/definitions.go
    - internal/cleaner/browsers/session_guard.go
    - internal/cleaner/browsers/session_guard_test.go
    - internal/cleaner/browsers/process.go
    - internal/cleaner/browsers/process_test.go
    - internal/cleaner/browsers/fixtures_test.go
  modified: []

key-decisions:
  - "D-01 & D-04: Declarative definitions for 6 browsers (Chrome, Brave, Arc, Edge, Firefox, Safari) with canonical path resolvers for macOS and Linux, keeping Arc and Safari restricted to Darwin"
  - "D-08: HasScriptCache flag set true exclusively for Chromium-based browsers; false for Gecko and WebKit"
  - "D-09 & D-11: Process Guard inspects both primary binaries and active helpers (e.g. 'Google Chrome Helper', 'firefox-bin') with isolated evaluation per browser"
  - "D-12: Fail-closed pgrep execution: exit code 1 indicates cleanly stopped; exit code 0 indicates running; unexpected exit or execution error returns uncertain=true"
  - "D-13: Inviolable Safari private state shield: ~/Library/Safari is unconditionally rejected; AppSupportDir returns empty string; only ~/Library/Caches/com.apple.Safari allowlisted leaves are accepted"
  - "D-15: Dual-layer session shield: positive allowlist of leaf folders + immutable negative assertion guard rejecting session, password, and history tokens"
  - "D-16: Wave 0 test fixtures populate double sentinels in t.TempDir() with 100% pure Go zero-CGO execution"

patterns-established:
  - "browsers.IsSessionOrProtectedData: Inviolable case-insensitive pattern matching across base names, full paths, and path components"
  - "browsers.VerifyCleanCandidate: Pre-scan and pre-clean safety contract preventing session or blacklisted mutation"
  - "browsers.CheckBrowserProcessStatus: Tokenized pgrep -x invocation via CommandExecutor with fail-closed safety posture"
  - "browsers.setupBrowserTestFixtures: Standardized double-sentinel directory populator for hermetic cleaner testing"

requirements-completed: [BROW-01, BROW-03, BROW-04, VAL-01]

# Coverage metadata
coverage:
  - id: D1
    description: "Inviolable session guard rejecting Cookies, History, Login Data, Sessions, and ~/Library/Safari"
    requirement: "BROW-03"
    verification:
      - kind: unit
        ref: "internal/cleaner/browsers/session_guard_test.go#TestBrowsers_InviolableSessionShield"
        status: pass
      - kind: unit
        ref: "internal/cleaner/browsers/session_guard_test.go#TestBrowsers_SafariIsolation"
        status: pass
    human_judgment: false
  - id: D2
    description: "Deterministic Process Guard querying pgrep -x for main binaries and active helpers with fail-closed semantics"
    requirement: "BROW-01"
    verification:
      - kind: unit
        ref: "internal/cleaner/browsers/process_test.go#TestBrowsers_ProcessGuard_Running"
        status: pass
      - kind: unit
        ref: "internal/cleaner/browsers/process_test.go#TestBrowsers_ProcessGuard_Stopped"
        status: pass
      - kind: unit
        ref: "internal/cleaner/browsers/process_test.go#TestBrowsers_ProcessGuard_FailClosed"
        status: pass
      - kind: unit
        ref: "internal/cleaner/browsers/process_test.go#TestBrowsers_ProcessGuard_NilExecutor"
        status: pass
      - kind: unit
        ref: "internal/cleaner/browsers/process_test.go#TestBrowsers_ProcessGuard_HelperDetection"
        status: pass
      - kind: unit
        ref: "internal/cleaner/browsers/process_test.go#TestBrowsers_ProcessGuard_PerBrowserIsolation"
        status: pass
    human_judgment: false
  - id: D3
    description: "Declarative browser definitions and engine categorization across Darwin and Linux"
    requirement: "BROW-01"
    verification:
      - kind: unit
        ref: "internal/cleaner/browsers/session_guard_test.go#TestBrowsers_Definitions"
        status: pass
    human_judgment: false
  - id: D4
    description: "Hermetic double-sentinel test fixtures helper in t.TempDir() without CGO"
    requirement: "VAL-01"
    verification:
      - kind: unit
        ref: "internal/cleaner/browsers/session_guard_test.go#TestBrowsers_FixturesHelper"
        status: pass
    human_judgment: false

# Metrics
duration: 10 min
completed: 2026-10-17
status: complete
---

# Phase 12 Plan 01: Security Foundation, Session Guard, Process Guard & Test Fixtures Summary

**Core browser definitions, Inviolable Dual-Layer Session Shield, deterministic fail-closed Process Guard, and Wave 0 double-sentinel test fixtures for the new `browsers` cleaner (BROW-01, BROW-03, BROW-04, VAL-01).**

## Performance

- **Duration:** 10 min
- **Started:** 2026-10-17T10:49:27Z
- **Completed:** 2026-10-17T10:59:00Z
- **Tasks:** 2
- **Files modified/created:** 7

## Accomplishments

- Implemented `internal/cleaner/browsers/types.go` and `internal/cleaner/browsers/definitions.go` establishing declarative identification, rendering engine classifications (`chromium`, `gecko`, `webkit`), canonical profile/cache path resolvers, and process mappings for 6 browsers (Google Chrome, Brave, Arc, Microsoft Edge, Mozilla Firefox, Apple Safari) with Arc and Safari restricted to Darwin.
- Implemented `browsers.IsSessionOrProtectedData` and `browsers.VerifyCleanCandidate` in `internal/cleaner/browsers/session_guard.go` with dual-layer security: strict positive allowlists for leaf cache directories plus an immutable negative invariant assertion guard rejecting all sensitive session tokens (`Cookies`, `History`, `Login Data`, `Web Data`, `Local Storage`, `IndexedDB`, `Sessions`, `places.sqlite`, `key4.db`, `logins.json`, `sessionstore`, etc.) and enforcing complete isolation of `~/Library/Safari`.
- Implemented `browsers.CheckBrowserProcessStatus` in `internal/cleaner/browsers/process.go` querying `pgrep -x` via `cleaner.CommandExecutor` without shell interpreters, detecting both primary executables and active helper processes (`Google Chrome Helper`, `firefox-bin`), with strict fail-closed posture on unexpected errors (`uncertain = true`) and per-browser isolation.
- Created Wave 0 hermetic test fixture helpers `setupBrowserTestFixtures` and `setupBrowserLinuxTestFixtures` in `internal/cleaner/browsers/fixtures_test.go` populating double sentinels (disposable cache entries vs protected session files) in `t.TempDir()`.
- Verified 100% test pass with zero CGO across `internal/cleaner/browsers` and the entire repository test suite.

## Task Commits

Each task was committed atomically following strict TDD discipline (RED → GREEN):

1. **Task 12-01-01: Core types, browser definitions, and Inviolable Session Guard with dual-layer protection**
   - RED test: `41ab210` (`test(12-01): add failing unit tests for session guard and test fixtures`)
   - GREEN feat: `25b1764` (`feat(12-01): implement core types, browser definitions, and session guard`)
2. **Task 12-01-02: Deterministic Process Guard with CommandExecutor & fail-closed pgrep semantics**
   - RED test: `f7123da` (`test(12-01): add failing unit tests for browser process guard`)
   - GREEN feat: `dc0641d` (`feat(12-01): implement deterministic process guard with fail-closed semantics`)

## Files Created/Modified

- `internal/cleaner/browsers/types.go` - `BrowserEngine`, `BrowserDef`, `ProfileInfo`, and `SubVolumeMetrics` type contracts
- `internal/cleaner/browsers/definitions.go` - Declarative configurations for 6 browsers across macOS and Linux
- `internal/cleaner/browsers/session_guard.go` - Inviolable session and Safari invariant assertion guard
- `internal/cleaner/browsers/session_guard_test.go` - Unit tests verifying rejection of session tokens and acceptance of cache leaves
- `internal/cleaner/browsers/process.go` - Fail-closed per-browser process monitor querying tokenized `pgrep -x`
- `internal/cleaner/browsers/process_test.go` - Hermetic unit tests with `cleaner.MockCommandExecutor` for running, stopped, and uncertain process states
- `internal/cleaner/browsers/fixtures_test.go` - Wave 0 double-sentinel test fixture helpers for macOS and Linux

## Decisions Made

- Enforced `Safari` `AppSupportDir` returning `""` unconditionally to prevent any accidental inspection or scanning of `~/Library/Application Support/Safari` or `~/Library/Safari`.
- Set `HasScriptCache` to `true` exclusively for the 4 Chromium browsers (`chrome`, `brave`, `arc`, `edge`), and `false` for `firefox` and `safari`.
- Isolated process monitoring so an active browser does not block clean checks for inactive browsers.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - standard Go library and internal primitives only.

## Next Plan Readiness

- Security foundation, process guard, session guard, and test fixtures complete and verified with pure Go zero-CGO tests.
- Ready for Plan 12-02: Multi-browser discovery, multi-profile traversal, consolidated target presentation, and ScriptCache gating.

---
*Phase: 12-browsers-session-exclusions-scriptcache-gate*
*Completed: 2026-10-17*

---
phase: 12-browsers-session-exclusions-scriptcache-gate
plan: 02
subsystem: cleaner/browsers
tags: [browsers, chromium, firefox, safari, profiles, scriptcache, tcc, security]

# Dependency graph
requires:
  - phase: 12-01
    provides: Browser definitions, session guard, process guard, test fixtures
provides:
  - Multi-profile traversal & cache scanner for Chromium browsers (Chrome, Brave, Arc, Edge) (BROW-01, BROW-02, D-01, D-02, D-03)
  - Strict ScriptCache gating into ReviewOnly targets (BROW-04, D-05, D-06)
  - Multi-profile traversal & cache scanner for Mozilla Firefox with SQLite preservation (BROW-01, BROW-02, BROW-03, D-03, D-08, D-15)
  - WebKit scanner for Apple Safari with Darwin-only restriction, ~/Library/Safari isolation, and TCC graceful degradation (BROW-01, BROW-02, BROW-03, D-04, D-08, D-13, D-14)
affects: [12-03]

# Actuals
actuals:
  tokens: 24000
  tasks: 2
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Multi-profile dynamic discovery across AppSupport/Config and OS Cache roots
    - Consolidated virtual targets (browser://<id>/caches) aggregating network, GPU, code, and CacheStorage caches
    - Inviolable ScriptCache gating with ReviewOnly: true, ReviewReason: scriptcache_requires_evidence, EligibleBytes: 0, TTLEligible: false
    - Graceful degradation under TCC permission denials with ProviderMetrics.Partial = true and zero sudo
    - Surgical unlinking of allowlisted leaf directories with pre-clean VerifyCleanCandidate and symlink defense

key-files:
  created:
    - internal/cleaner/browsers/chromium.go
    - internal/cleaner/browsers/chromium_test.go
    - internal/cleaner/browsers/firefox.go
    - internal/cleaner/browsers/firefox_test.go
    - internal/cleaner/browsers/safari.go
    - internal/cleaner/browsers/safari_test.go
  modified: []

key-decisions:
  - "D-01 & D-02: Consolidated single cache target per browser (browser://<id>/caches) aggregating network, GPU, code cache, and Service Worker CacheStorage"
  - "D-03: Dynamic traversal of all profiles (Default, Profile *, System Profile, Guest Profile, custom dirs with Preferences, and Firefox *.default* / *.release* / prefs.js)"
  - "D-04: Platform restrictions strictly enforced: Arc and Safari operate exclusively on Darwin; Chrome, Brave, Edge, and Firefox operate across Darwin and Linux"
  - "D-05 & D-06: Chromium ScriptCache strictly segregated into dedicated ReviewOnly target (browser://<id>/scriptcache) with EligibleBytes: 0, TTLEligible: false, and ReviewReason: scriptcache_requires_evidence"
  - "D-08: Zero ScriptCache targets emitted for Firefox or Safari"
  - "D-13: Absolute isolation of ~/Library/Safari; Safari scanner operates strictly within ~/Library/Caches/com.apple.Safari"
  - "D-14: Graceful degradation on TCC/permission denials (os.IsPermission or EACCES/EPERM) recording ProviderMetrics.Partial = true, ReviewOnly: true, ReviewReasonSystemProtected with zero sudo"
  - "D-15: Surgical unlinking of allowlisted cache leaves only, verified with VerifyCleanCandidate and safety.IsSymlink, preserving 100% of SQLite databases, preferences, and session files"

patterns-established:
  - "browsers.discoverChromiumProfiles: Scans AppSupport and OS Cache roots with seen map for Default, Profile *, System Profile, Guest Profile, and Preferences"
  - "browsers.scanChromiumBrowser: Consolidates allowlisted leaves across profiles into browser://<id>/caches and segregates ScriptCache into browser://<id>/scriptcache"
  - "browsers.cleanChromiumBrowser: Surgically unlinks allowlisted leaves across profiles while strictly leaving ScriptCache and session files untouched"
  - "browsers.discoverFirefoxProfiles: Matches *.default*, *.release*, and prefs.js across profiles roots"
  - "browsers.scanFirefox: Aggregates cache2, startupCache, jumpListCache, and thumbnails into browser://firefox/caches with zero ScriptCache"
  - "browsers.cleanFirefox: Unlinks allowlisted cache leaves while 100% preserving SQLite databases (places.sqlite, key4.db, logins.json)"
  - "browsers.scanSafari: Traverses allowlisted items in ~/Library/Caches/com.apple.Safari on Darwin with TCC graceful degradation and zero sudo"
  - "browsers.cleanSafari: Unlinks allowlisted items under ~/Library/Caches/com.apple.Safari, leaving ~/Library/Safari completely untouched"
  - "browsers.hashBrowserPath: SHA-256 derived 16-hex deterministic target ID"

requirements-completed: [BROW-01, BROW-02, BROW-03, BROW-04, SCAN-02]

# Coverage metadata
coverage:
  - id: D1
    description: "Multi-profile traversal and consolidated target presentation for Chromium browsers"
    requirement: "BROW-01, BROW-02"
    verification:
      - kind: unit
        ref: "internal/cleaner/browsers/chromium_test.go#TestBrowsers_MultiProfileTraversal"
        status: pass
      - kind: unit
        ref: "internal/cleaner/browsers/chromium_test.go#TestBrowsers_OnDiskDetection"
        status: pass
    human_judgment: false
  - id: D2
    description: "Dedicated ReviewOnly ScriptCache target gating for Chromium browsers"
    requirement: "BROW-04"
    verification:
      - kind: unit
        ref: "internal/cleaner/browsers/chromium_test.go#TestBrowsers_Chromium_ScriptCacheGated"
        status: pass
    human_judgment: false
  - id: D3
    description: "Firefox multi-profile traversal, cache consolidation, SQLite preservation, and zero ScriptCache"
    requirement: "BROW-01, BROW-02, BROW-03, BROW-04"
    verification:
      - kind: unit
        ref: "internal/cleaner/browsers/firefox_test.go#TestBrowsers_Firefox_ProfileDiscovery"
        status: pass
      - kind: unit
        ref: "internal/cleaner/browsers/firefox_test.go#TestBrowsers_Firefox_Scan"
        status: pass
      - kind: unit
        ref: "internal/cleaner/browsers/firefox_test.go#TestBrowsers_Firefox_CleanPreservesSession"
        status: pass
    human_judgment: false
  - id: D4
    description: "Safari Darwin-only cache scanning, ~/Library/Safari isolation, TCC graceful degradation, and zero ScriptCache"
    requirement: "BROW-01, BROW-02, BROW-03, BROW-04"
    verification:
      - kind: unit
        ref: "internal/cleaner/browsers/safari_test.go#TestBrowsers_Safari_Isolation"
        status: pass
      - kind: unit
        ref: "internal/cleaner/browsers/safari_test.go#TestBrowsers_Safari_TCCGracefulDegradation"
        status: pass
      - kind: unit
        ref: "internal/cleaner/browsers/safari_test.go#TestBrowsers_Safari_ZeroScriptCache"
        status: pass
      - kind: unit
        ref: "internal/cleaner/browsers/safari_test.go#TestBrowsers_ConsolidatedTargets"
        status: pass
    human_judgment: false

# Metrics
duration: 15 min
completed: 2026-10-17
status: complete
---

# Phase 12 Plan 02: Multi-Browser Discovery, Multi-Profile Traversal & Cache Scanning Summary

**Multi-browser discovery, multi-profile traversal, consolidated target presentation, ScriptCache review gating, and TCC graceful degradation across 6 browsers (BROW-01, BROW-02, BROW-03, BROW-04, SCAN-02).**

## Performance

- **Duration:** 15 min
- **Started:** 2026-10-17T11:00:24Z
- **Completed:** 2026-10-17T11:11:00Z
- **Tasks:** 2
- **Files modified/created:** 6

## Accomplishments

- Implemented `internal/cleaner/browsers/chromium.go` providing dynamic multi-profile traversal (`Default`, `Profile *`, `System Profile`, `Guest Profile`, and directories with `Preferences`), allowlisted leaf scanning (`GPUCache`, `DawnCache`, `Code Cache`, `CacheStorage`, `Cache_Data`, `Media Cache`), consolidated target emission (`browser://<id>/caches`), dedicated ReviewOnly ScriptCache gating (`browser://<id>/scriptcache`), and surgical cleaning without touching ScriptCache or session files.
- Implemented `internal/cleaner/browsers/firefox.go` providing Gecko profile discovery (`*.default*`, `*.release*`, and `prefs.js`), cache consolidation across profiles (`cache2`, `startupCache`, `jumpListCache`, `thumbnails`), zero ScriptCache emission, and surgical cleaning preserving 100% of SQLite databases (`places.sqlite`, `key4.db`, `logins.json`, `cookies.sqlite`).
- Implemented `internal/cleaner/browsers/safari.go` providing strict WebKit scanning on Darwin in `~/Library/Caches/com.apple.Safari`, complete isolation of `~/Library/Safari`, zero ScriptCache emission, and graceful degradation on TCC/permission denials without sudo (`ProviderMetrics.Partial = true`, `StatusNote: "permission denied (TCC / Full Disk Access required)"`, `ReviewReasonSystemProtected`).
- Wrote extensive unit tests in `chromium_test.go`, `firefox_test.go`, and `safari_test.go` verifying 100% test pass with zero CGO across the entire repository test suite (`CGO_ENABLED=0 go test ./...`).

## Task Commits

Each task was committed atomically following strict TDD discipline (RED → GREEN):

1. **Task 12-02-01: Chromium scanner (Chrome, Brave, Arc, Edge) with multi-profile traversal and ScriptCache gating**
   - RED test: `4bc6f78` (`test(12-02): add failing unit tests for Chromium scanner and ScriptCache gating`)
   - GREEN feat: `1b3cd10` (`feat(12-02): implement Chromium multi-profile scanner and ScriptCache gating`)
2. **Task 12-02-02: Firefox & Safari scanners with Darwin restrictions, TCC degradation, and consolidated targets**
   - RED test: `2daadaa` (`test(12-02): add failing unit tests for Firefox and Safari scanners`)
   - GREEN feat: `e9fb083` (`feat(12-02): implement Firefox and Safari scanners with TCC graceful degradation`)

## Files Created/Modified

- `internal/cleaner/browsers/chromium.go` - `discoverChromiumProfiles`, `scanChromiumBrowser`, `cleanChromiumBrowser`, and `hashBrowserPath`
- `internal/cleaner/browsers/chromium_test.go` - Unit tests for on-disk discovery, profile traversal, ScriptCache gating, process blocking, and symlink protection
- `internal/cleaner/browsers/firefox.go` - `discoverFirefoxProfiles`, `scanFirefox`, and `cleanFirefox`
- `internal/cleaner/browsers/firefox_test.go` - Unit tests for Gecko profile discovery, cache consolidation, zero ScriptCache, and SQLite database preservation
- `internal/cleaner/browsers/safari.go` - `scanSafari`, `cleanSafari`, and `isSyscallPermission`
- `internal/cleaner/browsers/safari_test.go` - Unit tests for Darwin cache scanning, `~/Library/Safari` shield, TCC degradation, and consolidated target verification

## Decisions Made

- Confirmed virtual addressing `browser://<id>/caches` for main cache targets and `browser://<id>/scriptcache` for ScriptCache targets.
- Enforced `EligibleBytes: 0`, `TTLEligible: false`, and `ReviewReason: "scriptcache_requires_evidence"` unconditionally for all Chromium ScriptCache targets.
- Firefox and Safari emit zero ScriptCache targets.
- Preserved `~/Library/Safari` with absolute zero file operations or inspection.
- Handled TCC permission denials without panicking, erroring, or attempting privilege elevation via `sudo`.

## Deviations from Plan

None - plan executed strictly as specified with atomic TDD commits.

## Issues Encountered

None.

## User Setup Required

None - standard Go library and internal primitives only.

## Next Plan Readiness

- Scanning, discovery, and profile engines for all 6 browsers complete and verified.
- Ready for Plan 12-03: Package cleaner lifecycle orchestration (`browsers.go`), engine registration, CLI/config integration, macOS user cache deduplication, and zero-mutation dry-run canary.

---
*Phase: 12-browsers-session-exclusions-scriptcache-gate*
*Completed: 2026-10-17*

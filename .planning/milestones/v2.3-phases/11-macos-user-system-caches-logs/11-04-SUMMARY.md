---
phase: 11-macos-user-system-caches-logs
plan: 04
subsystem: cleaner/macos
tags: [macos, system-logs, diagnostic-reports, code-sign-clones, engine-registration, zero-mutation, tcc-fallback, trash-purge]

# Dependency graph
requires:
  - phase: 11-02
    provides: User app caches, logs, and Finder trash scanner
  - phase: 11-03
    provides: Canonical Apple daemons, group containers, and surgical application support leaves
provides:
  - System logs and diagnostic reports scanner with graceful TCC degradation and zero sudo (MAC-04, D-09, D-10, D-12)
  - Darwin code_sign_clone user bucket isolation with Chrome process guard (MAC-04, D-11)
  - macosCleaner implementing domain.Cleaner with pre-deletion process revalidation and ownership checks (MAC-05, D-14, D-16)
  - Engine registration of macos cleaner with CommandExecutor and config defaults with MaxAgeDays: 14 (MAC-01..05)
  - CLI --trash flag coordination and deleter direct trash purging bypassing internal quarantine (MAC-01, D-01, D-03)
  - Zero-mutation canary integration test proving 100% byte integrity under DryRun (POL-04, VAL-02)
affects: [12-browsers, 13-system]

# Actuals
actuals:
  tokens: 24500
  tasks: 3
  commits: 6

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Graceful TCC permission degradation via ProviderMetrics.Partial and StatusNote
    - User bucket isolation for Darwin code-sign clones via TMPDIR/C/
    - Multi-layer pre-deletion process guard revalidation, symlink guard, and UID ownership verification
    - Deleter direct cleaner routing for macos-trash bypassing internal quarantine
    - SHA-256 and mtime byte-level canary integrity verification in integration test

key-files:
  created:
    - internal/cleaner/macos/system.go
    - internal/cleaner/macos/system_test.go
    - internal/cleaner/macos/macos.go
    - internal/cleaner/macos/macos_test.go
  modified:
    - internal/config/config.go
    - internal/config/defaults.go
    - internal/engine/register.go
    - internal/engine/register_test.go
    - internal/engine/engine.go
    - internal/engine/engine_test.go
    - cmd/clean.go
    - internal/deleter/deleter.go
    - internal/deleter/policy_test.go

key-decisions:
  - "D-01: CLI --trash coordinates with candidate filtering, enabling macos-trash deletion with ReviewOnly: false and TTLEligible: true"
  - "D-03: deleter partitions Category == 'macos-trash' into direct cleaner execution, permanently purging items without moving to internal .trash quarantine"
  - "D-09: scanDirectoryWithTCCFallback captures permission errors, reporting ProviderMetrics.Partial = true and StatusNote: 'permission denied (TCC / root required)' with readable bytes accumulated"
  - "D-10: DevClean operates with zero sudo and zero root elevation; system-protected logs remain ReviewOnly"
  - "D-11: Chromium code signature clones (*.code_sign_clone) are searched strictly within current user's Darwin cache directory (TMPDIR/C/)"
  - "D-12: Diagnostic reports are segregated into User Diagnostic Reports (eligible) and System Diagnostic Reports (ReviewOnly)"
  - "D-14: macosCleaner.Clean() revalidates bundle process status immediately before deletion, aborting if process started post-scan"

patterns-established:
  - "macos.scanDirectoryWithTCCFallback: WalkDir trapping permission denials and populating ProviderMetrics without failing silently"
  - "macos.scanCodeSignClones: User-isolated Darwin cache search protected by browser process guard"
  - "macosCleaner.Clean: Multi-layered pre-deletion defense (Symlink Guard, UID ownership, whitelist, Process Guard)"
  - "deleter Stage 2 directTargets: Specialized direct cleaner execution for pre-trashed content bypassing quarantine"

requirements-completed: [MAC-01, MAC-02, MAC-03, MAC-04, MAC-05]

# Coverage metadata
coverage:
  - id: D1
    description: "Segregation of User Diagnostic Reports (~/Library/Logs/DiagnosticReports, eligible) and System Diagnostic Reports (/Library/Logs/DiagnosticReports, ReviewOnly)"
    requirement: "MAC-04"
    verification:
      - kind: unit
        ref: "internal/cleaner/macos/system_test.go#TestDiagnosticReports_UserEligible_SystemReviewOnly"
        status: pass
    human_judgment: false
  - id: D2
    description: "System logs (/private/var/log) and system reports graceful TCC degradation via ProviderMetrics.Partial = true and StatusNote with zero sudo"
    requirement: "MAC-04"
    verification:
      - kind: unit
        ref: "internal/cleaner/macos/system_test.go#TestSystemLogs_GracefulDegradationTCC"
        status: pass
    human_judgment: false
  - id: D3
    description: "Chromium code signature clones (*.code_sign_clone) searched exclusively within current user's Darwin cache directory (TMPDIR/C/) with Chrome/Chromium process guard"
    requirement: "MAC-04"
    verification:
      - kind: unit
        ref: "internal/cleaner/macos/macos_test.go#TestCodeSignClones_IsolatedToUserTmpdir"
        status: pass
    human_judgment: false
  - id: D4
    description: "macosCleaner orchestrating all sub-scanners and implementing Clean() with pre-deletion process revalidation, Symlink Guard, and UID ownership check"
    requirement: "MAC-05"
    verification:
      - kind: unit
        ref: "internal/cleaner/macos/macos_test.go#TestMacOSCleaner_Lifecycle"
        status: pass
      - kind: unit
        ref: "internal/cleaner/macos/macos_test.go#TestMacOSCleaner_PreDeleteProcessRevalidation"
        status: pass
      - kind: unit
        ref: "internal/cleaner/macos/macos_test.go#TestMacOSCleaner_CleanTrashDirect"
        status: pass
    human_judgment: false
  - id: D5
    description: "Engine registration of macos cleaner with CommandExecutor and config defaults with MaxAgeDays: 14"
    requirement: "MAC-01"
    verification:
      - kind: unit
        ref: "internal/engine/register_test.go#TestRegisterDefaultCleaners"
        status: pass
      - kind: unit
        ref: "internal/engine/register_test.go#TestRegisterDefaultCleanersWithExecutor"
        status: pass
    human_judgment: false
  - id: D6
    description: "CLI --trash coordination and deleter direct cleaner execution bypassing internal quarantine"
    requirement: "MAC-01"
    verification:
      - kind: unit
        ref: "cmd/clean_test.go#TestCleanCmd_NewEcosystemsValidation"
        status: pass
      - kind: unit
        ref: "internal/deleter/policy_test.go#TestCoordinator_DirectTrashPurge_BypassesQuarantine"
        status: pass
    human_judgment: false
  - id: D7
    description: "Zero-mutation canary test proving 100% byte integrity (SHA-256, size, mtime) on trash items, system logs, diagnostic reports, and vaults under DryRun"
    requirement: "MAC-05"
    verification:
      - kind: integration
        ref: "internal/deleter/policy_test.go#TestCoordinator_DryRun_ZeroMutation_Phase11"
        status: pass
    human_judgment: false

# Metrics
duration: 25 min
completed: 2026-10-16
status: complete
---

# Phase 11 Plan 04: System Logs, Code-Sign Clones, Engine Registration & Canary Safety Summary

**Graceful TCC degradation in system logs and diagnostic reports with zero sudo, user-bucket Darwin code-sign clones, macosCleaner orchestrator with pre-delete process revalidation, engine registration, direct trash purge routing, and zero-mutation canary safety (MAC-01..05, D-01, D-03, D-09..D-12, D-14, D-16).**

## Performance

- **Duration:** 25 min
- **Started:** 2026-10-16T20:47:12Z
- **Completed:** 2026-10-16T21:11:00Z
- **Tasks:** 3
- **Files modified:** 13

## Accomplishments

- Implemented `macos.scanSystemLogsAndReports` and `macos.scanDirectoryWithTCCFallback` segregating User Diagnostic Reports (`~/Library/Logs/DiagnosticReports`, eligible for cleanup) from System Diagnostic Reports (`/Library/Logs/DiagnosticReports`, marked `ReviewOnly: true` with `ReviewReasonSystemProtected`).
- Implemented graceful TCC and permission degradation on `/private/var/log` and system diagnostic directories, setting `ProviderMetrics.Partial = true` and `StatusNote: "permission denied (TCC / root required)"`, preserving readable bytes without failing silently or prompting for root. DevClean executes with strictly zero `sudo` or privilege elevation.
- Implemented `macos.scanCodeSignClones` locating Chromium code-signature clones (`*.code_sign_clone`) exclusively within the current user's Darwin cache directory (`$TMPDIR/../C/`), protected by the active Chrome/Chromium process guard.
- Implemented `macosCleaner` implementing `domain.Cleaner`, orchestrating all 6 sub-scanners (user caches, user logs, Finder trash, Apple daemons, group containers, Application Support, and system logs/reports).
- Implemented multi-layered defense in `macosCleaner.Clean()`: automatically skipping `ReviewOnly` targets, delegating `macos-trash` to `cleanTrash()` for permanent direct purging without internal quarantine, enforcing Symlink Guard and UID ownership checks, and performing mandatory pre-deletion process revalidation immediately before file removal to abort deletion if an application was started post-scan.
- Registered the `macos` cleaner in `RegisterDefaultCleanersWithExecutor` using `cleaner.CommandExecutor`, expanding DevClean to 20 supported ecosystems.
- Configured default settings for macOS ecosystem with `MaxAgeDays: 14` and hook timeout `30s` in `internal/config/config.go` and `internal/config/defaults.go`.
- Integrated CLI `--trash` flag coordination in `cmd/clean.go`, making `macos-trash` targets authorized clean candidates (`ReviewOnly = false`, `EligibleBytes = SizeBytes`, `TTLEligible = true`).
- Partitioned `macos-trash` in `internal/deleter/deleter.go` Stage 2 into direct cleaner execution (`directTargets`), bypassing `c.trashMgr.Move()` to prevent redundant re-quarantining of already-trashed items.
- Added comprehensive zero-mutation canary regression test `TestCoordinator_DryRun_ZeroMutation_Phase11` in `internal/deleter/policy_test.go`, asserting 100% byte integrity (SHA-256 hash, size, and modtime) across trash items, system logs, crash reports, Keychain vaults, and 1Password vaults under DryRun.
- Verified 100% test pass rate across all packages with zero CGO compilation (`CGO_ENABLED=0`) on Darwin and Linux.

## Task Commits

Each task was committed atomically following strict TDD discipline (RED → GREEN):

1. **Task 11-04-01: System logs and DiagnosticReports segregation with graceful TCC degradation and zero sudo**
   - RED test: `306b32e` (`test(11-04): add failing unit tests for system logs, diagnostic reports, and TCC fallback`)
   - GREEN feat: `8a8b7be` (`feat(11-04): implement system logs and diagnostic reports with graceful TCC fallback`)
2. **Task 11-04-02: Darwin code_sign_clone user bucket isolation and macOS cleaner orchestration**
   - RED test: `aad864d` (`test(11-04): add failing unit tests for code sign clones and macos cleaner lifecycle`)
   - GREEN feat: `03e02c8` (`feat(11-04): implement code sign clones scanner and macos cleaner orchestrator`)
3. **Task 11-04-03: Engine registration, Config defaults, Deleter direct trash routing, CLI --trash integration, and zero-mutation canary test**
   - RED test: `33ee0c2` (`test(11-04): add failing tests for engine registration, direct trash purge, and zero-mutation canary`)
   - GREEN feat: `e21a811` (`feat(11-04): register macos cleaner, direct trash purge routing, and --trash flag coordination`)

## Files Created/Modified

- `internal/cleaner/macos/system.go` - System logs, diagnostic reports with TCC graceful fallback, and Darwin code-sign clones scanner
- `internal/cleaner/macos/system_test.go` - Unit tests for diagnostic report segregation and graceful TCC permission degradation
- `internal/cleaner/macos/macos.go` - macosCleaner orchestrating sub-scanners and implementing Clean() with pre-deletion guards
- `internal/cleaner/macos/macos_test.go` - Lifecycle, code-sign clones, pre-delete process revalidation, and direct trash cleaning tests
- `internal/config/config.go` - Added MacOS EcosystemConfig and HookConfig
- `internal/config/defaults.go` - Added default MacOS ecosystem configuration with MaxAgeDays: 14
- `internal/engine/register.go` - Registered macos cleaner in RegisterDefaultCleanersWithExecutor
- `internal/engine/register_test.go` - Updated tests to verify 20 ecosystems with macos
- `internal/engine/engine.go` - Mapped macos ecosystem in ecosystemConfig switch
- `internal/engine/engine_test.go` - Included MacOS in test default configuration
- `cmd/clean.go` - Coordinated --trash flag with macos-trash eligibility and getMaxAgeDays
- `internal/deleter/deleter.go` - Partitioned macos-trash into directTargets bypassing internal quarantine
- `internal/deleter/policy_test.go` - Added TestCoordinator_DryRun_ZeroMutation_Phase11 canary test and TestCoordinator_DirectTrashPurge_BypassesQuarantine

## Decisions Made

- `scanDirectoryWithTCCFallback` uses `fs.SkipDir` when encountering permission denials on directory roots, accumulating readable entries while reporting `ProviderMetrics.Partial = true` and `StatusNote: "permission denied (TCC / root required)"`.
- Chromium code signature clones are isolated strictly to `$TMPDIR/../C/` matching only `*.code_sign_clone` files, preventing any leakage into other users' temporary directory spaces.
- In `deleter.Coordinator`, targets with `Category == "macos-trash"` are partitioned into `directTargets` rather than `fsTargets`, ensuring that authorizing `--trash` permanently purges trash items directly via `cleanTrash()` instead of redundantly moving already-trashed files into DevClean's `.trash` quarantine.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug Fix] Engine ecosystemConfig missing macos mapping**
- **Found during:** Task 11-04-03 verification
- **Issue:** `internal/engine/engine.go:ecosystemConfig` did not have `case "macos"`, which would cause `ecosystemConfig("macos")` to fall back to default `{Enabled: true, FailMode: "continue", MaxAgeDays: 0}` rather than using `cfg.Ecosystems.MacOS.MaxAgeDays` (14).
- **Fix:** Added `case "macos": return e.config.Ecosystems.MacOS` to `internal/engine/engine.go` and added `MacOS` to `engine_test.go:defaultConfig()`.
- **Files modified:** `internal/engine/engine.go`, `internal/engine/engine_test.go`
- **Verification:** `go test -v ./internal/engine` passes 100%.
- **Committed in:** `e21a811` (Task 11-04-03 GREEN commit).

---

**Total deviations:** 1 auto-fixed (cleaner engine ecosystem mapping).
**Impact on plan:** Essential for correct engine TTL evaluation for macOS ecosystem. No scope creep.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 11 (macOS User/System Caches & Logs) is 100% complete across all 4 plans (11-01, 11-02, 11-03, 11-04).
- All 5 requirements (`MAC-01`, `MAC-02`, `MAC-03`, `MAC-04`, `MAC-05`) are fulfilled and backed by hermetic unit tests, zero-mutation canary tests, and zero-CGO static compilation.
- Ready for Phase 12 (Browser Cleaners: Chrome, Safari, Firefox, Edge, Brave, Arc, Opera, Vivaldi).

---
*Phase: 11-macos-user-system-caches-logs*
*Completed: 2026-10-16*

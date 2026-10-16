---
phase: 11-macos-user-system-caches-logs
plan: 02
subsystem: cleaner/macos
tags: [macos, user-caches, user-logs, trash, deduplication, consolidation]

# Dependency graph
requires:
  - phase: 11-01
    provides: Immutable security whitelist, fail-closed process guard, symlink guard, ownership verification
provides:
  - User app caches and logs scanner with canonical deduplication matrix and 50MB bundle consolidation (MAC-01, D-05, D-06)
  - Finder Trash scanner defaulting to ReviewOnly: true, EligibleBytes: 0, and tolerant direct purge preserving root directory (MAC-01, D-01, D-02, D-03, D-04)
affects: [11-03, 11-04]

# Actuals
actuals:
  tokens: 22000
  tasks: 2
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Canonical deduplication exclusion matrix against dedicated ecosystem cleaners (pnpm, uv, mise, Homebrew, AI editors)
    - 50MB bundle consolidation threshold for user app caches and logs
    - Invariant-driven ReviewOnly discovery for ~/.Trash requiring explicit --trash authorization
    - Tolerant chmod u+w recovery on locked trash items with root directory preservation

key-files:
  created:
    - internal/cleaner/macos/user.go
    - internal/cleaner/macos/user_test.go
    - internal/cleaner/macos/trash.go
    - internal/cleaner/macos/trash_test.go
  modified: []

key-decisions:
  - "D-01: ~/.Trash is always inventoried as ReviewOnly: true, ReviewReason: 'trash_requires_flag', EligibleBytes: 0, and TTLEligible: false on scan"
  - "D-02: Trash discovery strictly scans $HOME/.Trash, ignoring /Volumes/*/.Trashes"
  - "D-03: cleanTrash removes items inside ~/.Trash permanently via os.RemoveAll directly without internal quarantine, preserving root ~/.Trash"
  - "D-04: cleanTrash applies fs.ChmodRecursive (0755, 0644) to unlock read-only files before deletion, reporting remaining bytes on error"
  - "D-05: scanUserCaches excludes folders owned by dedicated ecosystem cleaners (pnpm, uv, Homebrew, CocoaPods, vcpkg, mise, gh, ms-playwright, Cypress, node, Yarn, OrbStack, AI editors)"
  - "D-06: Secondary bundles <= 50MB are consolidated into category targets; bundles > 50MB or with active processes are surfaced individually"

patterns-established:
  - "macos.scanUserCaches: Hierarchical filter applying deduplication exclusions, whitelist checks, symlink safety, process guard, and 50MB consolidation"
  - "macos.scanUserLogs: Category consolidation of user logs while segregating DiagnosticReports for system scanner"
  - "macos.scanTrash & cleanTrash: Honest trash safety invariant with direct purging and root directory preservation"

requirements-completed: [MAC-01]

# Coverage metadata
coverage:
  - id: D1
    description: "User app caches scanner with canonical deduplication matrix skipping dedicated ecosystem cleaners and AI editors"
    requirement: "MAC-01"
    verification:
      - kind: unit
        ref: "internal/cleaner/macos/user_test.go#TestUserCaches_DeduplicationExclusions"
        status: pass
    human_judgment: false
  - id: D2
    description: "50MB bundle consolidation for user caches and user logs breaking out large or active process bundles"
    requirement: "MAC-01"
    verification:
      - kind: unit
        ref: "internal/cleaner/macos/user_test.go#TestUserCaches_ConsolidationThreshold"
        status: pass
      - kind: unit
        ref: "internal/cleaner/macos/user_test.go#TestUserCaches_ActiveProcessSurfacedIndividually"
        status: pass
      - kind: unit
        ref: "internal/cleaner/macos/user_test.go#TestUserLogs_Consolidation"
        status: pass
    human_judgment: false
  - id: D3
    description: "User Trash discovery enforcing ReviewOnly: true, EligibleBytes: 0, TTLEligible: false, and ReviewReason: 'trash_requires_flag' by default"
    requirement: "MAC-01"
    verification:
      - kind: unit
        ref: "internal/cleaner/macos/trash_test.go#TestTrash_ScanAlwaysReviewOnly"
        status: pass
      - kind: unit
        ref: "internal/cleaner/macos/trash_test.go#TestTrash_IgnoresExternalVolumes"
        status: pass
    human_judgment: false
  - id: D4
    description: "Tolerant direct purge of ~/.Trash items with chmod u+w recovery on locked items and strict root preservation"
    requirement: "MAC-01"
    verification:
      - kind: unit
        ref: "internal/cleaner/macos/trash_test.go#TestTrash_CleanPurgesItemsPreservingRoot"
        status: pass
      - kind: unit
        ref: "internal/cleaner/macos/trash_test.go#TestTrash_ChmodRecoveryOnLockedFiles"
        status: pass
    human_judgment: false

# Metrics
duration: 15 min
completed: 2026-10-16
status: complete
---

# Phase 11 Plan 02: User Essentials & Honest Trash Policy Summary

**User caches and logs scanner with canonical deduplication exclusions, 50MB bundle consolidation, and honest Trash discovery ($HOME/.Trash ReviewOnly: true default) with tolerant direct purge preserving the root directory (MAC-01, D-01 to D-06).**

## Performance

- **Duration:** 15 min
- **Started:** 2026-10-16T20:21:01Z
- **Completed:** 2026-10-16T20:30:00Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- Implemented `macos.scanUserCaches` and `macos.isExcludedByDedicatedCleaner` with an exclusion table preventing double byte accounting against `pnpm`, `uv`, `pip`, `Homebrew`, `CocoaPods`, `vcpkg`, `mise`, `gh`, `ms-playwright`, `Cypress`, `node`, `Yarn`, `OrbStack`, and AI/Electron editors (`VSCode`, `Cursor`, `Antigravity`, `Windsurf`, `Claude`, `opencode`, `Zed`).
- Implemented 50MB bundle consolidation for `~/Library/Caches` and `~/Library/Logs`, surfacing bundles <= 50MB under clean category entries (`macOS User App Caches (consolidated)` and `macOS User App Logs (consolidated)`) while cleanly breaking out items > 50MB or active applications into individual targets with full process guard awareness.
- Segregated `DiagnosticReports` from `scanUserLogs` for specialized system handling in Plan 11-04.
- Implemented `macos.scanTrash` strictly inspecting `$HOME/.Trash` (never touching external volumes), reporting it on scan as `ReviewOnly: true`, `ReviewReason: "trash_requires_flag"`, `EligibleBytes: 0`, and `TTLEligible: false`.
- Implemented `macos.cleanTrash` performing permanent deletion via `os.RemoveAll` without routing to internal quarantine, applying tolerant `fs.ChmodRecursive` (0755, 0644) recovery on locked items (0444), reporting remaining bytes on persistent failure, and strictly preserving the root directory `~/.Trash`.
- Maintained 86.3% test coverage in `internal/cleaner/macos` with 100% passing hermetic unit tests under both native build and `CGO_ENABLED=0`.

## Task Commits

Each task was committed atomically following strict TDD discipline (RED → GREEN):

1. **Task 11-02-01: User caches and logs with canonical deduplication exclusions and 50MB bundle consolidation**
   - RED test: `cfe7253` (`test(11-02): add failing unit tests for user caches and logs`)
   - GREEN feat: `6887467` (`feat(11-02): implement user caches and logs scanner with deduplication and consolidation`)
2. **Task 11-02-02: User Trash discovery ($HOME/.Trash ReviewOnly: true default) and tolerant direct purge**
   - RED test: `a2603d0` (`test(11-02): add failing unit tests for trash discovery and direct purge`)
   - GREEN feat: `46728c3` (`feat(11-02): implement user trash discovery and tolerant direct purge`)

## Files Created/Modified

- `internal/cleaner/macos/user.go` - User app caches and logs scanner with deduplication matrix, symlink guard, and 50MB consolidation
- `internal/cleaner/macos/user_test.go` - Hermetic tests for deduplication table, consolidation threshold, active process breakout, and whitelisting
- `internal/cleaner/macos/trash.go` - Finder Trash discovery (ReviewOnly default) and direct purge with chmod recovery and root preservation
- `internal/cleaner/macos/trash_test.go` - Hermetic tests for trash invariants, direct purge, chmod recovery on 0444 files, and external volume exclusion

## Decisions Made

- Bundles with active processes are broken out into individual targets regardless of whether their allocated size is <= 50MB, ensuring that active process warnings and `ReviewReasonActiveProcess` are clearly visible to the user rather than hidden inside the consolidated bucket.
- `cleanTrash` applies `fs.ChmodRecursive` to every item before removal, allowing seamless purge of read-only trees while gracefully catching any undeletable items and reporting unremoved bytes without aborting the loop.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- User essentials and Trash policy modules implemented and fully verified.
- Ready for Plan 11-03: Positive Allowlists for Canonical Daemons (`daemons.go`), Group Containers (`groups.go`), and Application Support regenerable leaves (`appsupport.go`).

---
*Phase: 11-macos-user-system-caches-logs*
*Completed: 2026-10-16*

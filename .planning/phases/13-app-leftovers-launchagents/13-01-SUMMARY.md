---
phase: 13-app-leftovers-launchagents
plan: 01
subsystem: leftovers
tags: [plist, bplist00, launchagents, stale-detection, zero-cgo, macos]

# Dependency graph
requires:
  - phase: 09-macos-system-caches
    provides: Ownership checking pattern without CGO (ownership_darwin.go / ownership_other.go)
provides:
  - Domain constants ReviewReasonStaleLaunchAgent and ReviewReasonAppLeftover
  - Universal pure Go XML and binary (bplist00) plist decoder via howett.net/plist
  - Strict Stale vs Ambiguous LaunchAgent classifier with interpreter evasion and permission degradation
affects: [13-app-leftovers-launchagents, cleaner, engine]

# Actuals
actuals:
  tokens: 18500
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: [howett.net/plist]
  patterns: [transparent-plist-unmarshaling, strict-stale-binary-stat, interpreter-evasion, graceful-permission-degradation]

key-files:
  created:
    - internal/cleaner/leftovers/types.go
    - internal/cleaner/leftovers/plist.go
    - internal/cleaner/leftovers/plist_test.go
    - internal/cleaner/leftovers/fixtures_test.go
    - internal/cleaner/leftovers/ownership_darwin.go
    - internal/cleaner/leftovers/ownership_other.go
    - internal/cleaner/leftovers/launchagents.go
    - internal/cleaner/leftovers/launchagents_test.go
  modified:
    - internal/domain/types.go
    - internal/domain/target_test.go
    - go.mod
    - go.sum

key-decisions:
  - "D-01: Used howett.net/plist v1.0.1 for zero-CGO transparent unmarshaling of both XML and binary (bplist00) property lists"
  - "D-02: Enforced strict stale classification: only direct absolute non-interpreter paths missing on disk become ReviewReasonStaleLaunchAgent"
  - "D-03: Excluded /System/Library in scanLaunchAgents; degraded gracefully with Partial: true and ReviewReasonSystemProtected on permission denial"
  - "D-04: Fail-closed on corrupt or unreadable plists emitting ReviewReasonAmbiguous with 0 bytes and error note"
  - "D-09: Emitted 1 individual target per LaunchAgent plist with ReviewOnly: true and EligibleBytes: 0"
  - "D-12: Added ReviewReasonStaleLaunchAgent ('stale_launchagent') and ReviewReasonAppLeftover ('app_leftover') to domain"

patterns-established:
  - "Interpreter evasion: Evaluated relative/PATH commands and known interpreters (/bin/sh, python, osascript, env, node) to prevent false positives"
  - "Pure Go hermetic plist fixtures: Used plist.Marshal(..., plist.XMLFormat) and plist.Marshal(..., plist.BinaryFormat) in t.TempDir()"

requirements-completed:
  - LEFT-01
  - LEFT-02
  - VAL-01

coverage:
  - id: D1
    description: "Domain review reasons ReviewReasonStaleLaunchAgent and ReviewReasonAppLeftover declared and verified"
    requirement: "LEFT-01"
    verification:
      - kind: unit
        ref: "internal/domain/target_test.go#TestTarget_ReviewReasons"
        status: pass
    human_judgment: false
  - id: D2
    description: "Universal pure Go XML and binary bplist00 property list decoder"
    requirement: "LEFT-01"
    verification:
      - kind: unit
        ref: "internal/cleaner/leftovers/plist_test.go#TestLeftovers_LaunchAgents_XMLAndBinary"
        status: pass
    human_judgment: false
  - id: D3
    description: "Strict Stale LaunchAgent detection for missing absolute executable"
    requirement: "LEFT-02"
    verification:
      - kind: unit
        ref: "internal/cleaner/leftovers/launchagents_test.go#TestLeftovers_LaunchAgents_StaleClassification"
        status: pass
    human_judgment: false
  - id: D4
    description: "Interpreter and wrapper evasion classifying scripts and relative commands as ambiguous"
    requirement: "LEFT-02"
    verification:
      - kind: unit
        ref: "internal/cleaner/leftovers/launchagents_test.go#TestLeftovers_LaunchAgents_InterpretersAmbiguous"
        status: pass
    human_judgment: false
  - id: D5
    description: "Graceful permission degradation on inaccessible directories and symlink/ownership guards"
    requirement: "VAL-01"
    verification:
      - kind: unit
        ref: "internal/cleaner/leftovers/launchagents_test.go#TestLeftovers_LaunchAgents_PermissionDegradation"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-10-18
status: complete
---

# Phase 13 Plan 01: Foundation, Plist Decoder & LaunchAgent Scanner Summary

**Universal pure Go XML/binary plist decoder (`howett.net/plist`), domain review reasons, and deterministic LaunchAgent scanner with strict stale classification and interpreter evasion.**

## Performance

- **Duration:** 12 min
- **Started:** 2026-10-18T12:10:06-03:00
- **Completed:** 2026-10-18T12:22:00-03:00
- **Tasks:** 2
- **Files modified:** 12

## Accomplishments
- Declared domain review reasons `ReviewReasonStaleLaunchAgent` (`"stale_launchagent"`) and `ReviewReasonAppLeftover` (`"app_leftover"`) in `internal/domain/types.go` and verified them in `target_test.go`.
- Implemented pure Go, zero-CGO property list decoding (`plist.go`) supporting both XML and Apple binary formats (`bplist00`) via `howett.net/plist`.
- Created hermetic test fixture utilities in `fixtures_test.go` for creating XML/binary LaunchAgents and mock application bundles.
- Implemented ownership verification (`ownership_darwin.go`, `ownership_other.go`) inspecting `syscall.Stat_t` Uid without CGO.
- Built deterministic LaunchAgent scanner (`launchagents.go`) implementing:
  - Candidate executable extraction (`Program` over `ProgramArguments[0]`).
  - Relative and PATH-based command detection (marking as `ReviewReasonAmbiguous`).
  - Known interpreter evasion (`/bin/sh`, `/bin/bash`, `python`, `node`, `osascript`, `env`, etc., marking as `ReviewReasonAmbiguous`).
  - Strong stale verification (`os.IsNotExist` on direct absolute executable path -> `ReviewReasonStaleLaunchAgent`).
  - Active executable preservation (skipping items whose binary exists on disk).
  - Graceful permission degradation (`Partial: true`, `ReviewReasonSystemProtected`).
  - Symlink guard and current-user ownership enforcement.
  - Strict exclusion of `/System/Library`.

## Task Commits

Each task was committed atomically:

1. **Task 13-01-01: Domain ReviewReason constants, core leftover types, and pure Go XML/binary plist decoder (LEFT-01, D-01, D-12, VAL-01)** - `ae0cdbb` (feat)
2. **Task 13-01-02: LaunchAgent scanner with strict Stale vs. Ambiguous classification, interpreter evasion, and permission degradation (LEFT-01, LEFT-02, D-02, D-03, D-04, D-09, D-16, VAL-01)** - `ab1552d` (feat)

## Files Created/Modified
- `internal/domain/types.go` - Added `ReviewReasonStaleLaunchAgent` and `ReviewReasonAppLeftover` constants.
- `internal/domain/target_test.go` - Added assertions for new review reason constants.
- `internal/cleaner/leftovers/types.go` - Defined `LaunchAgentPlist` and `AppBundleInfoPlist` structs.
- `internal/cleaner/leftovers/plist.go` - Implemented `parseLaunchAgentFile` and `parseAppBundleInfoFile` with `howett.net/plist`.
- `internal/cleaner/leftovers/plist_test.go` - Unit tests for XML and binary plist parsing and corruption handling.
- `internal/cleaner/leftovers/fixtures_test.go` - Fixture generators for XML/binary LaunchAgents, app bundles, and sentinels in `t.TempDir()`.
- `internal/cleaner/leftovers/ownership_darwin.go` - Non-CGO `isOwnedByCurrentUser` for Darwin using `syscall.Stat_t`.
- `internal/cleaner/leftovers/ownership_other.go` - Cross-platform stub for non-Darwin environments.
- `internal/cleaner/leftovers/launchagents.go` - Candidate extraction, interpreter evasion, and LaunchAgent scanner.
- `internal/cleaner/leftovers/launchagents_test.go` - Unit tests for stale detection, active preservation, interpreter evasion, and permission degradation.
- `go.mod` & `go.sum` - Added `howett.net/plist v1.0.1` dependency.

## Decisions Made
- None - followed plan as specified.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Check relative and PATH-based executables before interpreter lookup**
- **Found during:** Task 13-01-02 (`TestLeftovers_LaunchAgents_RelativePathAmbiguous`)
- **Issue:** `node` is present in `knownInterpreters`. When evaluating relative command `node server.js`, the scanner marked it as "interpreter or wrapper invocation" instead of "relative or PATH-based executable".
- **Fix:** Swapped check order in `scanLaunchAgents` so `!filepath.IsAbs(candidate)` is evaluated before `isInterpreter(candidate)`.
- **Files modified:** `internal/cleaner/leftovers/launchagents.go`
- **Verification:** `TestLeftovers_LaunchAgents_RelativePathAmbiguous` passed.
- **Committed in:** `ab1552d` (Task 13-01-02 commit)

---

**Total deviations:** 1 auto-fixed (precedence order in candidate classification)
**Impact on plan:** Essential for precise status notes on relative vs interpreter executables. No scope creep.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- LaunchAgents scanner and pure Go plist decoder are fully tested and functional.
- Ready for Plan 13-02: App Indexer (`apps.go`), Leftover Scanner (`leftovers.go`), and Multi-Tier Exclusion Shield (`exclusions.go`).

---
*Phase: 13-app-leftovers-launchagents*
*Completed: 2026-10-18*

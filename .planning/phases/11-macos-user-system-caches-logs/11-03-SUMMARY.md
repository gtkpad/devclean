---
phase: 11-macos-user-system-caches-logs
plan: 03
subsystem: cleaner/macos
tags: [macos, daemons, group-containers, appsupport, spotify-cache, allowlist, process-guard]

# Dependency graph
requires:
  - phase: 11-01
    provides: Immutable security whitelist, fail-closed process guard, symlink guard, ownership verification
provides:
  - Canonical 8 Apple daemons scanner strictly covering Siri suggestions, Wallpaper agent, media analysis, Geod, Help, AMS, Parsecd, and Handoff with active process guards (MAC-02, D-13)
  - Group Containers scanner with positive allowlist restricting to Library/Caches, Library/Logs, Caches, and Logs while strictly excluding *.dev.orbstack (MAC-02, D-08)
  - Surgical Application Support scanner restricted exclusively to regenerable leaf subdirectories (Cache, Caches, logs, Logs, Crashpad/completed) preserving databases and preferences (MAC-03, D-07)
  - Dedicated Spotify PersistentCache target with active Spotify process guard (MAC-03, D-07)
affects: [11-04]

# Actuals
actuals:
  tokens: 21500
  tasks: 2
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Positive allowlist for canonical 8 Apple daemons with fail-closed process protection
    - Positive allowlist for group container regenerable subpaths (Library/Caches, Library/Logs, Caches, Logs) with strict OrbStack exclusion
    - Surgical leaf filtering in Application Support protecting SQLite databases and preferences
    - Dedicated high-volume Spotify PersistentCache target with isolated process guard

key-files:
  created:
    - internal/cleaner/macos/daemons.go
    - internal/cleaner/macos/daemons_test.go
    - internal/cleaner/macos/groups.go
    - internal/cleaner/macos/groups_test.go
    - internal/cleaner/macos/appsupport.go
    - internal/cleaner/macos/appsupport_test.go
  modified:
    - internal/cleaner/macos/whitelist.go

key-decisions:
  - "D-07: Application Support scanning strictly filters to safe regenerable leaf subdirectories (Cache, Caches, logs, Logs, Crashpad/completed), surfacing Spotify PersistentCache as an independent target and consolidating the rest"
  - "D-08: Group Containers scanning strictly inspects only Library/Caches, Library/Logs, Caches, or Logs, strictly excluding *.dev.orbstack and never targeting container roots"
  - "D-13: Canonical 8 Apple daemons (Siri, Wallpaper, mediaanalysisd, geod, helpd, AMS, parsecd, useractivityd) are covered under positive allowlist with process guard protection"
  - "D-14: Active or uncertain daemon and Spotify processes mark targets ReviewOnly: true with ReviewReasonActiveProcess and EligibleBytes: 0"
  - "D-15: Whitelist strengthens password manager coverage with wildcard patterns for 1Password, Bitwarden, and KeePassXC"

patterns-established:
  - "macos.scanDaemons: Positive allowlist scanner for canonical 8 Apple daemons with active process safeguards"
  - "macos.scanGroupContainers: Regenerable cache and log subfolder scanner with strict *.dev.orbstack exclusion"
  - "macos.scanApplicationSupport: Safe leaf inspector preserving databases and settings, with isolated Spotify PersistentCache discovery"

requirements-completed: [MAC-02, MAC-03]

# Coverage metadata
coverage:
  - id: D1
    description: "Positive allowlist scanner for canonical 8 Apple daemons with active process guards"
    requirement: "MAC-02"
    verification:
      - kind: unit
        ref: "internal/cleaner/macos/daemons_test.go#TestDaemons_CanonicalAllowlist8Daemons"
        status: pass
      - kind: unit
        ref: "internal/cleaner/macos/daemons_test.go#TestDaemons_ActiveProcessGuard"
        status: pass
      - kind: unit
        ref: "internal/cleaner/macos/daemons_test.go#TestDaemons_UncertainProcessGuard"
        status: pass
    human_judgment: false
  - id: D2
    description: "Group Containers scanner with positive allowlist (Library/Caches, Library/Logs, Caches, Logs) strictly excluding *.dev.orbstack and preserving container roots"
    requirement: "MAC-02"
    verification:
      - kind: unit
        ref: "internal/cleaner/macos/groups_test.go#TestGroupContainers_AllowlistedCachesOnly"
        status: pass
      - kind: unit
        ref: "internal/cleaner/macos/groups_test.go#TestGroupContainers_ExcludesOrbStack"
        status: pass
      - kind: unit
        ref: "internal/cleaner/macos/groups_test.go#TestGroupContainers_LargeBundleBreakout"
        status: pass
    human_judgment: false
  - id: D3
    description: "Surgical Application Support scanner targeting safe regenerable leaves while preserving databases and application state"
    requirement: "MAC-03"
    verification:
      - kind: unit
        ref: "internal/cleaner/macos/appsupport_test.go#TestAppSupport_RestrictedLeavesOnly"
        status: pass
    human_judgment: false
  - id: D4
    description: "Dedicated Spotify PersistentCache target with isolated Spotify process guard"
    requirement: "MAC-03"
    verification:
      - kind: unit
        ref: "internal/cleaner/macos/appsupport_test.go#TestAppSupport_SpotifyPersistentCacheDedicated"
        status: pass
      - kind: unit
        ref: "internal/cleaner/macos/appsupport_test.go#TestAppSupport_SpotifyActiveProcessGuard"
        status: pass
    human_judgment: false

# Metrics
duration: 15 min
completed: 2026-10-16
status: complete
---

# Phase 11 Plan 03: Positive Allowlists & Surgical Leaves Summary

**Canonical 8 Apple daemons with active process guards, Group Containers positive allowlist strictly excluding OrbStack, and surgical Application Support leaf filtering with dedicated Spotify PersistentCache target (MAC-02, MAC-03, D-07, D-08, D-13, D-14).**

## Performance

- **Duration:** 15 min
- **Started:** 2026-10-16T20:31:18Z
- **Completed:** 2026-10-16T20:46:00Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments

- Implemented `macos.scanDaemons` covering strictly the canonical 8 Apple daemons (Siri suggestions, macOS Wallpaper Agent, Media Analysis Daemon `com.apple.mediaanalysisd`, Location Services `com.apple.geod`, macOS Help Viewer `com.apple.helpd`, Apple Media Services `com.apple.AppleMediaServices`, Core Parsec Search `com.apple.parsecd`, and Handoff & Universal Clipboard `com.apple.coreservices.useractivityd`) with fail-closed active process safeguards.
- Implemented `macos.scanGroupContainers` enforcing a strict positive allowlist that only scans `Library/Caches`, `Library/Logs`, `Caches`, and `Logs` subdirectories, consolidates groups <= 50MB into `macOS Group Container Caches & Logs`, breaks out groups > 50MB into individual targets, strictly skips `*.dev.orbstack` (preserving Docker/OrbStack VM disks), and never emits container roots or SQLite databases.
- Implemented `macos.scanApplicationSupport` restricting scanning exclusively to safe regenerable leaf subdirectories (`Cache`, `Caches`, `logs`, `Logs`, `Crashpad/completed`), cleanly consolidating eligible leaves into `macOS Application Support Caches & Logs` while strictly leaving application databases (e.g. SQLite databases, cookies, user preferences, licensing data) intact.
- Implemented dedicated discovery for `Spotify PersistentCache` (`~/Library/Application Support/Spotify/PersistentCache`) surfaced as an independent target (`Category: "macos-spotify-cache"`) protected by dedicated `Spotify` process guard inspection.
- Strengthened immutable security whitelist with wildcard patterns for password managers (`*1password*`, `*bitwarden*`, `*keepassxc*`).
- Maintained 100% test pass rate with zero CGO compilation (`CGO_ENABLED=0`) across the entire repository.

## Task Commits

Each task was committed atomically following strict TDD discipline (RED → GREEN):

1. **Task 11-03-01: Canonical 8 Apple daemons and Group Containers positive allowlist**
   - RED test: `42dda09` (`test(11-03): add failing unit tests for apple daemons and group containers`)
   - GREEN feat: `7b1ab1f` (`feat(11-03): implement apple daemons and group containers positive allowlists`)
2. **Task 11-03-02: Surgical Application Support leaves and dedicated Spotify PersistentCache target**
   - RED test: `5f319fa` (`test(11-03): add failing unit tests for application support and spotify cache`)
   - GREEN feat: `4a8abef` (`feat(11-03): implement surgical application support leaves and spotify cache`)

## Files Created/Modified

- `internal/cleaner/macos/daemons.go` - Positive allowlist scanner for canonical 8 Apple daemons with process guards
- `internal/cleaner/macos/daemons_test.go` - Hermetic unit tests for 8 Apple daemons, process guards, and missing directory handling
- `internal/cleaner/macos/groups.go` - Group containers scanner restricting to Caches/Logs and strictly excluding OrbStack
- `internal/cleaner/macos/groups_test.go` - Hermetic unit tests for group container subfolder filtering and OrbStack exclusion
- `internal/cleaner/macos/appsupport.go` - Surgical Application Support leaf filtering and dedicated Spotify PersistentCache target
- `internal/cleaner/macos/appsupport_test.go` - Hermetic unit tests for leaf filtering, database preservation, and Spotify process guard
- `internal/cleaner/macos/whitelist.go` - Added wildcard patterns for password managers in Application Support

## Decisions Made

- Group Containers and Application Support scanners inspect only allowlisted subdirectories, ignoring root level databases, preferences, and documents, preventing any accidental deletion of stateful data.
- Spotify PersistentCache is extracted as its own top-level target rather than folded into the consolidated Application Support row, allowing users to see and purge high-volume streaming caches independently with dedicated process verification.
- Strengthened whitelist pattern matching for password managers (`*1password*`, `*bitwarden*`, `*keepassxc*`) to ensure Application Support folders matching these tools are unconditionally preserved.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Whitelist wildcard patterns for password managers in Application Support**
- **Found during:** Task 11-03-02 (Application Support leaf filtering)
- **Issue:** `whitelist.go` had bundle patterns like `com.1password.*` but missed directory name wildcards like `*1password*`, allowing `Application Support/1Password` to be scanned.
- **Fix:** Added `*1password*`, `*bitwarden*`, and `*keepassxc*` to `immutableSensitivePatterns` in `whitelist.go`.
- **Files modified:** `internal/cleaner/macos/whitelist.go`
- **Verification:** `go test -v ./internal/cleaner/macos -run "TestWhitelist|TestAppSupport"` passes.
- **Committed in:** `4a8abef` (Task 11-03-02 GREEN commit).

---

**Total deviations:** 1 auto-fixed (missing critical security pattern).
**Impact on plan:** Enhanced security guarantees for password manager storage in Application Support. No scope creep.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Canonical Apple daemons, Group Containers positive allowlist, and Application Support leaf filtering completed and verified with all tests passing.
- Ready for Plan 11-04: System Logs & DiagnosticReports (`system.go`), Code-Sign Clones, cleaner lifecycle orchestration (`macos.go`), engine registration, and CLI integration.

---
*Phase: 11-macos-user-system-caches-logs*
*Completed: 2026-10-16*

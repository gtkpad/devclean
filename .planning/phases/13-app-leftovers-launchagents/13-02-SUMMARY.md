---
phase: 13-app-leftovers-launchagents
plan: 02
subsystem: leftovers
tags: [app-index, bundle-scanner, app-leftovers, exclusion-shield, symlink-guard, consolidation, macos]

# Dependency graph
requires:
  - phase: 13-app-leftovers-launchagents
    provides: Plist decoder (parseAppBundleInfoFile), domain review reasons, and LaunchAgent scanner (13-01)
provides:
  - Bounded-depth installed application bundle scanner with immediate .app pruning (apps.go)
  - Bidirectional AppIndex correlating CFBundleIdentifier, app names, helper variants, and suffixes
  - Multi-tier exclusion shield protecting Apple components, 27+ dedicated DevClean cleaners, and CLI utilities (exclusions.go)
  - Consolidated application leftover scanner for Application Support, Caches, and Containers with exact disk metrics (leftovers.go)
affects: [13-app-leftovers-launchagents, cleaner, engine]

# Actuals
actuals:
  tokens: 6000
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns: [depth-bounded-app-discovery, bundle-pruning-skipdir, bidirectional-app-indexing, multi-tier-exclusion-shield, multi-directory-consolidation, symlink-boundary-guard]

key-files:
  created:
    - internal/cleaner/leftovers/apps.go
    - internal/cleaner/leftovers/apps_test.go
    - internal/cleaner/leftovers/exclusions.go
    - internal/cleaner/leftovers/exclusions_test.go
    - internal/cleaner/leftovers/leftovers.go
    - internal/cleaner/leftovers/leftovers_test.go
  modified: []

key-decisions:
  - "D-05: Direct deterministic .app bundle walk limited to depth 3 with immediate filepath.SkipDir at .app boundary preventing traversal into bundle internals"
  - "D-06: Bidirectional AppIndex storing normalized lowercase CFBundleIdentifier and friendly/executable app names, matching exact strings, prefix/helper relations, and last dot segment"
  - "D-07: Multi-tier exclusion shield filtering Apple system components (com.apple.*, apple*), 27+ dedicated DevClean cleaner directories, common CLI utilities without .app (git, ssh, cargo, etc.), core blacklists, and macOS whitelist"
  - "D-08: Strictly limited remnant inspection scope to Application Support, Caches, and Containers"
  - "D-10: Residual directories for the same uninstalled app across the 3 locations are grouped into a single consolidated target per application (e.g. '⊙ App Leftover: OldApp')"
  - "D-11: Real disk usage across consolidated leftover directories is calculated using fs.CalcUsageWithContext without arbitrary size cutoffs"
  - "D-16: Symlink guard verifying symlinks resolve to targets strictly inside the parent directory and not blacklisted"

patterns-established:
  - "Bundle pruning: Returning filepath.SkipDir upon encountering any directory ending in .app ensures scan times remain in milliseconds regardless of app bundle internal size"
  - "Bidirectional app indexing: Cross-matching both reverse DNS identifiers and executable/bundle friendly names prevents false positives for apps whose cache directory does not follow bundle ID conventions"
  - "Multi-directory consolidation: Gathering scattered residual directories in Application Support, Caches, and Containers into a single target per application with aggregate byte metrics and detailed path notes"

requirements-completed:
  - LEFT-03
  - VAL-01

coverage:
  - id: D1
    description: "Installed application bundle discovery with depth 3 limit and .app internal pruning"
    requirement: "LEFT-03"
    verification:
      - kind: unit
        ref: "internal/cleaner/leftovers/apps_test.go#TestLeftovers_AppIndexing_Discovery"
        status: pass
      - kind: unit
        ref: "internal/cleaner/leftovers/apps_test.go#TestLeftovers_AppIndexing_DepthPruning"
        status: pass
    human_judgment: false
  - id: D2
    description: "Bidirectional AppIndex correlating bundle IDs, app names, helper variants, and subdomains"
    requirement: "LEFT-03"
    verification:
      - kind: unit
        ref: "internal/cleaner/leftovers/apps_test.go#TestLeftovers_AppIndexing_Matching"
        status: pass
    human_judgment: false
  - id: D3
    description: "Multi-tier exclusion shield filtering Apple components, dedicated cleaners, and CLI utilities"
    requirement: "LEFT-03"
    verification:
      - kind: unit
        ref: "internal/cleaner/leftovers/exclusions_test.go#TestLeftovers_Exclusions_Shield"
        status: pass
    human_judgment: false
  - id: D4
    description: "Application leftovers scanner across Application Support, Caches, and Containers with multi-directory candidate consolidation"
    requirement: "LEFT-03"
    verification:
      - kind: unit
        ref: "internal/cleaner/leftovers/leftovers_test.go#TestLeftovers_AppLeftovers_DiscoveryAndMatching"
        status: pass
      - kind: unit
        ref: "internal/cleaner/leftovers/leftovers_test.go#TestLeftovers_AppLeftovers_Consolidation"
        status: pass
    human_judgment: false
  - id: D5
    description: "Symlink guard rejecting directory traversal outside scanned locations and blacklists"
    requirement: "VAL-01"
    verification:
      - kind: unit
        ref: "internal/cleaner/leftovers/leftovers_test.go#TestLeftovers_AppLeftovers_SymlinkGuard"
        status: pass
    human_judgment: false

duration: 9min
completed: 2026-10-18
status: complete
---

# Phase 13 Plan 02: App Indexer, Exclusion Shield & Leftovers Scanner Summary

**Depth-bounded installed application indexer (`apps.go`), multi-tier false-positive exclusion shield (`exclusions.go`), and consolidated application leftover scanner for Application Support, Caches, and Containers (`leftovers.go`).**

## Performance

- **Duration:** 9 min
- **Started:** 2026-10-18T12:20:13-03:00
- **Completed:** 2026-10-18T12:29:00-03:00
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Implemented installed application discovery (`apps.go`) bounded to depth 3 across application roots (`/Applications`, `/System/Applications`, `~/Applications`) with immediate `filepath.SkipDir` at `.app` bundle boundaries to prune traversal into heavy bundle internals.
- Built bidirectional `AppIndex` storing normalized lowercase sets of `CFBundleIdentifier`, `CFBundleName`, `CFBundleExecutable`, and bundle folder names, correlating reverse DNS identifiers, helper processes, and trailing dot segments.
- Constructed multi-tier exclusion shield (`exclusions.go`) protecting Apple system components (`com.apple.*`, `apple*`, and system directories), 27+ dedicated DevClean cleaner directories (`docker`, `pnpm`, `homebrew`, `google`, `code`, `cursor`, `firefox`, etc.), common terminal CLI utilities without `.app` (`git`, `ssh`, `cargo`, `mise`, `zsh`), core safety blacklists (`/System`, `/usr`), and macOS sensitive whitelists (`1Password`, `tailscale`).
- Built consolidated application leftover scanner (`leftovers.go`) inspecting `~/Library/Application Support`, `~/Library/Caches`, and `~/Library/Containers`, screening against the exclusion shield and active installed apps.
- Implemented multi-directory leftover candidate grouping (`AppLeftoverGroup`), emitting 1 consolidated target per residual application (`⊙ App Leftover: <app>`) with exact disk metrics via `fs.CalcUsageWithContext` and zero arbitrary size thresholds.
- Enforced symlink boundary guard (`isSafeSymlink`) rejecting symlinks pointing outside scanned locations or into core blacklisted directories.
- Confirmed all emitted leftover targets have `ReviewOnly: true`, `EligibleBytes: 0`, `TTLEligible: false`, `Category: "leftovers-app"`, and `ReviewReason: domain.ReviewReasonAppLeftover`.

## Task Commits

Each task was committed atomically:

1. **Task 13-02-01: Bounded-depth .app bundle traversal, Info.plist metadata extraction, and bidirectional AppIndex (LEFT-03, D-05, D-06, VAL-01)** - `e0071f1` (feat)
2. **Task 13-02-02: Multi-tier exclusion shield, leftovers scanner for Application Support, Caches, and Containers, and multi-directory candidate consolidation (LEFT-03, D-07, D-08, D-10, D-11, D-16, VAL-01)** - `f5093d1` (feat)

## Files Created/Modified
- `internal/cleaner/leftovers/apps.go` - Installed app traversal, Info.plist extraction, and bidirectional `AppIndex` with `Matches()` method.
- `internal/cleaner/leftovers/apps_test.go` - Unit tests for bundle discovery, depth pruning, and bidirectional matching logic.
- `internal/cleaner/leftovers/exclusions.go` - Multi-tier exclusion shield for Apple components, dedicated cleaner directories, and CLI utilities.
- `internal/cleaner/leftovers/exclusions_test.go` - Table-driven unit tests verifying exclusion rules across all categories.
- `internal/cleaner/leftovers/leftovers.go` - App leftovers scanner across Application Support, Caches, and Containers with multi-directory consolidation and disk metrics.
- `internal/cleaner/leftovers/leftovers_test.go` - Unit tests for discovery, installed app preservation, consolidation, and symlink guards.

## Decisions Made
- Followed plan decisions D-05, D-06, D-07, D-08, D-10, D-11, D-16 as specified.

## Deviations from Plan
None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- App Indexer, Exclusion Shield, and Leftovers Scanner are fully tested and functional.
- Ready for Plan 13-03: Cleaner Lifecycle (`cleaner.go`), Inviolable `Clean()` Guard, Config & Engine Wiring (`config.go`, `register.go`).

---
*Phase: 13-app-leftovers-launchagents*
*Completed: 2026-10-18*

---
phase: 11-macos-user-system-caches-logs
plan: 01
subsystem: cleaner/macos
tags: [macos, security, whitelist, process-guard, ownership, symlink-guard]

# Dependency graph
requires:
  - phase: 06-10
    provides: Unified core cleaner, safety blacklist, and CommandExecutor abstractions
provides:
  - Inviolable security whitelist blocking CloudKit, Mobile Documents, Keychain, Safari state, 1Password, Bitwarden, and VPNs (MAC-05, D-15)
  - Deterministic Process Guard querying pgrep -x with fail-closed semantics and cleaner.CommandExecutor (MAC-05, D-14)
  - Pure Go Zero-CGO UID ownership verification via syscall.Stat_t on Darwin and portable fallback (MAC-05, D-16)
  - Symlink Guard rejecting link escapes and references to blacklisted system roots (MAC-05, D-16)
affects: [11-02, 11-03, 11-04, 12-browsers]

# Actuals
actuals:
  tokens: 18500
  tasks: 2
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Immutable security whitelist immune to flags or configuration overrides
    - Fail-closed process status inspection via cleaner.CommandExecutor without shell interpreters
    - Zero-CGO Darwin syscall.Stat_t.Uid ownership verification

key-files:
  created:
    - internal/cleaner/macos/whitelist.go
    - internal/cleaner/macos/whitelist_test.go
    - internal/cleaner/macos/process.go
    - internal/cleaner/macos/process_test.go
    - internal/cleaner/macos/ownership_darwin.go
    - internal/cleaner/macos/ownership_other.go
    - internal/cleaner/macos/ownership_test.go
  modified: []

key-decisions:
  - "D-14: Fail-closed Process Guard maps bundles to processes via CommandExecutor running 'pgrep -x', returning uncertain=true on unexpected errors or exit code != 1"
  - "D-15: Immutable in-Go whitelist blocks CloudKit, Mobile Documents, Keychain, Safari state, VPNs, and Password Managers regardless of flags or configuration"
  - "D-16: Zero-CGO ownership check validates st_uid == current_uid via syscall.Stat_t on Darwin, preventing cross-user file tampering"
  - "D-16: Symlink Guard evaluates paths with filepath.EvalSymlinks on both link and cacheRoot, rejecting escapes and blacklisted system roots"

patterns-established:
  - "macos.IsWhitelisted: Inviolable case-insensitive pattern matching across base names, full paths, and path components"
  - "macos.CheckProcessStatus / IsBundleProcessRunning: Shell-free pgrep execution with strict fail-closed safety posture"
  - "macos.isOwnedByCurrentUser: Zero-CGO UID inspection with Darwin build tags and portable fallback"
  - "macos.isSafeSymlink: Canonical symlink boundary verification with Darwin /var -> /private/var normalization"

requirements-completed: [MAC-05]

# Coverage metadata
coverage:
  - id: D1
    description: "Immutable security whitelist in Go blocking sensitive Apple sync/cloud paths, password managers, and VPN profiles"
    requirement: "MAC-05"
    verification:
      - kind: unit
        ref: "internal/cleaner/macos/whitelist_test.go#TestWhitelist_ImmutablePatterns"
        status: pass
    human_judgment: false
  - id: D2
    description: "Deterministic Process Guard with fail-closed semantics mapping bundle IDs to running processes"
    requirement: "MAC-05"
    verification:
      - kind: unit
        ref: "internal/cleaner/macos/process_test.go#TestProcessGuard_Running"
        status: pass
      - kind: unit
        ref: "internal/cleaner/macos/process_test.go#TestProcessGuard_Stopped"
        status: pass
      - kind: unit
        ref: "internal/cleaner/macos/process_test.go#TestProcessGuard_FailClosedOnUnexpectedExit"
        status: pass
      - kind: unit
        ref: "internal/cleaner/macos/process_test.go#TestProcessGuard_FailClosedOnNilExecutor"
        status: pass
      - kind: unit
        ref: "internal/cleaner/macos/process_test.go#TestProcessGuard_KnownBundleMappings"
        status: pass
    human_judgment: false
  - id: D3
    description: "Zero-CGO ownership verification comparing syscall.Stat_t.Uid against current UID"
    requirement: "MAC-05"
    verification:
      - kind: unit
        ref: "internal/cleaner/macos/ownership_test.go#TestOwnership_CurrentUser"
        status: pass
      - kind: unit
        ref: "internal/cleaner/macos/ownership_test.go#TestOwnership_MismatchedUID"
        status: pass
    human_judgment: false
  - id: D4
    description: "Symlink Guard rejecting links escaping cache boundary or pointing to system roots"
    requirement: "MAC-05"
    verification:
      - kind: unit
        ref: "internal/cleaner/macos/ownership_test.go#TestSymlinkGuard_ExternalReference"
        status: pass
    human_judgment: false

# Metrics
duration: 12 min
completed: 2026-10-16
status: complete
---

# Phase 11 Plan 01: macOS Security Foundation & Tracer Summary

**Immutable security whitelist, fail-closed Process Guard with CommandExecutor, Zero-CGO ownership verification, and Symlink Guard for the macOS cleaner (MAC-05).**

## Performance

- **Duration:** 12 min
- **Started:** 2026-10-16T20:08:18Z
- **Completed:** 2026-10-16T20:20:00Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments

- Implemented `macos.IsWhitelisted(path string) bool` with immutable Go patterns protecting CloudKit, Mobile Documents, Keychain, Safari state, 1Password, Bitwarden, KeePassXC, Tailscale, WireGuard, ZeroTier, and Cloudflare WARP across all casing and path variations.
- Implemented `macos.CheckProcessStatus` and `macos.IsBundleProcessRunning` with deterministic `pgrep -x` invocation via `cleaner.CommandExecutor` adhering to fail-closed semantics (exit != 1 or error -> uncertain=true).
- Implemented pure Go zero-CGO ownership check (`isOwnedByCurrentUser`) using `syscall.Stat_t` on Darwin with portable fallback for non-Darwin environments, fully verified with `CGO_ENABLED=0`.
- Implemented `isSafeSymlink` with bidirectional symlink resolution (`cacheRoot` and `linkPath`) accounting for macOS `/var` -> `/private/var` symlink aliases and blacklisted system roots.
- Achieved 90.6% statement coverage on package `internal/cleaner/macos` with 100% hermetic unit tests.

## Task Commits

Each task was committed atomically following strict TDD discipline (RED → GREEN):

1. **Task 11-01-01: Immutable security whitelist, Symlink Guard, and Zero-CGO ownership**
   - RED test: `023693a` (`test(11-01): add failing unit tests for immutable whitelist, symlink guard, and zero-CGO ownership`)
   - GREEN feat: `ea2e792` (`feat(11-01): implement immutable whitelist, symlink guard, and zero-CGO ownership`)
2. **Task 11-01-02: Deterministic Process Guard with CommandExecutor fail-closed semantics**
   - RED test: `41f5045` (`test(11-01): add failing unit tests for process guard`)
   - GREEN feat: `a53f1d9` (`feat(11-01): implement process guard with fail-closed semantics`)

## Files Created/Modified

- `internal/cleaner/macos/whitelist.go` - Immutable sensitive pattern whitelist and symlink safety verification
- `internal/cleaner/macos/whitelist_test.go` - Hermetic unit tests for whitelist pattern matching and false-positive immunity
- `internal/cleaner/macos/ownership_darwin.go` - Darwin syscall.Stat_t.Uid current-user verification (zero CGO)
- `internal/cleaner/macos/ownership_other.go` - Portable fallback for non-Darwin environments
- `internal/cleaner/macos/ownership_test.go` - Unit tests for UID matching, mismatched UIDs, and symlink escape rejection
- `internal/cleaner/macos/process.go` - Known bundle-to-process mapping and fail-closed pgrep inspector
- `internal/cleaner/macos/process_test.go` - Mocked CommandExecutor unit tests for running, stopped, and uncertain process states

## Decisions Made

- Standardized `IsBundleProcessRunning` to strip `com.apple.` and `com.` prefixes as an automatic fallback when an application is not explicitly listed in `knownProcessMap`.
- Resolved both `cacheRoot` and `linkPath` via `filepath.EvalSymlinks` in `isSafeSymlink` to cleanly handle Darwin's default `/var` -> `/private/var` symlink structure without false-positive escape rejections.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- Initial symlink guard test failed on macOS temporary directories because `/var/folders` resolves to `/private/var/folders` when evaluating the link target. Resolved by canonicalizing `cacheRoot` through `filepath.EvalSymlinks` as well prior to relative path calculation.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Security foundation (whitelist, process guard, symlink guard, ownership verification) complete and passing all automated tests.
- Ready for Plan 11-02: User Essentials (`~/Library/Caches/*`, `~/Library/Logs/*`, deduplication exclusions, 50MB consolidation, and `$HOME/.Trash` ReviewOnly discovery and authorized direct purging).

---
*Phase: 11-macos-user-system-caches-logs*
*Completed: 2026-10-16*

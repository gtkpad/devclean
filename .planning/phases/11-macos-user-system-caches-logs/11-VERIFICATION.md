---
phase: 11-macos-user-system-caches-logs
verified: 2026-10-16T21:26:00Z
status: passed
score: 5/5 must-haves verified
covered_files:
  - .planning/phases/11-macos-user-system-caches-logs/11-01-PLAN.md
  - .planning/phases/11-macos-user-system-caches-logs/11-01-SUMMARY.md
  - .planning/phases/11-macos-user-system-caches-logs/11-02-PLAN.md
  - .planning/phases/11-macos-user-system-caches-logs/11-02-SUMMARY.md
  - .planning/phases/11-macos-user-system-caches-logs/11-03-PLAN.md
  - .planning/phases/11-macos-user-system-caches-logs/11-03-SUMMARY.md
  - .planning/phases/11-macos-user-system-caches-logs/11-04-PLAN.md
  - .planning/phases/11-macos-user-system-caches-logs/11-04-SUMMARY.md
  - cmd/clean.go
  - internal/cleaner/macos/appsupport.go
  - internal/cleaner/macos/appsupport_test.go
  - internal/cleaner/macos/daemons.go
  - internal/cleaner/macos/daemons_test.go
  - internal/cleaner/macos/groups.go
  - internal/cleaner/macos/groups_test.go
  - internal/cleaner/macos/macos.go
  - internal/cleaner/macos/macos_test.go
  - internal/cleaner/macos/ownership_darwin.go
  - internal/cleaner/macos/ownership_other.go
  - internal/cleaner/macos/ownership_test.go
  - internal/cleaner/macos/process.go
  - internal/cleaner/macos/process_test.go
  - internal/cleaner/macos/system.go
  - internal/cleaner/macos/system_test.go
  - internal/cleaner/macos/trash.go
  - internal/cleaner/macos/trash_test.go
  - internal/cleaner/macos/user.go
  - internal/cleaner/macos/user_test.go
  - internal/cleaner/macos/whitelist.go
  - internal/cleaner/macos/whitelist_test.go
  - internal/config/config.go
  - internal/config/defaults.go
  - internal/deleter/deleter.go
  - internal/deleter/policy_test.go
  - internal/engine/engine.go
  - internal/engine/engine_test.go
  - internal/engine/register.go
  - internal/engine/register_test.go
covered_digest: "v3:sha256:9616a7dfe4602d318ae94e7294cba5edee244918519b5b1738242d660a1e2426"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 11: macOS User/System Caches & Logs Verification Report

**Phase Goal:** Users can inventory allowlisted macOS caches, logs, Trash, and application leaves with honest permission coverage and protection for active or sensitive data.  
**Verified:** 2026-10-16T21:26:00Z  
**Status:** passed  
**Re-verification:** No — initial verification  

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | User caches, user logs, and Trash are inventoried; Trash is eligible only when an explicit option is enabled. | ✓ VERIFIED | `scanUserCaches` and `scanUserLogs` discover user caches/logs (`internal/cleaner/macos/user.go`). `scanTrash` sets `ReviewOnly: true`, `EligibleBytes: 0`, and `ReviewReason: "trash_requires_flag"` by default (`internal/cleaner/macos/trash.go`). CLI `--trash` flag coordinates eligibility (`cmd/clean.go`). Unit tested in `TestTrash_ScanAlwaysReviewOnly` and `TestCleanCmd_NewEcosystemsValidation`. |
| 2 | Apple daemon caches, Group Container caches/logs, Handoff, Spotify PersistentCache, and specified Application Support leaves use positive allowlists with dedicated ownership and no overlap double-counting. | ✓ VERIFIED | `canonicalDaemons` positive allowlist strictly covers 8 daemons (`internal/cleaner/macos/daemons.go`). `scanGroupContainers` restricts to `Library/Caches` and `Library/Logs`, strictly excluding `*.dev.orbstack` (`internal/cleaner/macos/groups.go`). `scanApplicationSupport` restricts to regenerable leaves (`Cache`, `Caches`, `logs`, `Logs`, `Crashpad/completed`) and isolates `Spotify PersistentCache` (`internal/cleaner/macos/appsupport.go`). Canonical deduplication table skips dedicated ecosystem caches (`internal/cleaner/macos/user.go`). Tested in `TestDaemons_CanonicalAllowlist8Daemons`, `TestGroupContainers_ExcludesOrbStack`, `TestAppSupport_RestrictedLeavesOnly`, and `TestUserCaches_DeduplicationExclusions`. |
| 3 | System logs, DiagnosticReports, and code-sign clones report incomplete coverage on permission or TCC denial rather than silently reporting zero bytes. | ✓ VERIFIED | `scanDirectoryWithTCCFallback` traps `os.ErrPermission`, accumulates readable bytes, and sets `ProviderMetrics.Partial = true` with `StatusNote: "permission denied (TCC / root required)"` (`internal/cleaner/macos/system.go`). DiagnosticReports segregated into User (eligible) and System (ReviewOnly). Clones search strictly current user's `$TMPDIR/../C/` with Chrome process guard. Tested in `TestSystemLogs_GracefulDegradationTCC`, `TestDiagnosticReports_UserEligible_SystemReviewOnly`, and `TestCodeSignClones_IsolatedToUserTmpdir`. |
| 4 | Process Guard, immutable whitelist, symlink checks, ownership checks, and immediate pre-delete revalidation prevent cleanup of active applications or protected data. | ✓ VERIFIED | `whitelist.go` immutable pattern list protects CloudKit, Mobile Documents, Keychain, Safari state, 1Password, Bitwarden, KeePassXC, Tailscale, WireGuard, ZeroTier, Cloudflare WARP across all paths. `process.go` implements fail-closed `pgrep -x` check. `ownership_darwin.go` validates `st_uid == current_uid` without CGO. `isSafeSymlink` rejects escapes. `macosCleaner.Clean()` revalidates active processes immediately prior to deletion. Tested in `TestWhitelist_ImmutablePatterns`, `TestProcessGuard_FailClosedOnUnexpectedExit`, `TestMacOSCleaner_PreDeleteProcessRevalidation`, and `TestSymlinkGuard_ExternalReference`. |
| 5 | macOS cleaner fixtures preserve protected sentinels across scan, dry-run, and deletion, including permission, symlink, race, and unknown-process cases; Darwin builds remain zero-CGO. | ✓ VERIFIED | `TestCoordinator_DryRun_ZeroMutation_Phase11` verifies 100% byte integrity (SHA-256, size, modtime) across user trash, system logs, diagnostic reports, and vaults under DryRun. `TestMacOSCleaner_PreDeleteProcessRevalidation` verifies deletion abort on post-scan process launch. `CGO_ENABLED=0 go test ./...` passes across Darwin and Linux. |

**Score:** 5/5 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/cleaner/macos/whitelist.go` | Inviolable `IsWhitelisted(path string) bool` check for sensitive vaults and bundles | ✓ VERIFIED | Present, substantive (131 lines), wired in user, groups, appsupport, and macos cleaners |
| `internal/cleaner/macos/whitelist_test.go` | Hermetic unit test suite for immutable whitelist matching patterns and casing | ✓ VERIFIED | Present, substantive (157 lines), 41 table-driven subtests passing |
| `internal/cleaner/macos/process.go` | `CheckProcessStatus` and `IsBundleProcessRunning` with fail-closed `pgrep -x` inspection | ✓ VERIFIED | Present, substantive (87 lines), wired in user, daemons, appsupport, system, and macos cleaners |
| `internal/cleaner/macos/process_test.go` | Hermetic unit tests for running, stopped, and uncertain process states | ✓ VERIFIED | Present, substantive (163 lines), all tests passing |
| `internal/cleaner/macos/ownership_darwin.go` | Darwin `syscall.Stat_t.Uid == current UID` verification without CGO | ✓ VERIFIED | Present, substantive (21 lines), builds with `//go:build darwin` |
| `internal/cleaner/macos/ownership_other.go` | Portable fallback ownership verification for Linux/CI | ✓ VERIFIED | Present, substantive (16 lines), builds with `//go:build !darwin` |
| `internal/cleaner/macos/ownership_test.go` | Hermetic unit tests for symlink escapes and UID ownership verification | ✓ VERIFIED | Present, substantive (83 lines), all tests passing |
| `internal/cleaner/macos/user.go` | `scanUserCaches` and `scanUserLogs` with deduplication exclusions and 50MB consolidation | ✓ VERIFIED | Present, substantive (208 lines), wired into `macosCleaner.Scan` |
| `internal/cleaner/macos/user_test.go` | Hermetic unit tests for deduplication table, bundle consolidation, and process guard | ✓ VERIFIED | Present, substantive (233 lines), all tests passing |
| `internal/cleaner/macos/trash.go` | `scanTrash` (ReviewOnly default) and `cleanTrash` (direct purge, chmod recovery, root preservation) | ✓ VERIFIED | Present, substantive (91 lines), wired into `macosCleaner.Scan` and `deleter.Coordinator` |
| `internal/cleaner/macos/trash_test.go` | Hermetic unit tests for trash scan invariants, direct purge, chmod recovery, and volume exclusion | ✓ VERIFIED | Present, substantive (134 lines), all tests passing |
| `internal/cleaner/macos/daemons.go` | `scanDaemons` with positive allowlist for canonical 8 Apple daemons | ✓ VERIFIED | Present, substantive (81 lines), wired into `macosCleaner.Scan` |
| `internal/cleaner/macos/daemons_test.go` | Hermetic unit tests for 8 Apple daemons and active daemon process guards | ✓ VERIFIED | Present, substantive (119 lines), all tests passing |
| `internal/cleaner/macos/groups.go` | `scanGroupContainers` with positive allowlist and strict exclusion of `*.dev.orbstack` | ✓ VERIFIED | Present, substantive (97 lines), wired into `macosCleaner.Scan` |
| `internal/cleaner/macos/groups_test.go` | Hermetic unit tests for Group Containers cache/log subfolder restriction and OrbStack omission | ✓ VERIFIED | Present, substantive (117 lines), all tests passing |
| `internal/cleaner/macos/appsupport.go` | `scanApplicationSupport` with surgical leaf filtering and dedicated Spotify PersistentCache target | ✓ VERIFIED | Present, substantive (108 lines), wired into `macosCleaner.Scan` |
| `internal/cleaner/macos/appsupport_test.go` | Hermetic unit tests asserting databases/preferences preservation and Spotify target isolation | ✓ VERIFIED | Present, substantive (129 lines), all tests passing |
| `internal/cleaner/macos/system.go` | `scanSystemLogsAndReports`, `scanDirectoryWithTCCFallback`, and `scanCodeSignClones` | ✓ VERIFIED | Present, substantive (186 lines), wired into `macosCleaner.Scan` |
| `internal/cleaner/macos/system_test.go` | Hermetic unit tests for TCC fallback degradation, report segregation, and code-sign clones | ✓ VERIFIED | Present, substantive (145 lines), all tests passing |
| `internal/cleaner/macos/macos.go` | `macosCleaner` implementing `domain.Cleaner` with pre-delete revalidation in `Clean()` | ✓ VERIFIED | Present, substantive (172 lines), registered in engine |
| `internal/cleaner/macos/macos_test.go` | Complete lifecycle tests (`IsInstalled`, `Scan`, `Clean`, `DryRun`) with mock executor | ✓ VERIFIED | Present, substantive (284 lines), all tests passing |
| `internal/config/config.go` | `EcosystemsConfig.MacOS` and `HooksConfig.MacOS` | ✓ VERIFIED | Present, mapped to config parser |
| `internal/config/defaults.go` | Default MacOS config with `MaxAgeDays: 14` | ✓ VERIFIED | Present, defaults populated |
| `internal/engine/register.go` | `macos.NewWithExecutor(exec)` registration in `RegisterDefaultCleanersWithExecutor` | ✓ VERIFIED | Present, wired in engine registration |
| `cmd/clean.go` | CLI `--trash` integration toggling `macos-trash` eligibility | ✓ VERIFIED | Present, wired to candidate filtering |
| `internal/deleter/deleter.go` | Direct purging partition for `Category == 'macos-trash'` bypassing internal quarantine | ✓ VERIFIED | Present, routed to `cleaner.Clean` |
| `internal/deleter/policy_test.go` | `TestCoordinator_DryRun_ZeroMutation_Phase11` canary integration test | ✓ VERIFIED | Present, substantive canary test passing |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/cleaner/macos/whitelist.go` | `internal/safety/blacklist.go` | `isSafeSymlink` calls `safety.IsCoreBlacklisted(evalLink)` | ✓ WIRED | Evaluates links against core blacklist roots |
| `internal/cleaner/macos/process.go` | `internal/cleaner/executor.go` | `cleaner.CommandExecutor` interface | ✓ WIRED | Mockable `pgrep` execution without shell injection |
| `internal/cleaner/macos/ownership_darwin.go` | `syscall.Stat_t` | `info.Sys().(*syscall.Stat_t).Uid` | ✓ WIRED | Native Darwin stat structure accessed without CGO |
| `internal/cleaner/macos/user.go` | `internal/cleaner/macos/whitelist.go` | `IsWhitelisted(fullPath)` / `IsWhitelisted(name)` | ✓ WIRED | Checks whitelist before inspecting app caches |
| `internal/cleaner/macos/user.go` | `internal/cleaner/macos/process.go` | `IsBundleProcessRunning(ctx, exec, name)` | ✓ WIRED | Consults process guard for each app bundle |
| `internal/cleaner/macos/trash.go` | `internal/fs/chmod.go` | `fs.ChmodRecursive` | ✓ WIRED | Recovers permissions on locked read-only trash items |
| `internal/cleaner/macos/daemons.go` | `internal/cleaner/macos/process.go` | `IsBundleProcessRunning(ctx, exec, d.BundleID)` | ✓ WIRED | Verifies active daemon state for all 8 daemons |
| `internal/cleaner/macos/groups.go` | `internal/cleaner/docker/orbstack.go` | Strict exclusion check `strings.Contains(name, ".dev.orbstack")` | ✓ WIRED | Enforces clean boundary protecting Docker/OrbStack VM disks |
| `internal/cleaner/macos/appsupport.go` | `internal/cleaner/macos/process.go` | `CheckProcessStatus(ctx, exec, "Spotify")` | ✓ WIRED | Protects running Spotify client during cache scan |
| `internal/cleaner/macos/system.go` | `internal/domain/types.go` | `domain.ProviderMetrics{Partial: true, StatusNote: ...}` | ✓ WIRED | Populates TCC fallback metrics into `domain.Target` |
| `internal/cleaner/macos/macos.go` | `internal/domain/types.go` | `domain.Cleaner` interface | ✓ WIRED | Implements `Name()`, `IsInstalled()`, `Scan()`, `Clean()` |
| `internal/engine/register.go` | `internal/cleaner/macos/macos.go` | `eng.Register(macos.NewWithExecutor(exec))` | ✓ WIRED | Registers `macos` cleaner into engine |
| `internal/deleter/deleter.go` | `internal/cleaner/macos/trash.go` | Direct `cleaner.Clean(ctx, directTargets)` execution | ✓ WIRED | Bypasses quarantine for `macos-trash`, directly calling `cleanTrash()` |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `internal/cleaner/macos/user.go` | `scanUserCaches` targets | Real filesystem traversal via `os.ReadDir(cachesRoot)` and `fs.CalcUsage` | Yes | ✓ FLOWING |
| `internal/cleaner/macos/user.go` | `scanUserLogs` targets | Real filesystem traversal via `os.ReadDir(logsRoot)` and `fs.CalcUsage` | Yes | ✓ FLOWING |
| `internal/cleaner/macos/trash.go` | `scanTrash` target | Real filesystem stat and usage via `fs.CalcUsage(trashDir)` | Yes | ✓ FLOWING |
| `internal/cleaner/macos/daemons.go` | `scanDaemons` targets | Real filesystem stat and usage via `fs.CalcUsage(fullPath)` | Yes | ✓ FLOWING |
| `internal/cleaner/macos/groups.go` | `scanGroupContainers` targets | Real filesystem traversal via `os.ReadDir(groupsRoot)` and `fs.CalcUsage` | Yes | ✓ FLOWING |
| `internal/cleaner/macos/appsupport.go` | `scanApplicationSupport` targets | Real filesystem traversal and `fs.CalcUsage` on allowed leaves | Yes | ✓ FLOWING |
| `internal/cleaner/macos/system.go` | `scanDirectoryWithTCCFallback` | Real `filepath.WalkDir` accumulating entry sizes | Yes | ✓ FLOWING |
| `internal/cleaner/macos/system.go` | `scanCodeSignClones` | Real `filepath.WalkDir` over `$TMPDIR/../C/` | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Whitelist blocks sensitive patterns | `go test -v ./internal/cleaner/macos -run TestWhitelist_ImmutablePatterns` | 41/41 subtests pass | ✓ PASS |
| Process guard fail-closed on unexpected error | `go test -v ./internal/cleaner/macos -run TestProcessGuard_FailClosedOnUnexpectedExit` | Pass | ✓ PASS |
| Trash always ReviewOnly: true with EligibleBytes: 0 on scan | `go test -v ./internal/cleaner/macos -run TestTrash_ScanAlwaysReviewOnly` | Pass | ✓ PASS |
| Pre-deletion process revalidation aborts clean on launched app | `go test -v ./internal/cleaner/macos -run TestMacOSCleaner_PreDeleteProcessRevalidation` | Pass | ✓ PASS |
| TCC permission denial degrades gracefully with partial metrics | `go test -v ./internal/cleaner/macos -run TestSystemLogs_GracefulDegradationTCC` | Pass | ✓ PASS |
| DryRun zero mutation canary integrity test | `go test -v ./internal/deleter -run TestCoordinator_DryRun_ZeroMutation_Phase11` | Pass | ✓ PASS |
| Full package test execution | `go test -v ./internal/cleaner/macos/...` | 27 tests pass | ✓ PASS |
| Full workspace test execution | `go test ./... && CGO_ENABLED=0 go test ./...` | All packages pass with zero CGO | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| **MAC-01** | 11-02, 11-04 | User caches, user logs and Trash are inventoried, with Trash requiring an explicit option. | ✓ SATISFIED | `internal/cleaner/macos/user.go` (`scanUserCaches`, `scanUserLogs`), `internal/cleaner/macos/trash.go` (`scanTrash` ReviewOnly default, `cleanTrash` direct purge), `cmd/clean.go` (`--trash` flag coordination). |
| **MAC-02** | 11-03, 11-04 | Apple daemon caches, Group Container caches/logs and Handoff use positive allowlists. | ✓ SATISFIED | `internal/cleaner/macos/daemons.go` (canonical 8 daemons positive allowlist), `internal/cleaner/macos/groups.go` (Group Containers positive allowlist for Caches/Logs, strict exclusion of `*.dev.orbstack`). |
| **MAC-03** | 11-03, 11-04 | Application Support scanning covers only specified regenerable cache/log/Crashpad leaves, including Spotify PersistentCache. | ✓ SATISFIED | `internal/cleaner/macos/appsupport.go` (allowlist: `Cache`, `Caches`, `logs`, `Logs`, `Crashpad/completed`, plus dedicated `Spotify/PersistentCache` target with Spotify process guard). |
| **MAC-04** | 11-04 | System logs, DiagnosticReports and code-sign clones degrade gracefully on permission or TCC denial and report incomplete coverage. | ✓ SATISFIED | `internal/cleaner/macos/system.go` (`scanDirectoryWithTCCFallback` populates `ProviderMetrics.Partial` and `StatusNote`, User vs System DiagnosticReports segregation, Darwin user-isolated `$TMPDIR/../C/` clone search). |
| **MAC-05** | 11-01, 11-04 | Process Guard, immutable whitelist, symlink guard and ownership checks prevent cleanup of active applications or protected data. | ✓ SATISFIED | `internal/cleaner/macos/whitelist.go` (inviolable sensitive pattern whitelist, `isSafeSymlink`), `internal/cleaner/macos/process.go` (fail-closed `pgrep -x`), `internal/cleaner/macos/ownership_darwin.go` (zero-CGO UID verification), `internal/cleaner/macos/macos.go` (`Clean()` pre-delete revalidation). |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None | - | - | - | Zero anti-patterns detected across all 16 implementation files |

### Human Verification Required

None — all behavioral invariants, edge cases, permission denials, and race condition revalidations are proven via deterministic unit and integration test fixtures.

### Gaps Summary

No gaps identified. All 5 phase requirements (`MAC-01`, `MAC-02`, `MAC-03`, `MAC-04`, `MAC-05`) and all 5 Roadmap Success Criteria are fully satisfied by substantive implementations, verified connections, zero-mutation canary safety, and passing test suites under both native and zero-CGO compilation.

---

_Verified: 2026-10-16T21:26:00Z_  
_Verifier: Claude (gsd-verifier)_

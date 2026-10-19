---
phase: "11"
slug: "macos-user-system-caches-logs"
status: approved
nyquist_compliant: true
wave_0_complete: false
created: "2026-10-16"
---

# Phase 11 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go 1.24 toolchain) |
| **Config file** | none (standard go toolchain) |
| **Quick run command** | `go test -v ./internal/cleaner/macos/...` |
| **Full suite command** | `CGO_ENABLED=0 go test ./...` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -v ./internal/cleaner/macos/...`
- **After every plan wave:** Run `CGO_ENABLED=0 go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 11-01-01 | 01 | 1 | MAC-05 | T-11-01 / T-11-04 | Immutable whitelist blocks sensitive paths; symlink guard rejects escape; zero-CGO ownership check | unit | `go test -v ./internal/cleaner/macos -run "TestWhitelist|TestSymlinkGuard|TestOwnership"` | ❌ W0 | ⬜ pending |
| 11-01-02 | 01 | 1 | MAC-05 | T-11-02 / T-11-03 | Fail-closed process guard via pgrep -x; pre-delete revalidation in Clean() | unit | `go test -v ./internal/cleaner/macos -run "TestProcessGuard"` | ❌ W0 | ⬜ pending |
| 11-02-01 | 02 | 2 | MAC-01 | T-11-05 | User caches & logs deduplication exclusions (pnpm, uv, etc.) & 50MB bundle consolidation | unit | `go test -v ./internal/cleaner/macos -run "TestUserCaches|TestUserLogs"` | ❌ W0 | ⬜ pending |
| 11-02-02 | 02 | 2 | MAC-01 | T-11-06 | Trash scan always ReviewOnly by default; direct purge preserves root only with explicit flag | unit | `go test -v ./internal/cleaner/macos -run "TestTrash"` | ❌ W0 | ⬜ pending |
| 11-03-01 | 03 | 3 | MAC-02 | T-11-07 | Canonical allowlist of 8 Apple daemons; Group Containers caches/logs positive allowlist (no OrbStack) | unit | `go test -v ./internal/cleaner/macos -run "TestDaemons|TestGroupContainers"` | ❌ W0 | ⬜ pending |
| 11-03-02 | 03 | 3 | MAC-03 | T-11-08 | Application Support restricted leaves only; dedicated Spotify PersistentCache target | unit | `go test -v ./internal/cleaner/macos -run "TestAppSupport"` | ❌ W0 | ⬜ pending |
| 11-04-01 | 04 | 4 | MAC-04 | T-11-09 | User vs System DiagnosticReports segregation; graceful TCC degradation with ProviderMetrics.Partial | unit | `go test -v ./internal/cleaner/macos -run "TestDiagnosticReports|TestSystemLogs"` | ❌ W0 | ⬜ pending |
| 11-04-02 | 04 | 4 | MAC-04 | T-11-10 | Darwin code_sign_clone discovery isolated to user TMPDIR/C/ | unit | `go test -v ./internal/cleaner/macos -run "TestCodeSignClones"` | ❌ W0 | ⬜ pending |
| 11-04-03 | 04 | 4 | MAC-01..05 | T-11-01..10 | Engine registration, CLI integration, zero-mutation ReviewOnly coordinator verification, zero-CGO build | integration | `go test -v ./internal/engine -run "TestRegisterDefaultCleaners" && go test -v ./cmd -run "TestClean" && CGO_ENABLED=0 go test ./...` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/cleaner/macos/macos_test.go` — initial package test fixture and sentinels
- [ ] `internal/cleaner/macos/whitelist_test.go` — security whitelist test assertions

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real macOS host TCC / Full Disk Access permission behavior | MAC-04 | Requires non-virtualized macOS environment with active TCC sandbox | Executed in Phase 14 validation on host machine |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 10s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved (GSD Planner)

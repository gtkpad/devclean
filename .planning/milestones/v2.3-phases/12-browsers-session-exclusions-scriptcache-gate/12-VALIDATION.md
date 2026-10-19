---
phase: "12"
slug: "browsers-session-exclusions-scriptcache-gate"
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-10-16"
---

# Phase 12 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go test (`go test`) |
| **Config file** | `go.mod` |
| **Quick run command** | `go test -v ./internal/cleaner/browsers/...` |
| **Full suite command** | `CGO_ENABLED=0 go test ./...` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -v ./internal/cleaner/browsers/...`
- **After every plan wave:** Run `CGO_ENABLED=0 go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 12-01-01 | 01 | 1 | BROW-03, VAL-01 | T-12-01 | Inviolable Session Guard: reject Cookies, History, Sessions, Login Data, ~/Library/Safari | unit | `go test -v ./internal/cleaner/browsers -run "TestBrowsers_InviolableSessionShield|TestBrowsers_SafariIsolation"` | ❌ W0 | ⬜ pending |
| 12-01-02 | 01 | 1 | BROW-01, BROW-04 | T-12-02 | Deterministic Process Guard: pgrep -x via MockCommandExecutor with fail-closed semantics | unit | `go test -v ./internal/cleaner/browsers -run "TestBrowsers_ProcessGuard"` | ❌ W0 | ⬜ pending |
| 12-02-01 | 02 | 2 | BROW-01, BROW-02 | T-12-03 | On-disk multi-browser discovery (6 browsers) & dynamic multi-profile traversal | unit | `go test -v ./internal/cleaner/browsers -run "TestBrowsers_OnDiskDetection|TestBrowsers_MultiProfileTraversal"` | ❌ W0 | ⬜ pending |
| 12-02-02 | 02 | 2 | BROW-02, BROW-04 | T-12-04 | Consolidated target presentation (1 per browser) & dedicated ReviewOnly ScriptCache target | unit | `go test -v ./internal/cleaner/browsers -run "TestBrowsers_ConsolidatedTargets|TestBrowsers_ScriptCacheGated"` | ❌ W0 | ⬜ pending |
| 12-03-01 | 03 | 3 | BROW-03, BROW-04 | T-12-05 | Surgical clean execution with atomic pre-clean process revalidation & double-sentinel preservation | unit | `go test -v ./internal/cleaner/browsers -run "TestBrowsers_PreCleanRevalidationRace|TestBrowsers_TCCGracefulDegradation"` | ❌ W0 | ⬜ pending |
| 12-03-02 | 03 | 3 | BROW-01..04, VAL-02 | T-12-06 | Engine registration, config defaults, macOS user cache deduplication & zero-mutation dry-run canary | integration | `go test -v ./internal/engine -run "TestRegisterDefaultCleaners" && go test -v ./internal/deleter -run "TestCoordinator_DryRun_ZeroMutation_Phase12" && CGO_ENABLED=0 go test ./...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/cleaner/browsers/` — package directory and test fixture helpers (`setupBrowserTestFixtures(t, homeDir)`)
- [ ] Hermetic mock directory structures with double sentinels (disposable cache entries vs protected session files)
- [ ] `cleaner.MockCommandExecutor` setup for deterministic process simulation

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| None | All | N/A | All phase behaviors have automated verification via hermetic fixtures and MockCommandExecutor. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

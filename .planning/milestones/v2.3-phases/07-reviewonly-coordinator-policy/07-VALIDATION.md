---
phase: "07"
slug: "reviewonly-coordinator-policy"
status: ready
nyquist_compliant: true
wave_0_complete: false
created: "2026-10-15"
---

# Phase 07 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` standard library |
| **Config file** | `go.mod` |
| **Quick run command** | `go test ./internal/domain ./internal/deleter ./internal/ui ./internal/tui ./internal/mcp` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~2 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/domain ./internal/deleter ./internal/ui ./internal/tui ./internal/mcp`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 07-01-01 | 01 | 1 | POL-01 | T-07-01 | Target exposes ReviewOnly & ReviewReason; EligibleBytes=0 and TTLEligible=false | unit | `go test ./internal/domain -run TestTarget_ReviewOnly` | ❌ W0 | ⬜ pending |
| 07-01-02 | 01 | 1 | POL-02 | T-07-02 | Coordinator Stage 1 drops ReviewOnly before hooks/sinks, fills SkippedReviewOnly | unit | `go test ./internal/deleter -run TestCoordinator_ReviewOnly` | ❌ W0 | ⬜ pending |
| 07-01-03 | 01 | 1 | POL-02 | T-07-03 | 100% ReviewOnly batch returns Result with SkippedReviewOnly, err == nil | unit | `go test ./internal/deleter -run TestCoordinator_AllReviewOnlyBatch` | ❌ W0 | ⬜ pending |
| 07-02-01 | 02 | 2 | POL-04 | T-07-04 | DryRun enforces zero side effects on cleaners, trash, hooks, scheduler, history | unit | `go test ./internal/deleter -run TestCoordinator_DryRun_ZeroMutation` | ❌ W0 | ⬜ pending |
| 07-02-02 | 02 | 2 | POL-04 | T-07-05 | Symlink to protected path rejected in Stage 1 canonical resolution | unit | `go test ./internal/deleter -run TestCoordinator_BlacklistSymlink` | ❌ W0 | ⬜ pending |
| 07-02-03 | 02 | 2 | POL-04 | T-07-06 | Subprocess execution uses exec.CommandContext direct argv slice, preventing shell injection | unit | `go test ./internal/cleaner/... -run TestProviders_NoShellInjection` | ❌ W0 | ⬜ pending |
| 07-03-01 | 03 | 3 | POL-03 | T-07-07 | CLI scan table prefixes ⊙ and verbose shows review reason; clean report shows notice | unit | `go test ./cmd ./internal/ui -run "TestFormatTable_ReviewOnly|TestReport_ReviewNotice"` | ❌ W0 | ⬜ pending |
| 07-03-02 | 03 | 3 | POL-03 | T-07-08 | TUI renders [⊙], blocks space toggle, ignores mass selection (a, m, l) | unit | `go test ./internal/tui -run TestTUI_ReviewOnly` | ❌ W0 | ⬜ pending |
| 07-04-01 | 04 | 3 | POL-03 | T-07-09 | MCP propose_deletion returns status: review_only and eligible_bytes: 0 | unit | `go test ./internal/mcp -run TestMCP_ReviewOnly` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/domain/target_test.go` — stubs/tests for `Target.IsReviewOnly()`, `EligibleBytes` invariants, and constants
- [ ] `internal/deleter/policy_test.go` — tests for `ReviewOnly` rejection, 100% review batch, symlink protection, and DryRun canary assertions
- [ ] `internal/tui/review_test.go` — tests for `[⊙]` rendering and selection exclusion
- [ ] `internal/mcp/review_test.go` — tests for MCP `review_only` status serialization

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| TUI visual appearance | POL-03 | Terminal interactive rendering | Run `devclean` in terminal with review-only targets present, verify `[⊙]` badge and styling |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 5s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** verified 2026-10-15

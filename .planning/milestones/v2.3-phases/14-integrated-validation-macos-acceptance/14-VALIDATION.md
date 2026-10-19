---
phase: "14"
slug: "integrated-validation-macos-acceptance"
status: ready
nyquist_compliant: true
wave_0_complete: false
created: "2026-10-18"
---

# Phase 14 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go test (`go test`) |
| **Config file** | `go.mod` |
| **Quick run command** | `go test -v ./internal/cleaner/macos/... ./internal/cleaner/browsers/... ./internal/cleaner/leftovers/...` |
| **Full suite command** | `CGO_ENABLED=0 go test ./... && ./scripts/verify-coverage.sh && ./scripts/verify-builds.sh` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run quick unit test or task-targeted verify command
- **After every plan wave:** Run `CGO_ENABLED=0 go test ./...`
- **Before `/gsd-verify-work`:** Full suite, coverage script, build script, E2E suite and acceptance test must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 14-01-01 | 01 | 1 | VAL-01, D-12 | T-14-01 | Remediate unit test coverage gaps in macos, browsers, leftovers (>85% each) with sentinels, permission errors, and symlink guard | unit | `go test -cover ./internal/cleaner/macos/... ./internal/cleaner/browsers/... ./internal/cleaner/leftovers/...` | ✅ | ⬜ pending |
| 14-01-02 | 01 | 1 | VAL-02, D-09, D-10, D-11 | T-14-02 | Automated coverage auditor script (>=85% per v2.3 package) and zero-CGO cross-compilation matrix script (darwin/linux amd64/arm64) | integration | `./scripts/verify-coverage.sh && ./scripts/verify-builds.sh` | ❌ W0 | ⬜ pending |
| 14-02-01 | 02 | 2 | VAL-01, VAL-02, D-05..D-08 | T-14-03 | Multichannel E2E integration suite across CLI, TUI headless, MCP, Scheduler, and Coordinator with hermetic t.TempDir() | integration | `go test -v -run "TestWorkflowV23_MultiChannel" ./cmd/...` | ❌ W0 | ⬜ pending |
| 14-02-02 | 02 | 2 | VAL-01, VAL-02, D-06, D-08 | T-14-04 | 4 fault injection stress scenarios (ReviewOnly non-mutation, Process Guard, Symlink Guard, Partial degradation) & canonical zero double-counting | integration | `go test -v -run "TestWorkflowV23_FaultInjection|TestWorkflowV23_ZeroDoubleCounting" ./cmd/...` | ❌ W0 | ⬜ pending |
| 14-03-01 | 03 | 3 | VAL-03, D-03, D-04 | T-14-05 | Darwin host diagnostics in pure Go (TCC/FDA, APFS, protected processes) and acceptance test infrastructure under //go:build acceptance | acceptance | `go test -v -tags=acceptance -run "TestHostDiagnostics" ./test/acceptance/...` | ❌ W0 | ⬜ pending |
| 14-03-02 | 03 | 3 | VAL-03, D-01, D-02, D-13..D-16 | T-14-06 | External Mole dry-run with SPEC fallback, 4 hard failure assertions (t.Fatalf), and generation of MACOS-ACCEPTANCE.md/.json | acceptance | `go test -v -tags=acceptance -run "TestMacOSAcceptance_ParityAndReceipt" ./test/acceptance/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `scripts/verify-coverage.sh` — automated per-package statement coverage verifier (>=85.0%)
- [ ] `scripts/verify-builds.sh` — zero-CGO matrix static compiler (darwin/arm64, darwin/amd64, linux/amd64, linux/arm64)
- [ ] `cmd/workflow_v23_e2e_test.go` — multichannel E2E integration test suite
- [ ] `test/acceptance/` — acceptance package directory with `//go:build acceptance` test suite

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Visual inspection of MACOS-ACCEPTANCE.md | VAL-03, D-01, D-13 | Ensure Markdown tables, badges, callouts, and design divergence rationales format cleanly | Open `.planning/phases/14-integrated-validation-macos-acceptance/MACOS-ACCEPTANCE.md` in markdown viewer |

*All other phase behaviors have automated verification.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 15s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** ready

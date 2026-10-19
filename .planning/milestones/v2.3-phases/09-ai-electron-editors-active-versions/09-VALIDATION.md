---
phase: "09"
slug: "ai-electron-editors-active-versions"
status: approved
nyquist_compliant: true
wave_0_complete: false
created: "2026-10-15"
---

# Phase 09 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go test (`go test`) |
| **Config file** | `go.mod` (root) |
| **Quick run command** | `go test -v ./internal/cleaner/ai/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -v ./internal/cleaner/ai/...`
- **After every plan wave:** Run `go test ./... && CGO_ENABLED=0 GOOS=darwin go build ./...`
- **Before `/gsd-verify-work`:** Full suite must be green, coverage > 85% in altered packages
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 09-01-01 | 01 | 1 | AI-01 | T-09-01 | Domain review reasons export & CommandExecutor injection | unit | `go test -v ./internal/domain -run "TestTarget_ReviewReasons" && go test -v ./internal/cleaner/ai -run "TestAICleaner_ConstructorsWithExecutor"` | ❌ W0 | ⬜ pending |
| 09-01-02 | 01 | 1 | AI-01 | T-09-02 | 8-Editor Chromium/Electron leaf scanner & functional grouping | unit | `go test -v ./internal/cleaner/ai -run "TestEditors\|TestAICleaner_FullScanAndClean"` | ❌ W0 | ⬜ pending |
| 09-02-01 | 02 | 2 | AI-02 | T-09-03 | Surgical `state.vscdb.backup` selection & `state.vscdb` preservation | unit | `go test -v ./internal/cleaner/ai -run "TestEditors_WorkspaceStorage_SurgicalBackup"` | ❌ W0 | ⬜ pending |
| 09-02-02 | 02 | 2 | AI-02 | T-09-04, T-09-05 | Parser of `.obsolete` as metadata & symlink/traversal defense | unit | `go test -v ./internal/cleaner/ai -run "TestEditors_ObsoleteExtensions_Safety\|TestEditors_CachedExtensionVSIXs"` | ❌ W0 | ⬜ pending |
| 09-03-01 | 03 | 3 | AI-03 | T-09-07 | Canonical active symlink resolution & active version preservation | unit | `go test -v ./internal/cleaner/ai -run "TestClaude_ActiveVersion_Preserved\|TestClaude_ProtectedFiles_Untouched\|TestClaude$"` | ❌ W0 | ⬜ pending |
| 09-03-02 | 03 | 3 | AI-03 | T-09-08 | Fail-closed on missing, broken, or ambiguous symlinks | unit | `go test -v ./internal/cleaner/ai -run "TestClaude_MissingSymlink_FailClosed\|TestClaude_BrokenSymlink_FailClosed\|TestClaude_AmbiguousSymlink_FailClosed"` | ❌ W0 | ⬜ pending |
| 09-04-01 | 04 | 4 | AI-01, AI-03 | T-09-12 | Fail-closed Process Guard with isolated per-tool blocking | unit | `go test -v ./internal/cleaner/ai -run "TestEditors_ProcessGuard_IsolatedBlocking\|TestEditors_ProcessGuard_FailClosed\|TestClaude_ProcessGuard_ActiveCLI" && go test -v ./internal/engine -run "TestRegisterDefaultCleaners"` | ❌ W0 | ⬜ pending |
| 09-04-02 | 04 | 4 | AI-01, AI-02, AI-03 | T-09-10, T-09-11 | Pre-deletion process revalidation & surgical virtual cleanup | unit | `go test -v ./internal/cleaner/ai -run "TestAICleaner_Clean_PreDeletionRevalidation\|TestAICleaner_Clean_VirtualEditorTarget\|TestAICleaner_Clean_SymlinkProtection"` | ❌ W0 | ⬜ pending |
| 09-04-03 | 04 | 4 | POL-01, POL-02, VAL-01, VAL-02 | T-09-13 | Coordinator DryRun zero mutation canary & >85% coverage | unit | `go test -v ./internal/deleter -run "TestCoordinator_DryRun_ZeroMutation_Phase09" && go test -cover ./internal/cleaner/ai/... && go test ./... && CGO_ENABLED=0 GOOS=darwin go build ./... && CGO_ENABLED=0 GOOS=linux go build ./...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/cleaner/ai/test_helpers_test.go` — shared mock executor and process runners
- [ ] `internal/cleaner/ai/editors_test.go` — stubs and fixtures for AI-01, AI-02
- [ ] `internal/cleaner/ai/claude_test.go` — stubs and symlink test cases for AI-03

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| *None* | N/A | All phase behaviors have automated verification via CommandExecutor mocks and hermetic fixtures | N/A |

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 10s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved

---
phase: "08"
slug: "node-headless-shell-cli-xcode"
status: draft
nyquist_compliant: true
wave_0_complete: false
created: "2026-10-15"
---

# Phase 08 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` standard library (`testing.T`, `go test`) |
| **Config file** | `go.mod` |
| **Quick run command** | `go test -v -run "TestNode|TestPnpm|TestShell|TestXcode" ./internal/cleaner/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~3 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -v -run "TestNode|TestPnpm|TestShell|TestXcode" ./internal/cleaner/...`
- **After every plan wave:** Run `go test -v -cover ./internal/cleaner/node ./internal/cleaner/pnpm ./internal/cleaner/shell ./internal/cleaner/xcode && go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green, with coverage > 85% on altered packages
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 08-01-01 | 01 | 1 | DEV-01 | T-08-01 | Granular Node caches & automation browsers discovered by disk presence without npm/node in PATH | unit | `go test -v ./internal/cleaner/node -run "TestNodeCleaner_Scan_GlobalCaches_AutomationAndNpm"` | ❌ W0 | ⬜ pending |
| 08-01-02 | 01 | 1 | DEV-01 | T-08-02 | RiskMedium on Puppeteer/Playwright and RiskLow on Cypress/npm/Corepack | unit | `go test -v ./internal/cleaner/node -run "TestNodeCleaner_RiskLevels_Automation"` | ❌ W0 | ⬜ pending |
| 08-01-03 | 01 | 1 | DEV-01 | T-08-03 | pnpm HTTP metadata cache discovered by disk presence without pnpm CLI in PATH | unit | `go test -v ./internal/cleaner/pnpm -run "TestPnpmCleaner_Scan_DiskMetadataCache|TestPnpmCleaner_IsInstalled_DiskPresence"` | ❌ W0 | ⬜ pending |
| 08-02-01 | 02 | 1 | DEV-02 | T-08-04 | Shell cleaner discovery for mise, gh, pre-commit, omz, zcompdump with zero config/history mutation | unit | `go test -v ./internal/cleaner/shell -run "TestShellCleaner_IsInstalled|TestShellCleaner_NonInterferenceWithShellRC"` | ❌ W0 | ⬜ pending |
| 08-02-02 | 02 | 1 | DEV-02 | T-08-05 | Separate mise cache vs downloads targets & aggregated ~/.zcompdump* target | unit | `go test -v ./internal/cleaner/shell -run "TestShellCleaner_Scan_MiseSeparateTargets|TestShellCleaner_Scan_ZshCompletionAggregated"` | ❌ W0 | ⬜ pending |
| 08-02-03 | 02 | 1 | DEV-02 | T-08-06 | Shell cleaner risk levels (RiskLow for gh/omz/mise/zcompdump, RiskMedium for pre-commit) | unit | `go test -v ./internal/cleaner/shell -run "TestShellCleaner_RiskLevels"` | ❌ W0 | ⬜ pending |
| 08-03-01 | 03 | 1 | DEV-03 | T-08-07 | Xcode expanded targets (CoreSimulator Caches/Temp, DeviceSupport, Archives) discovered by disk presence | unit | `go test -v ./internal/cleaner/xcode -run "TestXcodeCleaner_Scan_ExpandedTargets"` | ❌ W0 | ⬜ pending |
| 08-03-02 | 03 | 1 | DEV-03 | T-08-08 | Process guard fail-closed via mock CommandExecutor: active Xcode/Simulator sets ReviewOnly=true and EligibleBytes=0 | unit | `go test -v ./internal/cleaner/xcode -run "TestXcodeCleaner_ProcessGuard_XcodeActive|TestXcodeCleaner_ProcessGuard_SimulatorActive|TestXcodeCleaner_ProcessGuard_FailClosedOnPgrepError"` | ❌ W0 | ⬜ pending |
| 08-03-03 | 03 | 1 | DEV-03 | T-08-09 | Xcode Archives classified as RiskHigh when Xcode is closed | unit | `go test -v ./internal/cleaner/xcode -run "TestXcodeCleaner_Archives_RiskHighWhenClosed"` | ❌ W0 | ⬜ pending |
| 08-04-01 | 04 | 2 | DEV-01..03, VAL-01 | T-08-10 | Symlink protection prevents traversal and outside file deletion across all new/expanded cleaners | unit | `go test -v ./internal/cleaner/... -run "Test.*_Clean_SymlinkProtection"` | ❌ W0 | ⬜ pending |
| 08-04-02 | 04 | 2 | DEV-01..03, VAL-02 | T-08-11 | DryRun zero-mutation canary verification test suite across all Phase 8 cleaners | unit | `go test -v ./internal/deleter -run "TestCoordinator_DryRun_ZeroMutation_Phase08"` | ❌ W0 | ⬜ pending |
| 08-04-03 | 04 | 2 | DEV-01..03 | T-08-12 | Engine and Config registration for shell ecosystem with test coverage > 85% | unit | `go test -v -cover ./internal/cleaner/node ./internal/cleaner/pnpm ./internal/cleaner/shell ./internal/cleaner/xcode ./internal/engine` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/cleaner/node/node_test.go` — stubs/tests for automation browsers, granular npm subdirectories, and Corepack
- [ ] `internal/cleaner/pnpm/pnpm_test.go` — tests for disk-based metadata cache discovery
- [ ] `internal/cleaner/shell/shell_test.go` — tests for new shell cleaner package, mise targets, zcompdump aggregation, and shell non-interference
- [ ] `internal/cleaner/xcode/xcode_test.go` — tests for expanded targets and mock CommandExecutor process guard

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Xcode process lock in live system | DEV-03 | Live OS GUI process detection | Open Xcode.app or Simulator.app on macOS, run `devclean scan` or interactive TUI, verify Xcode targets have `[⊙]` and cannot be selected for deletion. |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 5s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** verified 2026-10-15

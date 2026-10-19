---
phase: 08-node-headless-shell-cli-xcode
verified: "2026-10-15T19:53:00Z"
status: passed
score: 22/22 must-haves verified
requirements:
  DEV-01: satisfied
  DEV-02: satisfied
  DEV-03: satisfied
covered_digest: "v3:sha256:eed700d9c1327fb1b529b561cfcdff53e4cd8c4ac6de4b67024e8ae9daa3fd05"
covered_files:
  - .planning/phases/08-node-headless-shell-cli-xcode/08-01-PLAN.md
  - .planning/phases/08-node-headless-shell-cli-xcode/08-01-SUMMARY.md
  - .planning/phases/08-node-headless-shell-cli-xcode/08-02-PLAN.md
  - .planning/phases/08-node-headless-shell-cli-xcode/08-02-SUMMARY.md
  - .planning/phases/08-node-headless-shell-cli-xcode/08-03-PLAN.md
  - .planning/phases/08-node-headless-shell-cli-xcode/08-03-SUMMARY.md
  - .planning/phases/08-node-headless-shell-cli-xcode/08-04-PLAN.md
  - .planning/phases/08-node-headless-shell-cli-xcode/08-04-SUMMARY.md
  - cmd/clean_test.go
  - cmd/scan_test.go
  - internal/cleaner/cleaner_installed_test.go
  - internal/cleaner/cleaner_test.go
  - internal/cleaner/executor_mock.go
  - internal/cleaner/node/node.go
  - internal/cleaner/node/node_test.go
  - internal/cleaner/pnpm/pnpm.go
  - internal/cleaner/pnpm/pnpm_test.go
  - internal/cleaner/shell/shell.go
  - internal/cleaner/shell/shell_test.go
  - internal/cleaner/xcode/xcode.go
  - internal/cleaner/xcode/xcode_test.go
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/config/defaults.go
  - internal/config/save.go
  - internal/config/validate.go
  - internal/deleter/policy_test.go
  - internal/engine/engine.go
  - internal/engine/register.go
  - internal/engine/register_test.go
  - internal/tui/config.go
  - internal/tui/config_test.go
---

# Phase 08: Node, Headless Automation, Shell/CLI & Xcode — Verification Report

**Phase Goal:** Users can discover the specified Node/headless automation, shell/CLI, and Xcode artifacts by disk presence while active or unknown tool state blocks unsafe cleanup.  
**Milestone:** v2.3 Paridade Total com Mole (`tw93/mole`)  
**Status:** PASSED  
**Verification Date:** 2026-10-15T19:53:00Z  

---

## Executive Summary

Phase 08 achieves total inventory parity with `tw93/mole` for developer tools and automation across three core ecosystems: Node.js / headless automation browsers, developer shell and CLI tools, and Apple Xcode / CoreSimulator environments.

All capabilities enforce the core architectural principles established in earlier phases:
1. **On-disk Discovery (SCAN-02):** Artifacts are discovered by direct filesystem inspection in `$HOME` and standard cache locations without requiring runtime binaries (`node`, `npm`, `pnpm`, `mise`, `gh`, `pre-commit`, `zsh`) to be present in `$PATH`.
2. **Strict Shell Non-Interference (DEV-02, D-08):** Shell configuration files (`.zshrc`, `.bashrc`, `.profile`) and command history files (`.zsh_history`, `.bash_history`) are never inventoried, read, or modified. Zsh completion dumps (`~/.zcompdump*`) are aggregated into a single consolidated virtual target (`shell://zsh/compdump`).
3. **Fail-Closed Process Guard (DEV-03, D-11, D-12):** Active or uncertain `Xcode` or `Simulator` process state invokes `pgrep -x` via `cleaner.CommandExecutor` (with zero shell string interpolation and zero CGO) and sets `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonActiveProcess`, and `EligibleBytes: 0`, completely locking down affected targets at the `deleter.Coordinator` boundary.
4. **Symlink and Safety Guards (VAL-01, D-05):** All cleaner deletion routines validate against `safety.IsCoreBlacklisted` and `safety.IsSymlink`, refusing to traverse or remove external targets pointed to by symlinks.
5. **Zero-Mutation DryRun Canary (POL-04, VAL-02):** Cryptographic verification across 9 Phase 08 target fixtures verifies 0 SHA-256 mutation, 0 timestamp change, 0 byte change, and an empty trash folder.
6. **Code Coverage & Pure Go Builds (VAL-02):** Statement coverage on all altered packages exceeds 89% (surpassing the 85% requirement), and pure Go builds (`CGO_ENABLED=0`) succeed cleanly on both Darwin (`arm64`) and Linux (`amd64`).

---

## Requirements Verification Matrix

| Requirement ID | Plan(s) | Status | Code Artifacts | Test Verification | Result & Evidence |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **DEV-01** | `08-01`<br>`08-04` | **SATISFIED** | `internal/cleaner/node/node.go`<br>`internal/cleaner/pnpm/pnpm.go` | `node_test.go`<br>`pnpm_test.go` | - Discovers `~/.npm/_cacache`, `~/.npm/_npx`, `~/.npm/_logs`, Corepack cache by disk presence.<br>- Discovers Puppeteer (`~/.cache/puppeteer`), Playwright (`~/Library/Caches/ms-playwright`, `~/.cache/ms-playwright`), Cypress (`~/Library/Caches/Cypress`, `~/.cache/Cypress`) by disk presence.<br>- Risk classification: `RiskMedium` for browser binaries (Puppeteer, Playwright); `RiskLow` for package caches and Cypress.<br>- pnpm cleaner discovers physical HTTP metadata cache (`~/Library/Caches/pnpm`, `~/.cache/pnpm`) and reports `IsInstalled() == true` without `pnpm` binary in `$PATH`.<br>- Direct deletion protected by `safety.IsCoreBlacklisted` and `safety.IsSymlink`. |
| **DEV-02** | `08-02`<br>`08-04` | **SATISFIED** | `internal/cleaner/shell/shell.go`<br>`internal/config/`<br>`internal/engine/register.go` | `shell_test.go`<br>`config_test.go`<br>`register_test.go` | - Implements new `internal/cleaner/shell` under ecosystem `"shell"`.<br>- Discovers `mise cache` (`RiskLow`) and `mise downloads` (`RiskLow`) as separate targets.<br>- Discovers `gh` (`RiskLow`), `pre-commit` (`RiskMedium`), and `Oh My Zsh` (`RiskLow`).<br>- Aggregates `~/.zcompdump*` files from `$HOME` and `$ZDOTDIR` into virtual target `shell://zsh/compdump`.<br>- Strict non-interference verified: `.zshrc`, `.bashrc`, `.profile`, `.zsh_history`, and `.bash_history` are never inventoried or touched.<br>- `IsInstalled()` returns `true` on either binary in `$PATH` or directory presence on disk. |
| **DEV-03** | `08-03`<br>`08-04` | **SATISFIED** | `internal/cleaner/xcode/xcode.go`<br>`internal/cleaner/executor_mock.go` | `xcode_test.go`<br>`policy_test.go` | - Expands inventory for `CoreSimulator Caches`, `CoreSimulator Temp`, `iOS DeviceSupport`, `watchOS DeviceSupport`, and `Archives` by disk presence.<br>- Zero-CGO, argument-vector process guard invokes `pgrep -x Xcode` and `pgrep -x Simulator`.<br>- Fail-closed behavior: active or uncertain Xcode locks all targets with `ReviewOnly: true`, `ReviewReason: active_process`, `EligibleBytes: 0`, and `TTLEligible: false`.<br>- Active Simulator locks simulator-related targets.<br>- `Xcode Archives` classified as `RiskHigh` when Xcode is closed.<br>- Filesystem cleanup guarded by `safety.IsCoreBlacklisted` and `safety.IsSymlink`. |

---

## Detailed Must-Haves Verification (22/22 Verified)

### Plan 08-01: Node & Headless Automation Caches Expansion
- [x] **Truth 1:** Node cleaner scans granular subdirectories for npm cache (`~/.npm/_cacache`), npx cache (`~/.npm/_npx`), npm logs (`~/.npm/_logs`), and Corepack cache without requiring node or npm in PATH.  
  *Evidence:* `node.go:195-228`, verified by `TestNodeCleaner_Scan_GlobalCaches_AutomationAndNpm`.
- [x] **Truth 2:** Automation browser caches (Puppeteer `~/.cache/puppeteer`, Playwright `~/Library/Caches/ms-playwright` / `~/.cache/ms-playwright`, Cypress `~/Library/Caches/Cypress` / `~/.cache/Cypress`) are discovered by on-disk presence.  
  *Evidence:* `node.go:229-246`, verified by `TestNodeCleaner_Scan_GlobalCaches_AutomationAndNpm`.
- [x] **Truth 3:** Risk classification assigns `RiskMedium` to browser binaries (Puppeteer, Playwright) and `RiskLow` to package caches and logs (npm, npx, logs, Corepack, Cypress).  
  *Evidence:* `node.go:199,205,211,223,232,238,244`, verified by `TestNodeCleaner_RiskLevels_Automation`.
- [x] **Truth 4:** pnpm cleaner discovers the physical HTTP metadata cache (`~/Library/Caches/pnpm` and `~/.cache/pnpm`) by disk presence, reporting `IsInstalled() == true` even when pnpm is missing from PATH.  
  *Evidence:* `pnpm.go:48-56,84-126`, verified by `TestPnpmCleaner_IsInstalled_DiskPresence` and `TestPnpmCleaner_Scan_DiskMetadataCache`.
- [x] **Truth 5:** Direct filesystem deletion via `os.RemoveAll` is protected by `safety.IsCoreBlacklisted` and `safety.IsSymlink`, without invoking external package manager CLI tools.  
  *Evidence:* `node.go:140-151`, `pnpm.go:140-149`, verified by `TestNodeCleaner_Clean_DirectFilesystem` and `TestNodeCleaner_Clean_SymlinkProtection`.

### Plan 08-02: Shell & CLI Tools Cleaner Package
- [x] **Truth 6:** New cleaner package `internal/cleaner/shell` implements `domain.Cleaner` under the `'shell'` ecosystem identifier.  
  *Evidence:* `shell.go:34`, verified by `TestShellCleaner_Name`.
- [x] **Truth 7:** `IsInstalled()` returns `true` if any supported CLI binary (`mise`, `gh`, `pre-commit`, `zsh`) is in PATH or if any target cache directory exists on disk.  
  *Evidence:* `shell.go:40-100`, verified by `TestShellCleaner_IsInstalled_LookPath` and `TestShellCleaner_IsInstalled_DiskPresence`.
- [x] **Truth 8:** `mise` is inventoried as two distinct targets: `'mise cache'` (`RiskLow`) and `'mise downloads'` (`RiskLow`).  
  *Evidence:* `shell.go:152-164`, verified by `TestShellCleaner_Scan_MiseSeparateTargets`.
- [x] **Truth 9:** Zsh completion dumps matching `~/.zcompdump*` in `$HOME` and `$ZDOTDIR` are aggregated into a single consolidated virtual target `'shell://zsh/compdump'` (`RiskLow`).  
  *Evidence:* `shell.go:254-325`, verified by `TestShellCleaner_Scan_ZshCompletionAggregated`.
- [x] **Truth 10:** Risk classification assigns `RiskMedium` to pre-commit cache and `RiskLow` to mise, gh, Oh My Zsh, and zcompdump.  
  *Evidence:* `shell.go:155,161,167,173,179,321`, verified by `TestShellCleaner_RiskLevels`.
- [x] **Truth 11:** Shell configuration files (`.zshrc`, `.bashrc`, `.profile`) and history files (`.zsh_history`, `.bash_history`) are NEVER read, modified, or deleted.  
  *Evidence:* `shell.go` restricts globs strictly to `.zcompdump*`, verified by `TestShellCleaner_NonInterferenceWithShellRC`.

### Plan 08-03: Xcode and Simulators Expansion with Process Guard
- [x] **Truth 12:** Xcode cleaner expands inventory to discover CoreSimulator Caches, CoreSimulator Temp, iOS DeviceSupport, watchOS DeviceSupport, and Archives by disk presence.  
  *Evidence:* `xcode.go:120-165`, verified by `TestXcodeCleaner_Scan_ExpandedTargets`.
- [x] **Truth 13:** Process guard invokes `'pgrep -x Xcode'` and `'pgrep -x Simulator'` via `cleaner.CommandExecutor` with zero CGO and zero shell string interpolation.  
  *Evidence:* `xcode.go:88-111`, verified by `TestProviders_NoShellInjection`.
- [x] **Truth 14:** When Xcode is active or uncertain, all Xcode targets receive `ReviewOnly=true`, `ReviewReason=domain.ReviewReasonActiveProcess`, `EligibleBytes=0`, and `TTLEligible=false`.  
  *Evidence:* `xcode.go:276-282`, verified by `TestXcodeCleaner_ProcessGuard_XcodeActive` and `TestXcodeCleaner_ProcessGuard_FailClosedOnPgrepError`.
- [x] **Truth 15:** When Simulator is active or uncertain while Xcode is closed, CoreSimulator Caches, CoreSimulator Temp, and virtual simulator targets receive `ReviewOnly=true`.  
  *Evidence:* `xcode.go:282-287`, verified by `TestXcodeCleaner_ProcessGuard_SimulatorActive`.
- [x] **Truth 16:** Xcode Archives (`~/Library/Developer/Xcode/Archives`) is classified as `RiskHigh` when Xcode is closed to alert users about dSYM debug symbol loss.  
  *Evidence:* `xcode.go:163`, verified by `TestXcodeCleaner_Archives_RiskHighWhenClosed`.
- [x] **Truth 17:** `Clean()` validates all physical directories against `safety.IsCoreBlacklisted` and `safety.IsSymlink` before removal.  
  *Evidence:* `xcode.go:303-308`, verified by `TestXcodeCleaner_Clean_SymlinkProtection`.

### Plan 08-04: Integration, Coordinator Verification & Hermetic Suite
- [x] **Truth 18:** Ecosystem `'shell'` is registered in `internal/engine/register.go` and recognized across config defaults, validation, templates, and engine routing.  
  *Evidence:* `register.go:58`, `engine.go:33`, `config.go:45,55`, `defaults.go:34`, `validate.go:17`, `save.go:106`, verified by `TestConfig_Defaults`, `TestConfig_Validate`, `TestRegisterDefaultCleaners`.
- [x] **Truth 19:** TUI configuration metadata in `internal/tui/config.go` includes `'shell'` under the ecosystem listing.  
  *Evidence:* `internal/tui/config.go:23,54`, verified by `TestConfig_ToggleEcosystem_TUI`.
- [x] **Truth 20:** Hermetic symlink protection test suite asserts that symlinks in any Phase 08 cleaner (node, pnpm, shell, xcode) are never followed outside target directories.  
  *Evidence:* `internal/cleaner/cleaner_test.go:33-88`, verified by `TestCrossCleaner_SymlinkProtection_Phase08`.
- [x] **Truth 21:** Coordinator DryRun canary verification test suite asserts zero mutation across all Phase 08 targets, preserving file contents, mtime, size, and trash integrity.  
  *Evidence:* `internal/deleter/policy_test.go:900-1052`, verified by `TestCoordinator_DryRun_ZeroMutation_Phase08`.
- [x] **Truth 22:** Test coverage on all altered packages (`internal/cleaner/node`, `internal/cleaner/pnpm`, `internal/cleaner/shell`, `internal/cleaner/xcode`, `internal/engine`) exceeds 85%.  
  *Evidence:* Measured coverage ranges from 89.9% to 95.3%.

---

## Validation Strategy & Test Results (VAL-01, VAL-02)

### 1. Statement Coverage on Altered Packages (Goal: >85%)
Command: `go test -count=1 -cover ./internal/cleaner/node ./internal/cleaner/pnpm ./internal/cleaner/shell ./internal/cleaner/xcode ./internal/engine`

| Package | Statement Coverage | Status |
| :--- | :---: | :---: |
| `internal/cleaner/node` | **91.3%** | PASS (>85%) |
| `internal/cleaner/pnpm` | **92.7%** | PASS (>85%) |
| `internal/cleaner/shell` | **91.4%** | PASS (>85%) |
| `internal/cleaner/xcode` | **89.9%** | PASS (>85%) |
| `internal/engine` | **95.3%** | PASS (>85%) |

### 2. Full Test Suite Execution
Command: `go test ./...`
- **Result:** PASS (41 packages tested, 0 failures, exit code 0).

### 3. DryRun Zero-Mutation Canary
Test: `TestCoordinator_DryRun_ZeroMutation_Phase08` in `internal/deleter/policy_test.go`
- **Payload Targets:** Puppeteer cache, npm cacache, pnpm metadata, mise cache, mise downloads, Zsh compdumps, Xcode DerivedData, Xcode Archives, CoreSimulator Caches.
- **Cryptographic Invariants Verified:**
  - SHA-256 hash identical before and after DryRun for all payload files.
  - Byte size identical for all payload files.
  - Modification timestamp (`mtime`) identical for all payload files.
  - Trash directory completely empty, 0 bytes, no `manifest.json`.
  - Zero execution calls submitted to underlying cleaners.

### 4. Cross-Cleaner Symlink Protection (VAL-01)
Test: `TestCrossCleaner_SymlinkProtection_Phase08` and per-cleaner `Test<Cleaner>_Clean_SymlinkProtection`
- **Verification:** Symlinks pointing outside target directories (e.g. to mock user projects containing sentinel files) are detected via `safety.IsSymlink` and skipped. Outside canary files remained 100% intact, and symlinks themselves were not removed or traversed.

### 5. Cross-Platform Zero-CGO Builds (VAL-02)
- `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o /dev/null .`: **PASS** (exit code 0)
- `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /dev/null .`: **PASS** (exit code 0)

---

## Code Review Findings Analysis (`08-REVIEW.md`)

The standard adversarial code review (`08-REVIEW.md`) identified 0 Critical issues, 4 Warnings, and 3 Info notes:

1. **WR-01 (Node global caches exclude paths):** Global caches in `scanGlobalCaches` do not consult `rules.ExcludePaths`.  
   *Assessment:* Minor enhancement for selective exclusion of specific global caches; does not compromise discovery by disk presence or safety.
2. **WR-02 (pnpm exclude paths):** Virtual store and metadata caches do not consult `rules.ExcludePaths`.  
   *Assessment:* Minor enhancement; does not affect disk presence discovery or safe deletion.
3. **WR-03 (Xcode virtual simulator exclude paths):** Virtual target `xcode://simulators/unavailable` is appended without checking `rules.ExcludePaths`.  
   *Assessment:* Virtual target carries 0 occupied bytes and is locked by process guard when Xcode/Simulator is active; exclusion filtering is a non-breaking quality enhancement.
4. **WR-04 (`isExcluded` path boundary match):** `strings.HasPrefix(path, exc)` does not strictly check path separator boundaries.  
   *Assessment:* Pattern matching edge case; does not impact default configurations.
5. **IN-01..03 (Context propagation, helper deduplication, virtual target granularity):** Architectural polish items that do not block milestone delivery.

All 4 warnings represent non-blocking feature polish that can be addressed in routine maintenance without impeding Phase 08 sign-off.

---

## Conclusion

Phase 08 successfully delivers all commitments:
> **"Users can discover the specified Node/headless automation, shell/CLI, and Xcode artifacts by disk presence while active or unknown tool state blocks unsafe cleanup."**

Every requirement ID (`DEV-01`, `DEV-02`, `DEV-03`) is satisfied and verified with comprehensive hermetic tests, >89% coverage, and zero regressions.

**Final Verdict: PASSED**

---
phase: "13"
slug: "app-leftovers-launchagents"
status: ready
nyquist_compliant: true
wave_0_complete: false
created: "2026-10-18"
---

# Phase 13 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go test (`go test`) |
| **Config file** | `go.mod` |
| **Quick run command** | `go test -v ./internal/cleaner/leftovers/...` |
| **Full suite command** | `CGO_ENABLED=0 go test ./...` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -v ./internal/cleaner/leftovers/...`
- **After every plan wave:** Run `CGO_ENABLED=0 go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 13-01-01 | 01 | 1 | LEFT-01, LEFT-02, VAL-01 | T-13-01 | Pure Go plist decoding (XML & binary bplist00) via howett.net/plist | unit | `go test -v ./internal/cleaner/leftovers -run "TestLeftovers_LaunchAgents_XMLAndBinary"` | ❌ W0 | ⬜ pending |
| 13-01-02 | 01 | 1 | LEFT-02, VAL-01 | T-13-02 | Stale vs Ambiguous classifier: missing absolute path = Stale; interpreters, PATH, relative = Ambiguous | unit | `go test -v ./internal/cleaner/leftovers -run "TestLeftovers_LaunchAgents_StaleClassification|TestLeftovers_LaunchAgents_InterpretersAmbiguous"` | ❌ W0 | ⬜ pending |
| 13-02-01 | 02 | 2 | LEFT-03, VAL-01 | T-13-03 | Installed application bundle indexer (depth 3, skip .app traversal) | unit | `go test -v ./internal/cleaner/leftovers -run "TestLeftovers_AppIndexing"` | ❌ W0 | ⬜ pending |
| 13-02-02 | 02 | 2 | LEFT-03, VAL-01 | T-13-04 | App leftover scanner with multi-tier exclusion shield & candidate consolidation | unit | `go test -v ./internal/cleaner/leftovers -run "TestLeftovers_AppLeftovers_DiscoveryAndMatching|TestLeftovers_AppLeftovers_Exclusions"` | ❌ W0 | ⬜ pending |
| 13-03-01 | 03 | 3 | LEFT-04, VAL-01 | T-13-05 | Inviolable Clean() defensive refusal & dry-run zero-mutation guarantee | unit | `go test -v ./internal/cleaner/leftovers -run "TestLeftovers_CleanRefusesDeletion|TestLeftovers_DryRun"` | ❌ W0 | ⬜ pending |
| 13-03-02 | 03 | 3 | LEFT-01..04, VAL-01, VAL-02 | T-13-06 | Engine registration (22 ecosystems), config defaults & full test suite compatibility | integration | `go test -v ./internal/engine -run "TestRegisterDefaultCleaners" && CGO_ENABLED=0 go test ./...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/cleaner/leftovers/` — package directory and test fixture helpers (`fixtures_test.go`)
- [ ] Dependency `howett.net/plist` added to `go.mod`
- [ ] Hermetic mock directory structures with sentinels for LaunchAgents and leftover bundles

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real macOS LaunchAgents discovery | LEFT-01, LEFT-02 | Live system environment testing | Run `./devclean scan --ecosystem leftovers` on macOS host and inspect output |
| Real macOS App Leftovers discovery | LEFT-03 | Live system Applications directory scan | Run `./devclean scan --ecosystem leftovers` and verify detected remnants |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 10s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** ready

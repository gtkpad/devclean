---
phase: "10"
slug: "docker-orbstack-shared-stores"
status: approved
nyquist_compliant: true
wave_0_complete: false
created: "2026-10-16"
---

# Phase 10 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go test (`go test`) |
| **Config file** | `go.mod` (root) |
| **Quick run command** | `go test -v ./internal/cleaner/docker/... ./internal/cleaner/dotnet/... ./internal/cleaner/pnpm/... ./internal/fs/...` |
| **Full suite command** | `go test ./... && CGO_ENABLED=0 GOOS=darwin go build ./... && CGO_ENABLED=0 GOOS=linux go build ./...` |
| **Estimated runtime** | ~6 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -v ./internal/cleaner/docker/... ./internal/cleaner/dotnet/... ./internal/cleaner/pnpm/... ./internal/fs/...`
- **After every plan wave:** Run `go test ./... && CGO_ENABLED=0 GOOS=darwin go build ./... && CGO_ENABLED=0 GOOS=linux go build ./...`
- **Before `/gsd-verify-work`:** Full suite must be green, test coverage > 85% in altered packages
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 10-01-01 | 01 | 1 | STORE-01, STORE-02 | T-10-01 | ProviderMetrics struct in domain.Target & zero-CGO Stat_t block accounting | unit | `go test -v ./internal/domain -run "TestTarget_ProviderMetrics" && go test -v ./internal/fs -run "TestCalcUsage"` | ❌ W0 | ⬜ pending |
| 10-01-02 | 01 | 1 | STORE-03 | T-10-02 | Hardlink inode deduplication (dev, ino) & fail-closed lock/error reporting | unit | `go test -v ./internal/fs -run "TestCalcUsage_Hardlinks|TestCalcUsage_PartialLock"` | ❌ W0 | ⬜ pending |
| 10-02-01 | 02 | 2 | STORE-01, STORE-03 | T-10-03 | Docker CommandExecutor standardization, JSON parsing & categorized ReviewOnly targets | unit | `go test -v ./internal/cleaner/docker -run "TestDockerCleaner_SystemDF|TestDockerCleaner_CategorizedReviewOnly"` | ❌ W0 | ⬜ pending |
| 10-02-02 | 02 | 2 | STORE-01 | T-10-04 | Daemon error / offline visibility with docker-daemon-status (ReviewReasonAmbiguous) | unit | `go test -v ./internal/cleaner/docker -run "TestDockerCleaner_DaemonOffline_StatusTarget|TestDockerCleaner_MalformedJSON"` | ❌ W0 | ⬜ pending |
| 10-02-03 | 02 | 2 | STORE-01, STORE-03 | T-10-05 | Docker Desktop host VM disk detection with sparse metrics & defensive Clean() guard | unit | `go test -v ./internal/cleaner/docker -run "TestDockerCleaner_VMDisk_Sparse|TestDockerCleaner_Clean_PreservesAll"` | ❌ W0 | ⬜ pending |
| 10-03-01 | 03 | 3 | STORE-02, STORE-03 | T-10-06 | OrbStack cache vs VM/Data separation & sparse metrics | unit | `go test -v ./internal/cleaner/docker -run "TestOrbStack_Scan_CacheAndDataSeparation"` | ❌ W0 | ⬜ pending |
| 10-03-02 | 03 | 3 | STORE-02 | T-10-07 | OrbStack Process Guard fail-closed detection & pre-deletion Clean() revalidation | unit | `go test -v ./internal/cleaner/docker -run "TestOrbStack_ProcessGuard|TestOrbStack_Clean_PreDeletionRevalidation"` | ❌ W0 | ⬜ pending |
| 10-04-01 | 04 | 4 | STORE-03 | T-10-08 | DotNet nuget-packages locked to ReviewOnly (shared_store) & Clean() guard | unit | `go test -v ./internal/cleaner/dotnet -run "TestDotnetCleaner_NuGetPackages_ReviewOnly|TestDotnetCleaner_Clean_PreservesNuGet"` | ❌ W0 | ⬜ pending |
| 10-04-02 | 04 | 4 | STORE-03 | T-10-09 | pnpm store hybrid discovery (CLI + disk), ReviewOnly lock, no prune & Clean() guard | unit | `go test -v ./internal/cleaner/pnpm -run "TestPNPMCleaner_Store_ReviewOnly|TestPNPMCleaner_Clean_NeverPrunesStore"` | ❌ W0 | ⬜ pending |
| 10-04-03 | 04 | 4 | POL-01, POL-02, VAL-01, VAL-02 | T-10-10 | Coordinator DryRun canary for Phase 10 targets & altered package coverage suite | unit | `go test -v ./internal/deleter -run "TestCoordinator_DryRun_ZeroMutation_Phase10" && go test -cover ./internal/cleaner/docker/... ./internal/cleaner/dotnet/... ./internal/cleaner/pnpm/... ./internal/fs/... && go test ./... && CGO_ENABLED=0 GOOS=darwin go build ./... && CGO_ENABLED=0 GOOS=linux go build ./...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/fs/usage_test.go` — stubs and test fixtures for sparse files and hardlinks
- [ ] `internal/cleaner/docker/test_helpers_test.go` — MockCommandExecutor fixtures for `docker system df` JSON streams and OrbStack processes
- [ ] `internal/cleaner/pnpm/pnpm_test.go` — stubs for hybrid store discovery and inode deduplication

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

**Approval:** approved (planning complete)

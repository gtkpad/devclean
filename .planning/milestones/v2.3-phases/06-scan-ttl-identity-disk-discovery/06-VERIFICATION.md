---
status: passed
phase: 06-scan-ttl-identity-disk-discovery
requirements:
  - SCAN-01
  - SCAN-02
  - SCAN-03
  - SCAN-04
covered_files:
  - .planning/phases/06-scan-ttl-identity-disk-discovery/06-01-PLAN.md
  - .planning/phases/06-scan-ttl-identity-disk-discovery/06-01-SUMMARY.md
  - .planning/phases/06-scan-ttl-identity-disk-discovery/06-02-PLAN.md
  - .planning/phases/06-scan-ttl-identity-disk-discovery/06-02-SUMMARY.md
  - .planning/phases/06-scan-ttl-identity-disk-discovery/06-03-PLAN.md
  - .planning/phases/06-scan-ttl-identity-disk-discovery/06-03-SUMMARY.md
  - cmd/clean.go
  - cmd/clean_test.go
  - cmd/format.go
  - cmd/format_test.go
  - internal/cleaner/ai/ai_test.go
  - internal/cleaner/ai/claude.go
  - internal/cleaner/ai/editors.go
  - internal/cleaner/ai/huggingface.go
  - internal/cleaner/ai/ollama.go
  - internal/cleaner/bun/bun.go
  - internal/cleaner/bun/bun_test.go
  - internal/cleaner/cleaner_installed_test.go
  - internal/cleaner/cleaner_mtime_test.go
  - internal/cleaner/cocoapods/cocoapods.go
  - internal/cleaner/cpp/cpp.go
  - internal/cleaner/docker/docker.go
  - internal/cleaner/docker/docker_test.go
  - internal/cleaner/dotnet/dotnet.go
  - internal/cleaner/elixir/elixir.go
  - internal/cleaner/flutter/flutter.go
  - internal/cleaner/golang/golang.go
  - internal/cleaner/golang/golang_test.go
  - internal/cleaner/homebrew/homebrew.go
  - internal/cleaner/java/java.go
  - internal/cleaner/nextjs/nextjs.go
  - internal/cleaner/nextjs/nextjs_test.go
  - internal/cleaner/node/node.go
  - internal/cleaner/pnpm/pnpm.go
  - internal/cleaner/pnpm/pnpm_test.go
  - internal/cleaner/python/python.go
  - internal/cleaner/python/python_test.go
  - internal/cleaner/risk.go
  - internal/cleaner/rust/rust.go
  - internal/cleaner/shallow.go
  - internal/cleaner/terraform/terraform.go
  - internal/cleaner/xcode/xcode.go
  - internal/cleaner/xcode/xcode_test.go
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/domain/types.go
  - internal/domain/types_test.go
  - internal/engine/engine.go
  - internal/engine/engine_test.go
  - internal/engine/ownership.go
  - internal/engine/ownership_test.go
  - internal/engine/register.go
  - internal/fs/stat.go
  - internal/fs/stat_test.go
  - internal/tui/model.go
covered_digest: "v3:sha256:d97da64db4514a17d5f2b301d8e2fadb3e2445f0fd634344731754756dba5e97"
---

# Phase 06: Scan, TTL, Identity & Disk Discovery — Verification Report

**Phase Goal**: Users can see every existing target and its occupied bytes regardless of root mtime, executable availability, or empty path configuration, with each byte owned by one canonical target.  
**Milestone**: v2.3 Paridade Total com Mole (tw93/mole)  
**Status**: PASSED  
**Verification Date**: 2026-10-14  

---

## Executive Summary

Phase 06 addresses the root causes of the 17+ GB inventory gap discovered during the tw93/mole benchmark. The entire implementation was verified against live code, hermetic test suites, and strict architectural constraints:
1. **SCAN-01**: `Scan()` in all 18 cleaners was decoupled from root directory modification time (`mtime`). `domain.Target` now models physical disk size (`SizeBytes`) distinctly from cleanup eligibility (`EligibleBytes` and `TTLEligible`). `cmd/clean.go` in `--auto` mode directly consumes `t.TTLEligible`.
2. **SCAN-02**: Cleaners report `IsInstalled() == true` via disk presence detection when toolchain cache directories or project manifests exist, even if the executable is absent from `$PATH`. All presence checks are shallow (`os.Stat` or bounded `os.ReadDir`), ensuring instantaneous startup (<60ms across all 18 cleaners).
3. **SCAN-03**: When `cfg.ScanPaths` is empty, `EffectiveScanPaths()` auto-discovers the 7 canonical folders under `$HOME` (`Documents`, `Projects`, `Developer`, `Code`, `Work`, `repos`, `workspace`) in strict deterministic order with safe empty fallback and immutable configuration.
4. **SCAN-04**: Canonical path ownership and hierarchical byte deduction (`internal/engine/ownership.go`) guarantee that dedicated cleaners take precedence over generic cleaners, symlinks resolve to physical paths, overlapping child bytes are deducted from ancestors with defensive zero clamping, and absorbed duplicates are suppressed.

All unit and integration tests compile cleanly under `CGO_ENABLED=0` and pass without warnings or regressions.

---

## Requirements Verification Matrix

| Requirement ID | Plan(s) | Status | Code Artifacts | Test Suite | Evidence & Falsification Check |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **SCAN-01** | `06-01` | **PASSED** | `internal/domain/types.go`<br>`internal/cleaner/*`<br>`cmd/clean.go`<br>`cmd/format.go` | `internal/domain/types_test.go`<br>`internal/cleaner/cleaner_mtime_test.go`<br>`cmd/clean_test.go`<br>`cmd/format_test.go` | - Root directory mtime manipulation via `os.Chtimes` does not suppress target discovery in any cleaner.<br>- `domain.Target` has `EligibleBytes int64` and `TTLEligible bool`.<br>- When `MaxAgeDays == 0`, `EligibleBytes == SizeBytes` and `TTLEligible == true`.<br>- In `--auto` mode, `cmd/clean.go` filters using `t.TTLEligible`. |
| **SCAN-02** | `06-02` | **PASSED** | `internal/fs/stat.go`<br>`internal/cleaner/shallow.go`<br>`internal/cleaner/*` | `internal/fs/stat_test.go`<br>`internal/cleaner/cleaner_installed_test.go` | - Toolchain cleaners (`dotnet`, `golang`, `pnpm`, `homebrew`, `xcode`, `rust`, `python`, `bun`, `java`) report `IsInstalled() == true` with empty `PATH` when cache exists on disk.<br>- Project cleaners (`cpp`, `flutter`, `terraform`, `cocoapods`, `elixir`, `nextjs`) detect shallow manifests in `ScanPaths`.<br>- Bounded startup: sequential execution of all 18 cleaners' `IsInstalled()` takes <60ms (<100ms threshold). |
| **SCAN-03** | `06-02` | **PASSED** | `internal/config/config.go`<br>`internal/engine/register.go` | `internal/config/config_test.go` | - Auto-discovers 7 canonical folders under `$HOME` in strict order.<br>- Returns `[]string{}` empty slice when none exist (prevents full `$HOME` recursive crawl).<br>- Explicit `ScanPaths` in YAML or CLI flag takes strict precedence.<br>- `Config` struct and YAML remain immutable.<br>- Injected via `RegisterDefaultCleaners` into all project cleaners. |
| **SCAN-04** | `06-03` | **PASSED** | `internal/engine/ownership.go`<br>`internal/engine/engine.go`<br>`internal/tui/model.go` | `internal/engine/ownership_test.go`<br>`internal/engine/engine_test.go` | - `TierDedicated` cleaners beat `TierGeneric` cleaners on identical physical paths.<br>- Hierarchical byte deduction: `parent.SizeBytes = max(0, parent.SizeBytes - child.SizeBytes)`.<br>- Symlinks resolved via `filepath.EvalSymlinks`, circular symlinks degrade safely to `filepath.Clean`.<br>- Subordinate duplicate targets are omitted from final results.<br>- Integrated in `engine.Collect` and TUI `scanDoneMsg`/`scanCompleteMsg`. |

---

## Detailed Goal & Success Criteria Audit

### 1. Root Mtime Independence & TTL Eligibility Separation (SCAN-01)
- **Roadmap Success Criterion 1**: *"A scan reports existing targets and descendant bytes even when a parent is recently modified or a child is old; TTL changes only explicit cleanup eligibility."*
- **Live Code Verification**:
  - `internal/domain/types.go` lines 27-29 define:
    ```go
    SizeBytes     int64     `json:"size_bytes"`
    EligibleBytes int64     `json:"eligible_bytes"`
    TTLEligible   bool      `json:"ttl_eligible"`
    ```
  - Cleaners no longer discard candidates with `if info.ModTime().After(cutoff) { return nil, nil }`. Instead, `SizeBytes` is unconditionally computed via filesystem size utilities, and `TTLEligible` / `EligibleBytes` are computed based on internal file activity or TTL rules.
  - Test verification in `internal/cleaner/cleaner_mtime_test.go`:
    - `TestCleaners_RootMtimeRecent_StillReportedInScan`: Sets root and artifact dir mtime to `time.Now()` with `MaxAgeDays: 14`. Asserts `len(targets) > 0`, `SizeBytes > 0`, `TTLEligible == false`, and `EligibleBytes == 0`.
    - `TestCleaners_MaxAgeZero_AlwaysTTLEligible`: Asserts `TTLEligible == true` and `EligibleBytes == SizeBytes` when `MaxAgeDays == 0`.
  - In `cmd/clean.go`:
    ```go
    func filterByTTL(targets []domain.Target, maxAgeDays int) []domain.Target {
        if maxAgeDays == 0 {
            return targets
        }
        cutoff := time.Now().Add(-time.Duration(maxAgeDays) * 24 * time.Hour)
        var filtered []domain.Target
        for _, t := range targets {
            if t.TTLEligible {
                filtered = append(filtered, t)
            } else if t.Category == "" && t.SizeBytes == 0 && !t.LastAccessed.IsZero() && t.LastAccessed.Before(cutoff) {
                filtered = append(filtered, t)
            }
        }
        return filtered
    }
    ```
  - In `cmd/format.go`: Table output displays actual occupied disk space (`humanSize(t.SizeBytes)`), and verbose mode annotates `(eligible: %s, ttl: %t)` when `EligibleBytes < SizeBytes`.

### 2. Disk Presence Detection for `IsInstalled()` (SCAN-02)
- **Roadmap Success Criterion 2a**: *"An on-disk target is inventoried when its owning executable is absent from PATH..."*
- **Live Code Verification**:
  - `internal/fs/stat.go`:
    ```go
    func DirExists(path string) bool {
        if path == "" { return false }
        info, err := os.Stat(path)
        if err != nil { return false }
        return info.IsDir()
    }
    ```
  - Toolchains check PATH or cache presence:
    - `dotnet`: `exec.LookPath("dotnet") == nil || fs.DirExists(paths.DotNetNuGet)`
    - `golang`: `exec.LookPath("go") == nil || fs.DirExists(modCache) || fs.DirExists(buildCache)`
    - `pnpm`: `exec.LookPath("pnpm") == nil || fs.DirExists(storePath)`
    - `homebrew`: `exec.LookPath("brew") == nil || fs.DirExists(cacheDir)`
    - `xcode`: `exec.LookPath("xcrun") == nil || fs.DirExists(derivedData)`
    - `rust`: `exec.LookPath("cargo") == nil || fs.DirExists(cargoRegistry)`
    - `python`: `exec.LookPath("python") == nil || fs.DirExists(uvCache)`
    - `bun`: `exec.LookPath("bun") == nil || fs.DirExists(cachePath)`
    - `java`: `exec.LookPath("java") == nil || fs.DirExists(gradleCache) || fs.DirExists(m2Cache)`
  - Bounded shallow manifest inspection in `internal/cleaner/shallow.go`:
    - `HasShallowManifest(paths []string, manifests ...string) bool` inspects only root and immediate level-1 subfolders, capped at 50 directory entries per level via `os.ReadDir`, skipping hidden dot-folders.
  - Test verification in `internal/cleaner/cleaner_installed_test.go`:
    - `TestCleaners_IsInstalled_NoBinaryInPath_CacheOnDisk` passed for all 9 toolchains.
    - `TestCleaners_IsInstalled_ManifestInScanPaths` passed for `cpp`, `flutter`, `terraform`, `cocoapods`, `elixir`, and `nextjs`.
    - `TestCleaners_IsInstalled_InstantResponse` passed with <60ms total execution time.

### 3. Canonical ScanPaths Auto-Discovery (SCAN-03)
- **Roadmap Success Criterion 2b**: *"...and empty ScanPaths discovers Documents, Projects, Developer, Code, Work, repos, and workspace in deterministic order."*
- **Live Code Verification**:
  - `internal/config/config.go` lines 290-337:
    - Slice `CanonicalDiscoveryFolders`: `["Documents", "Projects", "Developer", "Code", "Work", "repos", "workspace"]`.
    - `EffectiveScanPaths()` checks existing folders in that exact deterministic order.
    - Each folder is resolved via `filepath.EvalSymlinks` and checked against `safety.IsCoreBlacklisted`.
    - Safe empty fallback: returns empty `[]string{}` if no canonical folder exists, preventing unbounded recursion over `$HOME`.
    - Precedence: if `cfg.ScanPaths` contains entries, discovery is skipped and explicit paths are returned.
    - Immutability: operates dynamically in memory without mutating `cfg.ScanPaths` or writing to config files.
  - `internal/engine/register.go` line 39 passes `cfg.EffectiveScanPaths()` to all project cleaners.
  - Tests in `internal/config/config_test.go` (`DiscoveryOrder`, `EmptyFallback`, `ExplicitPrecedence`, `Immutability`) all pass.

### 4. Canonical Ownership Resolution and Overlapping Target Deduplication (SCAN-04)
- **Roadmap Success Criterion 3**: *"Overlapping or symlinked roots produce one canonical owner and do not double-count descendant bytes or submit duplicate deletion targets."*
- **Live Code Verification**:
  - `internal/engine/ownership.go`:
    - Ecosystem tiers: `TierGeneric = 1` (`"macos"`, `"system"`, `"browsers"`, `"leftovers"`), `TierDedicated = 2` (all specific tool/language cleaners).
    - `ResolveCanonicalOwnership(results []domain.ScanResult) []domain.ScanResult`:
      1. Symlink normalization: resolves targets to physical real path with `filepath.EvalSymlinks`, falling back safely to `filepath.Clean` on error/loops (addressing T-06-05).
      2. Exact collision resolution: higher tier (`TierDedicated`) wins over lower tier (`TierGeneric`). Subordinate targets are marked `omitted` (addressing D-01, D-04).
      3. Direct descendant byte deduction: child bytes are deducted from parent targets with zero clamping:
         ```go
         if parent.target.SizeBytes <= child.initSizeBytes {
             parent.target.SizeBytes = 0
         } else {
             parent.target.SizeBytes -= child.initSizeBytes
         }
         ```
         (addressing D-01, T-06-06).
      4. Multi-level nesting protection: intermediate targets prevent double deduction from ancestors.
      5. Immunity: virtual targets (`t.IsVirtual() == true`) and empty test paths bypass filesystem deduplication.
  - Integration:
    - `engine.Collect(ctx, ch)` in `internal/engine/engine.go` automatically invokes `ResolveCanonicalOwnership` on channel drain or cancellation.
    - `internal/tui/model.go` calls `ResolveCanonicalOwnership` in both `scanDoneMsg` and `scanCompleteMsg` before flattening targets.
  - Tests in `internal/engine/ownership_test.go` and `internal/engine/engine_test.go` all pass.

### 5. Hermetic Test Coverage and Zero-CGO Buildability
- **Roadmap Success Criterion 4**: *"Scan fixtures cover old/new mtime combinations, permission and symlink boundaries, and deterministic ordering while retaining zero-CGO buildability."*
- **Live Execution**:
  - `CGO_ENABLED=0 go test ./...` passed across all 39 packages.
  - `CGO_ENABLED=0 go test -count=1 ./internal/domain ./internal/cleaner/... ./internal/config ./internal/fs ./internal/engine ./cmd` completed with 100% test pass rate without test cache.

---

## Anti-Regressions & Invariants Check

- [x] **No CGO dependency**: All packages build with `CGO_ENABLED=0`.
- [x] **No root mtime drop**: Cleaners never return `nil, nil` due to root directory mtime.
- [x] **No YAML config corruption**: Auto-discovery operates in memory and leaves configuration files untouched.
- [x] **No unconstrained `$HOME` crawl**: Safe fallback returns empty slice if canonical folders are missing.
- [x] **No negative byte underflow**: All deductions clamp defensively to 0.
- [x] **No double deletion**: Subordinate targets with identical paths are omitted from final scan results.
- [x] **Virtual targets untouched**: Docker and virtual URI targets remain unaffected by path resolution.

---

## Conclusion & Verdict

Phase 06 has achieved 100% of its defined requirements (`SCAN-01`, `SCAN-02`, `SCAN-03`, `SCAN-04`), satisfying all success criteria defined in `ROADMAP.md` and locked decisions `D-01` through `D-16`. The inventory gap against tw93/mole is resolved at the foundational layer.

**Final Verdict**: `PASSED`

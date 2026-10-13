# Phase 01: Novos Ecossistemas e Presets de Limpeza — Plan 01-04 Summary

**Executed At:** 2026-10-12  
**Status:** Completed  
**Requirements Covered:** `PRESET-01`, `NFR-01`, `NFR-02`, `ECO-01`, `ECO-02`, `ECO-03`, `ECO-04`, `ECO-05`  
**Decisions Implemented:** `D-01`, `D-02`, `D-03`, `D-04`  
**Threat Mitigations Verified:** `T-01-10`, `T-01-11`, `T-01-12`, `T-01-SC`  

---

## 1. Executive Summary

Plan 01-04 implements Wave 2 of Phase 01, integrating the 5 new cleaners (`cpp`, `flutter`, `terraform`, `cocoapods`, `elixir`) into the central engine and configuration model (expanding from 13 to 18 registered ecosystems), and delivering the complete cleanup presets pipeline (`--preset safe`, `--preset standard`, `--preset aggressive`) on `devclean clean`:
1. **Config & Engine Registration (`internal/config`, `internal/engine`, `cmd/scan`)**: Expanded `EcosystemsConfig` and `HooksConfig` to 18 ecosystems with robust defaults (14 days for local builds/dependencies, 30 days for Terraform), hook duration and platform validations, central registration in `engine.RegisterDefaultCleanersWithExecutor`, routing in `ecosystemConfig(name)`, and TTL zeroing helpers.
2. **Cleanup Presets Pipeline (`cmd/clean.go`)**:
   - `--preset` supports `safe`, `standard`, and `aggressive` with descriptive validation and mutual exclusion against `--force` (D-01).
   - `--preset` implicitly enables `auto = true`, running unattended without requiring redundant flags (D-01).
   - `--preset aggressive` forces `MaxAgeDays = 0` across all 18 ecosystems while keeping reversible trash enabled by default (`trashActive = true`) unless `--permanent` is explicitly provided (D-03).
   - `--preset safe` enforces strict low-risk filtering via `filterSafePreset`, rejecting all targets outside `RiskLow` and discarding any project-local builds or dependencies, retaining only global caches and tool downloads (D-02).
   - `--smart` combines orthogonally with `--preset`, refining survivor targets with heuristic scoring (D-04).
   - Immutable blacklist (`rejectBlacklisted`) is enforced unconditionally across all preset flows (T-01-10).

All tests compile and run hermetically under `CGO_ENABLED=0` with 100% statement coverage on clean preset filter helpers and 88.7% overall package coverage in `cmd`.

---

## 2. Tasks Executed

### Task 1: Registro dos 5 Novos Ecossistemas e Modelo de Configuração (`internal/config`, `internal/engine`, `cmd/scan.go`)
- **TDD Workflow**:
  - **RED**: Updated `internal/config/config_test.go` and created `internal/engine/register_test.go` asserting defaults and registration for all 18 ecosystems.
  - **GREEN**:
    - Expanded `EcosystemsConfig` and `HooksConfig` in `internal/config/config.go` with `CPP`, `Flutter`, `Terraform`, `CocoaPods`, and `Elixir`.
    - Added viper defaults in `internal/config/config.go:Load()`.
    - Defined defaults in `internal/config/defaults.go` (CPP 14d, Flutter 14d, Terraform 30d, CocoaPods 14d, Elixir 14d; 30s hook timeouts).
    - Added hook validation in `internal/config/validate.go` and tests in `validate_test.go`.
    - Registered the 5 cleaners in `internal/engine/register.go` within `RegisterDefaultCleanersWithExecutor`.
    - Mapped configurations in `internal/engine/engine.go:ecosystemConfig()`.
    - Updated `cmd/scan.go` with `zeroAllMaxAgeDays(cfg)` to cover all 18 ecosystems when `--all` is set.
  - **REFACTOR**: Consolidated `engine_test.go` and `register_test.go`.
- **Commit**: `5600501` (`feat(engine): register 5 new cleaners and expand config model to 18 ecosystems`).

### Task 2: Implementação do Pipeline de Presets (`cmd/clean.go` & `cmd/clean_test.go`)
- **TDD Workflow**:
  - **RED**: Added comprehensive test cases in `cmd/clean_test.go`:
    - `TestCleanCmd_PresetValidation`: validation of accepted presets (`safe`, `standard`, `aggressive`) and rejection of invalid presets.
    - `TestCleanCmd_PresetMutuallyExclusiveWithForce`: mutual exclusion of `--preset` and `--force`.
    - `TestCleanCmd_PresetImpliesAuto`: automated inference of `auto = true`.
    - `TestCleanCmd_PresetAggressive_ZerosTTL`: TTL zeroing and trash retention for aggressive preset.
    - `TestCleanCmd_PresetSafe_FiltersLocalBuilds`: verification of `filterSafePreset` excluding project-local categories.
    - `TestCleanCmd_PresetWithSmart`: orthogonal composition of `--preset` and `--smart`.
    - `TestCleanCmd_PresetRespectsBlacklist`: immutable blacklist enforcement.
    - `TestGetMaxAgeDays_All18Ecosystems`: verification of TTL retrieval for all 18 ecosystems.
  - **GREEN**:
    - Added `--preset` flag to `newCleanCmd()` with validation.
    - Enforced mutual exclusion between `--preset` and `--force`.
    - Handled automatic `auto = true` inference when `--preset` is present.
    - Handled `preset == "aggressive"` by zeroing TTLs across all 18 ecosystems and defaulting `useTrash = true` unless `--permanent`.
    - Implemented `filterSafePreset(targets []domain.Target)` strictly filtering out non-`RiskLow` items and local build/dependency directories.
    - Updated `getMaxAgeDays` to resolve TTLs for all 18 ecosystems.
    - Integrated with `rejectBlacklisted` and smart heuristic filtering.
  - **REFACTOR**: Eliminated code duplication between `scan.go` and `clean.go` for TTL zeroing using `zeroAllMaxAgeDays`.
- **Commit**: `b79eda4` (`feat(cmd): add cleanup presets safe, standard, aggressive with safety filters`).

---

## 3. Artifacts Created & Modified

| File | Status | Description |
| :--- | :--- | :--- |
| `internal/config/config.go` | Modified | Added 5 new ecosystems to `EcosystemsConfig`, `HooksConfig`, and viper `SetDefault` calls |
| `internal/config/defaults.go` | Modified | Configured default TTLs (14d builds/deps, 30d terraform) and 30s hook timeouts |
| `internal/config/validate.go` | Modified | Added hook timeout and platform validation for 18 ecosystems |
| `internal/config/config_test.go` | Modified | Updated test assertions to verify all 18 ecosystems |
| `internal/config/validate_test.go` | Modified | Added tests verifying validation on new ecosystem hooks |
| `internal/engine/register.go` | Modified | Registered 5 new ecosystem cleaners (total 18) |
| `internal/engine/register_test.go` | Created | Test suite verifying that 18 cleaners are registered and mapped properly |
| `internal/engine/engine.go` | Modified | Mapped 5 new ecosystems in `ecosystemConfig(name)` |
| `internal/engine/engine_test.go` | Modified | Cleaned up moved registration tests |
| `cmd/scan.go` | Modified | Added `zeroAllMaxAgeDays(cfg)` helper covering all 18 ecosystems for `--all` flag |
| `cmd/scan_test.go` | Modified | Updated `createEmptyConfig` fixture to disable all 18 ecosystems |
| `cmd/clean.go` | Modified | Added `--preset` flag, validation, auto inference, aggressive TTL zeroing, and `filterSafePreset` |
| `cmd/clean_test.go` | Modified | Comprehensive test suite for cleanup presets, mutual exclusions, and 18 ecosystems |

---

## 4. Verification & Quality Gates

### Automated Test Suite
```bash
CGO_ENABLED=0 go test -v -count=1 ./internal/config/... ./internal/engine/... ./cmd/...
```
- `internal/config`: PASS (15 test suites)
- `internal/engine`: PASS (14 test suites)
- `cmd`: PASS (all test suites, 97.1s)

### Code Coverage
- `cmd/clean.go:newCleanCmd`: **95.6%**
- `cmd/clean.go:filterByTTL`: **100.0%**
- `cmd/clean.go:filterSafePreset`: **100.0%**
- `cmd/clean.go:rejectBlacklisted`: **100.0%**
- `cmd/clean.go:getMaxAgeDays`: **100.0%**
- `cmd` package overall: **88.7%** (exceeding >85% requirement)

### Static Analysis
```bash
go vet ./...
```
- 0 warnings, clean pass across entire workspace.

---

## 5. Threat Model & Security Compliance

- **T-01-10 (Bypassing Immutable Blacklist via Aggressive Preset):** Verified. `rejectBlacklisted(targets, bl)` runs unconditionally after preset and smart scoring filters. Blacklisted paths (`.git`, `.env`, `.ssh`, `secrets.json`) cannot be scheduled for deletion under any preset (`TestCleanCmd_PresetRespectsBlacklist`).
- **T-01-11 (Destructive Deletion of Local Projects under Safe Preset):** Verified. `filterSafePreset` enforces that only `RiskLow` targets survive and strictly discards local compilation and dependency directories (`node-modules`, `cmake-build`, `vcpkg-installed`, `flutter-build`, `flutter-dart-tool`, `terraform-local`, `cocoapods-pods`, `elixir-build`, `elixir-deps`, etc.), protecting developer projects (`TestCleanCmd_PresetSafe_FiltersLocalBuilds`).
- **T-01-12 (Malicious Hook Injection or Invalid Configuration):** Verified. `config.Validate()` inspects hook timeouts and supported platforms for all 18 ecosystems before engine execution (`TestValidate_NewHooksValidation`).
- **T-01-SC (Zero CGO Hermeticity):** Verified. All packages compile and execute under `CGO_ENABLED=0`.

---

## 6. Self-Check Verification

- [x] `internal/config/config.go` contains 18 ecosystems in `EcosystemsConfig` and `HooksConfig`
- [x] `internal/config/defaults.go` has default TTLs for all 18 ecosystems
- [x] `internal/config/validate.go` validates hooks for all 18 ecosystems
- [x] `internal/engine/register.go` registers all 18 ecosystem cleaners
- [x] `internal/engine/engine.go` maps all 18 ecosystems in `ecosystemConfig`
- [x] `cmd/scan.go` `--all` zeros TTLs for all 18 ecosystems
- [x] `cmd/clean.go` `--preset` supports `safe`, `standard`, and `aggressive`
- [x] Mutual exclusion between `--preset` and `--force` enforced
- [x] `--preset` implies `auto = true` implicitly
- [x] Safe preset excludes local builds and dependencies
- [x] Aggressive preset zeros TTLs and preserves reversible trash by default
- [x] Smart mode combines orthogonally with presets
- [x] Immutable blacklist enforced unconditionally
- [x] `getMaxAgeDays` maps all 18 ecosystems
- [x] Test coverage in `cmd/clean.go` exceeds 85% (88.7% package, 95.6% newCleanCmd)
- [x] All commits made atomically
- [x] `CGO_ENABLED=0` verified
- [x] `STATE.md` and `ROADMAP.md` left untouched

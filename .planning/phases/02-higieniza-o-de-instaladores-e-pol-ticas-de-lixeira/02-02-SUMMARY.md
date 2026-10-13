# Phase 02: Higienização de Instaladores e Políticas de Lixeira — Plan 02-02 Summary

**Executed At:** 2026-10-13  
**Status:** Completed  
**Requirements Covered:** `TRASH-01`, `TRASH-02`, `NFR-01`, `NFR-02`  
**Decisions Implemented:** `D-09`, `D-10`, `D-11`, `D-12`, `D-14`  
**Threat Mitigations Verified:** `T-02-04`, `T-02-05`, `T-02-06`, `T-02-SC`  

---

## 1. Executive Summary

Plan 02-02 establishes the sustainable governance, atomic persistence, and auto-expiration infrastructure for DevClean's reversible trash (`internal/trash`, `internal/config`):
1. **Tracer / Atomic Manifest Persistence & Orphan Sanitization (`internal/trash/manifest.go`, `trash.go`)**: Rewrote `SaveManifest` to use an atomic tempfile pattern (`os.CreateTemp` in the trash directory, explicit disk flush via `tmpFile.Sync()`, file closure, and atomic `os.Rename`), with cleanup deferral preventing orphaned tempfiles on failure. Added `Manager.SanitizeOrphans(ctx)` to safely clean physical unlisted batch directories in `batches/` left after abrupt process interruptions (`SIGKILL`, power failure) without affecting valid entries or corrupted state (D-11, T-02-04, T-02-06).
2. **Retention Auto-Expiration & Structured Metrics (`internal/trash/trash.go`)**: Implemented `CleanStats` struct and `CleanExpired(ctx context.Context) (CleanStats, error)` to purge entries older than the configured retention period (`retention_days`), physically removing entry data from disk (`os.RemoveAll`), removing empty batch directories, and updating the manifest atomically while returning exact counts of batches purged, entries purged, and bytes freed. Refactored `EnforceRetention(ctx)` to delegate to `CleanExpired` for seamless backward compatibility (D-09, D-11, D-14, TRASH-01).
3. **Strict FIFO/LRU Quota Eviction & Security Defaults v2.2 (`internal/trash/trash.go`, `internal/config`)**:
   - Updated `config.Defaults()` with v2.2 safety defaults: `TrashEnabled: true` (reversible trash enabled by default), `RetentionDays: 30` (30 days safe retention), and `MaxSizeMB: 10240` (10 GB max quota) (D-12).
   - Added `Validate()` rules requiring `RetentionDays >= 1` and `MaxSizeMB >= 100` to prevent invalid or zero-byte configurations.
   - Implemented `QuotaInfo` and `Manager.QuotaStatus() QuotaInfo` for transparent quota inspection with usage percentages and exceeded flags.
   - Refined `Manager.EnforceMaxSize(ctx context.Context) error` to strictly evict oldest entries first (FIFO/LRU based on `MovedAt`) until `TotalSize <= maxSize`, cleaning empty batch folders and persisting updates atomically (D-10, TRASH-02, T-02-05).

All components compile hermetically with `CGO_ENABLED=0` (NFR-01), achieving 86.7% test coverage in `internal/trash` and 94.5% in `internal/config` (NFR-02).

---

## 2. Tasks Executed

### Task 1: Tracer Slice — Escrita Atômica do Manifesto & Saneamento de Órfãos (`manifest.go`, `trash.go`, `trash_test.go`)
- **TDD Workflow**:
  - **RED**: Added `TestSaveManifest_AtomicWrite`, `TestSaveManifest_CleansTempOnError`, `TestSanitizeOrphans_RemovesUnlistedBatches`, `TestSanitizeOrphans_NoBatchesDir`, and `TestSanitizeOrphans_ContextCanceled` in `internal/trash/trash_test.go`.
  - **GREEN**: Implemented atomic `SaveManifest` in `internal/trash/manifest.go` using `os.CreateTemp` + `Sync()` + `Rename()`, and added `Manager.SanitizeOrphans(ctx)` in `internal/trash/trash.go`.
  - **REFACTOR**: Wrapped I/O errors with `%w` and added `TestSanitizeOrphans_CorruptedManifestPreservesDirectories` edge probe to ensure no directories are removed if the manifest cannot be loaded.
- **Commit**: `ba784dc` (`feat(trash): implement atomic manifest persistence and orphan sanitization`).

### Task 2: Auto-Expiração por Retenção & Métricas Estruturadas (`trash.go`, `trash_test.go`)
- **TDD Workflow**:
  - **RED**: Added `TestEnforceRetention_RemovesPhysicalFilesAndEmptyBatches`, `TestCleanExpired_StatsAccuracy`, `TestEnforceRetention_FreshBatchesPreserved`, `TestCleanExpired_EmptyTrash`, and `TestCleanExpired_ContextCanceled` in `internal/trash/trash_test.go`.
  - **GREEN**: Defined `CleanStats` struct, implemented `CleanExpired(ctx)` with physical deletion of entry and empty batch folders, and refactored `EnforceRetention(ctx)` to delegate to `CleanExpired`.
  - **REFACTOR**: Updated `removeEntryData` to automatically remove empty batch directories when individual entries are removed.
- **Commit**: `1d7272c` (`feat(trash): implement retention auto-expiration with structured metrics`).

### Task 3: Gestão de Quota Máxima via FIFO/LRU & Novos Padrões de Segurança (`trash.go`, `defaults.go`, `validate.go`, `trash_test.go`, `config_test.go`)
- **TDD Workflow**:
  - **RED**: Added `TestConfig_DefaultsV22` and `TestValidate_SafetyRules` in `internal/config/config_test.go`, plus `TestQuotaStatus_Calculations`, `TestEnforceMaxSize_StrictFIFOOrder`, and `TestEnforceMaxSize_NoOpWhenMaxSizeZeroOrNegative` in `internal/trash/trash_test.go`.
  - **GREEN**:
    - Updated `Defaults()` in `internal/config/defaults.go` with `TrashEnabled: true`, `RetentionDays: 30`, `MaxSizeMB: 10240`.
    - Added safety validation rules in `internal/config/validate.go` (`RetentionDays >= 1`, `MaxSizeMB >= 100`).
    - Added `QuotaInfo` and `Manager.QuotaStatus() QuotaInfo` in `internal/trash/trash.go`.
    - Refined `Manager.EnforceMaxSize(ctx)` with strict FIFO eviction by `MovedAt` and early no-op when `maxSize <= 0`.
  - **REFACTOR**: Ensured context cancellation is respected prior to no-op checks.
- **Commit**: `836af55` (`feat(trash): implement FIFO quota eviction and v2.2 safety defaults`).

---

## 3. Artifacts Created & Modified

| File | Status | Description |
| :--- | :--- | :--- |
| `internal/trash/manifest.go` | Modified | Atomic manifest writing with `os.CreateTemp`, `Sync()`, and `Rename()` |
| `internal/trash/trash.go` | Modified | Added `SanitizeOrphans`, `CleanStats`, `CleanExpired`, `QuotaInfo`, `QuotaStatus`, and FIFO `EnforceMaxSize` |
| `internal/trash/trash_test.go` | Modified | Test suite for atomic write, orphan sanitization, stats accuracy, QuotaStatus, and strict FIFO eviction |
| `internal/config/defaults.go` | Modified | Updated v2.2 compiled defaults: `TrashEnabled: true`, `RetentionDays: 30`, `MaxSizeMB: 10240` |
| `internal/config/validate.go` | Modified | Added validation for `Safety.RetentionDays >= 1` and `Safety.MaxSizeMB >= 100` |
| `internal/config/config_test.go` | Modified | Added unit tests verifying v2.2 safety defaults and validation constraints |

---

## 4. Verification & Quality Gates

### Automated Test Suite
```bash
CGO_ENABLED=0 go test -v -count=1 ./internal/trash ./internal/config
```
- `internal/trash`: PASS (36 test suites, 0 failures)
- `internal/config`: PASS (23 test suites, 0 failures)
- Repository-wide test suite (`CGO_ENABLED=0 go test -race=false ./...`): PASS (0 regressions across all 37 packages)

### Code Coverage
- `internal/trash`: **86.7%** (target > 85%)
- `internal/config`: **94.5%** (target > 85%)

---

## 5. Next Steps

With Plan 02-02 complete, Phase 02 is ready for:
- **Plan 02-03**: Canonical `devclean trash clean` command with `--expired` and `--all` flags, transparent `trash purge` alias, and enriched `devclean trash list` with quota status and `[EXPIRADO]` indicators (`cmd/trash.go`).
- **Plan 02-04**: CLI command `devclean installer` with Lip Gloss preview, interactive confirmation, and unified `deleter.Coordinator` execution (`cmd/installer.go`).

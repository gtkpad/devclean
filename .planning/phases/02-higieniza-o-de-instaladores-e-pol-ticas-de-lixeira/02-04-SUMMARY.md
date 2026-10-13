# Phase 02: Higienização de Instaladores e Políticas de Lixeira — Plan 02-04 Summary

**Executed At:** 2026-10-13  
**Status:** Completed  
**Requirements Covered:** `TRASH-01`, `TRASH-02`, `NFR-01`, `NFR-02`  
**Decisions Implemented:** `D-09`, `D-10`, `D-13`, `D-14`, `D-15`, `D-16`  
**Threat Mitigations Verified:** `T-02-10`, `T-02-11`, `T-02-12`, `T-02-SC`  

---

## 1. Executive Summary

Plan 02-04 completes the lifecycle integration and CLI harmonization for DevClean's reversible trash system (`internal/deleter`, `cmd`):
1. **Silent Retention & Quota Hook in Coordinator (`internal/deleter/deleter.go`, `internal/trash/trash.go`)**:
   - Integrated Stage 6.1 into `Coordinator.Delete`: when `!opts.DryRun` and `c.trashMgr != nil`, silently triggers `c.trashMgr.EnforceRetention(ctx)` and `c.trashMgr.EnforceMaxSize(ctx)` at the end of each deletion lifecycle (D-09, D-10).
   - Added friendly non-blocking warning via `opts.OnWarning` whenever the configured quota is exceeded: `"Aviso: Lixeira atingiu %s (limite configurado: %s). Lotes mais antigos foram expurgados."` (D-10, T-02-11).
   - Guaranteed that `--dry-run` bypasses retention and eviction routines completely, preserving trashed items (T-02-10).
   - Guarded `CleanExpired` in `internal/trash/trash.go` with early no-op when `retention <= 0`, preventing accidental purges when retention days are unset.
2. **Canonical CLI Command `devclean trash clean` (`cmd/trash.go`)**:
   - Implemented canonical `devclean trash clean` with flags `--expired` (default `true`), `--all` (default `false`), and `--format` (`table|json`) (D-13, D-14).
   - Default behavior safely purges only batches older than `retention_days` (`--expired` implicit), reporting batches purged and bytes freed.
   - Flag `--all` permanently empties the trash, requiring explicit user intent (T-02-10).
3. **Transparent Backward-Compatible Alias `devclean trash purge` (`cmd/trash.go`)**:
   - Converted `devclean trash purge` into a transparent wrapper delegating directly to `trash clean`, maintaining 100% backward compatibility for existing scripts with `--all` and format support (D-13).
4. **Enriched Visual and JSON Inspection in `devclean trash list` (`cmd/trash.go`)**:
   - Displays real-time quota status header in humanized format: `"Lixeira: %s / %s (%.1f%% ocupado)\n\n"` (D-15).
   - Visual badges: marks expired batches with `[EXPIRADO]` and active batches with remaining retention time (e.g. `tempo restante: 18d`).
   - Structured JSON output via `--format json` emitting `quota` (`QuotaInfo`), `total_batches`, and detailed batches with `expired: bool` and `remaining_retention: string` for MCP agents, scripts, and CI pipelines (D-16).

All packages compile hermetically with `CGO_ENABLED=0` (NFR-01), achieving 91.8% coverage in `internal/deleter`, 86.8% in `internal/trash`, and 98.8%+ across `cmd/trash.go` routines (NFR-02).

---

## 2. Tasks Executed

### Task 1: Tracer Slice — Hook de Auto-Expiração Silenciosa e Quota no Coordinator (`deleter.go`, `deleter_test.go`, `trash.go`)
- **TDD Workflow**:
  - **RED**: Added `TestCoordinator_TriggersSilentRetention`, `TestCoordinator_QuotaWarningEmitted`, `TestCoordinator_DryRun_NoRetention`, and `TestCoordinator_NilTrashMgr_NoPanic` in `internal/deleter/deleter_test.go`.
  - **GREEN**: Implemented Stage 6.1 in `Coordinator.Delete` (`internal/deleter/deleter.go`), calling `EnforceRetention(ctx)`, `EnforceMaxSize(ctx)`, inspecting `QuotaStatus()`, and dispatching formatted warning on `opts.OnWarning`. Added `m.retention <= 0` early return in `internal/trash/trash.go:CleanExpired`.
  - **REFACTOR**: Verified zero regressions across the entire deletion lifecycle and scheduler runs.
- **Commit**: `0040777` (`feat(deleter): add silent retention hook and quota warning in coordinator stage 6`).

### Task 2: Subcomando Canônico `devclean trash clean`, Alias `purge` e Enriquecimento de `list` (`cmd/trash.go`, `cmd/trash_test.go`)
- **TDD Workflow**:
  - **RED**: Added comprehensive test suite in `cmd/trash_test.go` covering `TestTrashCleanCmd_ExpiredOnlyDefault`, `TestTrashCleanCmd_AllFlag`, `TestTrashCleanCmd_JsonOutput`, `TestTrashPurgeCmd_AliasEquivalence`, `TestTrashListCmd_QuotaAndExpiredBadge`, `TestTrashListCmd_Json`, and `TestTrash_InvalidFormat`.
  - **GREEN**:
    - Registered `newTrashCleanCmd` and `newTrashPurgeCmd` alongside `undo` and `list` in `newTrashCmd` (4 subcommands).
    - Implemented `newTrashCleanCmd` with `--expired`, `--all`, `--format`, humanized table output, and JSON metrics.
    - Converted `newTrashPurgeCmd` into transparent wrapper pointing to `cleanCmd`.
    - Enriched `newTrashListCmd` with quota header, `[EXPIRADO]` badge, remaining retention calculations, and structured JSON output.
    - Updated output streams to `cmd.OutOrStdout()` for test isolation.
  - **REFACTOR**: Validated table format string alignments and verified 98.8%+ coverage across all trash commands.
- **Commit**: `1731355` (`feat(cmd): add canonical trash clean subcommand, purge alias, and quota visualization in trash list`).

---

## 3. Artifacts Created & Modified

| File | Status | Description |
| :--- | :--- | :--- |
| `internal/deleter/deleter.go` | Modified | Added Stage 6.1 silent retention hook, FIFO quota enforcement, and `OnWarning` feedback |
| `internal/deleter/deleter_test.go` | Modified | Unit tests covering silent retention, quota warnings, dry-run safety, and nil trashMgr |
| `internal/trash/trash.go` | Modified | Added `retention <= 0` guard in `CleanExpired` preventing unintended purges |
| `cmd/trash.go` | Modified | Canonical `trash clean`, transparent `trash purge` alias, and enriched `trash list` with quota/badges |
| `cmd/trash_test.go` | Modified | Test suite covering clean default/all, JSON outputs, purge equivalence, quota badges, and metadata |

---

## 4. Verification & Quality Gates

### Automated Test Suite
```bash
CGO_ENABLED=0 go test -v -count=1 ./internal/deleter ./cmd -run "TestCoordinator|TestTrash"
```
- `internal/deleter`: PASS (10 test suites, 0 failures)
- `cmd`: PASS (13 trash test suites, 0 failures)
- Repository-wide test suite (`CGO_ENABLED=0 go test -race=false ./...`): PASS (0 regressions across all 37 packages)

### Code Coverage
- `internal/deleter`: **91.8%** of statements (target > 85%)
- `internal/trash`: **86.8%** of statements (target > 85%)
- `cmd/trash.go`:
  - `newTrashCmd`: **100.0%**
  - `newTrashUndoCmd`: **100.0%**
  - `newTrashListCmd`: **98.8%**
  - `newTrashCleanCmd`: **95.7%**
  - `newTrashPurgeCmd`: **100.0%**
  - `formatAge`: **100.0%**

---

## 5. Next Steps

With Plan 02-04 complete, all plans in Wave 2 and Phase 02 (`02-01`, `02-02`, `02-03`, `02-04`) are executed:
- Requirements `INST-01`, `INST-02`, `TRASH-01`, `TRASH-02`, `NFR-01`, `NFR-02`, and `NFR-03` are fully satisfied.
- The phase is ready for Wave 2 reconciliation and milestone progression by the orchestrator.

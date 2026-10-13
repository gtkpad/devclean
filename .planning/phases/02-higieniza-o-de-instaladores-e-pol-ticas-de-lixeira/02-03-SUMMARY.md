# Phase 02: Higienização de Instaladores e Políticas de Lixeira — Plan 02-03 Summary

**Executed At:** 2026-10-13  
**Status:** Completed  
**Requirements Covered:** `INST-02`, `NFR-01`, `NFR-02`, `NFR-03`  
**Decisions Implemented:** `D-05`, `D-06`, `D-07`, `D-08`  
**Threat Mitigations Verified:** `T-02-07`, `T-02-08`, `T-02-09`, `T-02-SC`  

---

## 1. Executive Summary

Plan 02-03 establishes the CLI interface and deletion integration for DevClean's orphan installer cleaner (`internal/installer`, `cmd`):
1. **`domain.Cleaner` Adapter for Installers (`internal/installer/cleaner.go`)**: Implemented the `domain.Cleaner` interface for the `"installer"` ecosystem (`Name() == "installer"`, `IsInstalled() == true`), mapping discovered `Candidate` entities to `domain.Target` structures (`Risk: RiskLow`, `Category: "installer"`). Implemented safe direct deletion in `Clean(ctx, targets)` using `devfs.ChmodRecursive(path, 0755, 0666)` prior to `os.RemoveAll`, unlocking read-only trees and supporting `--permanent` deletion or operations with trash disabled without relying on CGO (D-07, NFR-01, NFR-03).
2. **CLI Subcommand `devclean installer` (`cmd/installer.go`, `cmd/root.go`)**:
   - Registered `devclean installer` in root Cobra command tree.
   - Comprehensive flag set: `--min-age` (default `"24h"`, parsed via duration/days helper, `"0"` to disable), `--min-size` (e.g. `"50MB"`, `"1GB"`), `--path` (custom single directory), `--dry-run` (preview without deletion), `-y`/`--yes` (bypass interactive prompt), `--trash` (force move to trash), `--permanent` (permanent deletion without trash), and `--format` (`table` or `json`) (D-05, D-06).
   - Strict flag validations: enforces mutual exclusion between `--trash` and `--permanent`, validates format against `table|json`, and verifies valid duration/size strings.
3. **UX, Lip Gloss Table & Strict TTY Confirmation (`cmd/installer.go`)**:
   - Rendered visual tables using Lip Gloss (`table.New()`) styled with color-coded headers and summary footer (file count and reclaimable space).
   - Structured JSON output (`--format json`) returning `candidates` list, `total_files`, and `total_size_bytes`.
   - Strict TTY check: when run without `--yes` and without `--dry-run`, verifies `term.IsTerminal`; aborts with explanatory error (`"confirmação interativa indisponível sem TTY; use --yes para prosseguir"`) if non-interactive, preventing headless pipeline lockups (D-05, T-02-07).
   - Interactive prompt `[y/N]` reading from configurable reader `installerStdinReader`, gracefully handling user cancellation (`"Operação cancelada pelo usuário."`).
4. **Unified Deletion Coordinator Integration (`cmd/installer.go`)**:
   - Integrated with `deleter.Coordinator`, supplying `installerCleanerProvider` with `installer.NewCleaner(...)`.
   - Dispatches targets to `coord.Delete(ctx, map[string][]domain.Target{"installer": targets}, opts)` with full 6-stage lifecycle, double blacklist protection (`.git`, `.env`, `.ssh`), and audit logging to `devclean history` (D-07, D-08, NFR-03, T-02-08).

All packages compile hermetically with `CGO_ENABLED=0` (NFR-01), achieving 89.3% test coverage in `internal/installer` and 93.1% in `cmd/installer.go` (NFR-02).

---

## 2. Tasks Executed

### Task 1: Tracer Slice — Implementação de `domain.Cleaner` para Instaladores (`cleaner.go`, `cleaner_test.go`)
- **TDD Workflow**:
  - **RED**: Created `internal/installer/cleaner_test.go` covering `TestCleaner_Name`, `TestCleaner_IsInstalled`, `TestCleaner_Scan`, `TestCleaner_Scan_ContextCanceled`, `TestCleaner_Clean_DirectDeletionWithChmod`, and `TestCleaner_Clean_ContextCanceled`.
  - **GREEN**: Implemented `internal/installer/cleaner.go` with `NewCleaner(opts)`, `Name()`, `IsInstalled()`, `Scan()`, and `Clean()` with `fs.ChmodRecursive` POSIX unlocking before `os.RemoveAll`.
  - **REFACTOR**: Verified atomic deletion loop, context cancellation handling, and zero regressions across existing archive, detector, and scanner tests.
- **Commit**: `cedf252` (`feat(installer): implement domain.Cleaner adapter with chmod-unlocked deletion`).

### Task 2: Subcomando CLI `devclean installer` com Preview Lip Gloss, Confirmação TTY e Integração ao Coordinator (`cmd/installer.go`, `cmd/installer_test.go`, `cmd/root.go`)
- **TDD Workflow**:
  - **RED**: Created `cmd/installer_test.go` testing flag validation, `--dry-run`, `--format json`, interactive confirmation (`"y\n"`), cancellation (`"n\n"`), non-TTY error enforcement, immutable blacklist protection, `--permanent` deletion, `--trash` flag, and cleaner provider dispatch.
  - **GREEN**:
    - Implemented `cmd/installer.go` with `newInstallerCmd()`, Lip Gloss table rendering (`renderInstallerTable`), JSON output serialization, TTY check via `installerIsTerminal`, interactive prompt via `installerStdinReader`, and `deleter.Coordinator` integration.
    - Registered `newInstallerCmd()` in `cmd/root.go`.
  - **REFACTOR**: Added `SilenceUsage: true` on `newInstallerCmd` and added edge cases in `cmd/installer_test.go` for empty JSON and cleaner provider mapping to bring `cmd/installer.go` coverage to >93%.
- **Commit**: `81d6205` (`feat(cmd): add installer subcommand with table preview, TTY check, and deletion coordinator`).

---

## 3. Artifacts Created & Modified

| File | Status | Description |
| :--- | :--- | :--- |
| `internal/installer/cleaner.go` | Created | `domain.Cleaner` adapter with `Scan()` mapping and chmod-unlocked `Clean()` |
| `internal/installer/cleaner_test.go` | Created | Unit tests verifying Name, IsInstalled, Scan mapping, context cancellation, and read-only chmod deletion |
| `cmd/installer.go` | Created | CLI command `devclean installer` with Lip Gloss table, JSON output, TTY check, and Coordinator integration |
| `cmd/installer_test.go` | Created | Test suite covering flags, dry-run, JSON, TTY mock, interactive confirm/cancel, blacklist protection, and permanent deletion |
| `cmd/root.go` | Modified | Registered `newInstallerCmd()` in root command tree |

---

## 4. Verification & Quality Gates

### Automated Test Suite
```bash
CGO_ENABLED=0 go test -v -count=1 ./internal/installer ./cmd -run "TestInstaller|TestCleaner"
```
- `internal/installer`: PASS (24 test suites, 0 failures)
- `cmd`: PASS (11 installer test suites, 0 failures)
- Repository-wide test suite (`CGO_ENABLED=0 go test -race=false ./...`): PASS (0 regressions across all 37 packages)

### Code Coverage
- `internal/installer`: **89.3%** (target > 85%)
- `cmd/installer.go`: **93.1%** (target > 85%)

---

## 5. Next Steps

With Plan 02-03 complete, Phase 02 is ready for:
- **Plan 02-04**: CLI command `devclean trash clean` (canonical cleaning with `--expired` and `--all`), transparent `trash purge` alias, enriched `devclean trash list` with quota status, and JSON output support (`cmd/trash.go`).

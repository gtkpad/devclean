# Phase 02: Higienização de Instaladores e Políticas de Lixeira — Plan 02-01 Summary

**Executed At:** 2026-10-13  
**Status:** Completed  
**Requirements Covered:** `INST-01`, `NFR-01`, `NFR-02`, `NFR-03`  
**Decisions Implemented:** `D-01`, `D-02`, `D-03`, `D-04`, `D-08`  
**Threat Mitigations Verified:** `T-02-01`, `T-02-02`, `T-02-03`, `T-02-SC`  

---

## 1. Executive Summary

Plan 02-01 establishes Wave 1 of Phase 02, delivering the core orphan installer detection engine (`internal/installer`) and the configuration foundation (`internal/config`):
1. **In-Memory Archive Inspector (`internal/installer/archive.go`)**: Zero disk extraction scanner checking Central Directory headers for `.zip` and up to 100 headers in `.tar.gz`/`.tgz` via streaming readers. Validates direct installers, macOS app bundles (`.app/Contents/MacOS/`), and magic bytes for binaries without extensions (ELF, Mach-O 32/64-bit and reversed, PE), while rejecting source code false positives (`package.json`, `go.mod`, `Cargo.toml`, `pom.xml`, `build.gradle`, `CMakeLists.txt`) (D-01, T-02-01).
2. **Installer Detector (`internal/installer/detector.go`, `types.go`)**: Deterministic classifier identifying direct installer extensions (`.dmg`, `.pkg`, `.iso`, `.deb`, `.rpm`, `.appimage`, `.exe`, `.msi`), discarding incomplete browser downloads (`.crdownload`, `.part`, `.download`, `.aria2`), and delegating archive formats to in-memory inspection (D-04, D-03).
3. **Depth-Limited Scanner & Configuration (`internal/installer/scanner.go`, `internal/config`)**:
   - `Scanner` walks user directories (`~/Downloads`, `~/Desktop`) with relative depth limiter (`max_depth: 2`, D-02).
   - Inviolability of the Immutable Blacklist: paths matching `.git`, `.env`, `.ssh` return `filepath.SkipDir` immediately (NFR-03, T-02-03).
   - Symlink isolation: entries with `ModeSymlink != 0` are strictly ignored to prevent loops and directory escapes (T-02-02).
   - macOS `.app` bundles: calculates aggregate folder size and skips internal tree traversal (`filepath.SkipDir`).
   - Age and size filters: supports minimum age (`--min-age`, default 24h) and minimum size (`--min-size`).
   - Configuration: `InstallerConfig` added to `Config` with defaults (`~/Downloads`, `~/Desktop`, `max_depth: 2`) and validation (`max_depth <= 5`).

All packages compile hermetically with `CGO_ENABLED=0` (NFR-01) with 89.0% test coverage in `internal/installer` and 94.5% in `internal/config` (NFR-02).

---

## 2. Tasks Executed

### Task 1: In-Memory Archive Inspector (`internal/installer/archive.go`, `archive_test.go`)
- **TDD Workflow**:
  - **RED**: Created `internal/installer/archive_test.go` with synthetic zip and tar.gz generators in `t.TempDir()`.
  - **GREEN**: Implemented `InspectZip` and `InspectTarGz` with zero disk extraction, executable magic byte detection, and source code false-positive pruning.
  - **REFACTOR**: Streamlined header buffer checks and verified error resilience on malformed and empty archives.
- **Commit**: `d1a8b11` (`feat(installer): add in-memory archive inspector with zero disk extraction`).

### Task 2: Detector de Classificação de Instaladores (`detector.go`, `detector_test.go`, `types.go`)
- **TDD Workflow**:
  - **RED**: Created `internal/installer/detector_test.go` verifying direct extensions, partial downloads, archive delegation, and non-installer files.
  - **GREEN**: Implemented `internal/installer/types.go` (`Candidate`, `InstallerType`, `Options`) and `internal/installer/detector.go` (`NewDetector`, `Detect`).
  - **REFACTOR**: Added edge cases for malformed archives and standardized descriptive details.
- **Commit**: `72ceb18` (`feat(installer): add installer detector with direct extensions and archive delegation`).

### Task 3: Scanner com Profundidade Limitada & Modelo de Configuração (`scanner.go`, `scanner_test.go`, `config.go`)
- **TDD Workflow**:
  - **RED**: Added `TestLoad_InstallerDefaults`, `TestLoad_InstallerCustomConfig`, and `TestValidate_InstallerMaxDepth` in `internal/config/config_test.go`, and comprehensive synthetic directory tests in `internal/installer/scanner_test.go`.
  - **GREEN**:
    - Added `InstallerConfig` to `internal/config/config.go`, defaults in `defaults.go`, and validation in `validate.go`.
    - Implemented `internal/installer/scanner.go` with depth limiting (`filepath.Rel`), symlink isolation, blacklist protection, macOS `.app` bundle aggregation, and min-age/min-size filters.
  - **REFACTOR**: Reusable tilde expansion and recursive size calculation ignoring internal symlinks.
- **Commit**: `4731ca9` (`feat(installer): add depth-limited scanner with blacklist and symlink isolation`).

---

## 3. Artifacts Created & Modified

| File | Status | Description |
| :--- | :--- | :--- |
| `internal/config/config.go` | Modified | Added `InstallerConfig` struct and `Installer` field in `Config` with viper defaults |
| `internal/config/defaults.go` | Modified | Set default installer paths (`~/Downloads`, `~/Desktop`) and `max_depth: 2` |
| `internal/config/validate.go` | Modified | Enforced `max_depth between 0 and 5` and non-empty path validation |
| `internal/config/config_test.go` | Modified | Added tests for default and custom installer configs and validation |
| `internal/installer/types.go` | Created | Defined `InstallerType`, `Candidate`, and `Options` |
| `internal/installer/archive.go` | Created | In-memory archive inspector for `.zip` and `.tar.gz`/`.tgz` with magic byte validation |
| `internal/installer/archive_test.go` | Created | Test suite for archive detection and source false-positive rejection |
| `internal/installer/detector.go` | Created | Classification engine for direct extensions, partial downloads, and archives |
| `internal/installer/detector_test.go` | Created | Unit tests for candidate detection and file type mappings |
| `internal/installer/scanner.go` | Created | Depth-limited scanner with blacklist, symlink isolation, `.app` bundle handling |
| `internal/installer/scanner_test.go` | Created | Test suite verifying depth limits, symlink skipping, age/size filters, and dotfiles |

---

## 4. Verification & Quality Gates

### Automated Test Suite
```bash
CGO_ENABLED=0 go test -v -count=1 ./internal/installer ./internal/config
```
- `internal/installer`: PASS (18 test suites)
- `internal/config`: PASS (21 test suites)
- Repository-wide test suite (`go test ./...`): PASS (0 regressions across all 33 packages)

### Code Coverage
- `internal/installer`: **89.0%**
- `internal/config`: **94.5%**

---

## 5. Next Steps

With Plan 02-01 complete, Phase 02 can proceed to:
- **Plan 02-02**: Reversible trash auto-expiration and quota management (`internal/trash`).
- **Plan 02-03**: Canonical `devclean trash clean` command and enriched quota inspection (`cmd/trash.go`).
- **Plan 02-04**: CLI command `devclean installer` with Lip Gloss preview and `deleter.Coordinator` integration (`cmd/installer.go`).

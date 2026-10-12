# Phase 01: Novos Ecossistemas e Presets de Limpeza — Plan 01-02 Summary

**Executed At:** 2026-10-12  
**Status:** Completed  
**Requirements Covered:** `ECO-02`, `ECO-03`, `NFR-01`, `NFR-02`  
**Decisions Implemented:** `D-09`, `D-10`, `D-12`, `D-13`, `D-14`  
**Threat Mitigations Verified:** `T-01-04`, `T-01-05`, `T-01-06`, `T-01-SC`  

---

## 1. Executive Summary

Plan 01-02 implements the Flutter/Dart and Terraform/OpenTofu ecosystem cleaners (`internal/cleaner/flutter` and `internal/cleaner/terraform`) within DevClean:
1. **Flutter/Dart Ecosystem Cleaner (`internal/cleaner/flutter/flutter.go`)**: Implements `domain.Cleaner` with strict parent manifest validation (`pubspec.yaml` required for claiming `.dart_tool/` as `RiskLow` and `build/` as `RiskMedium`), global Pub cache discovery via `$PUB_CACHE` / `~/.pub-cache` as `RiskLow`, bounded concurrency with `runtime.NumCPU()`, symlink rejection via `safety.IsSymlink`, and context-aware cancellation.
2. **Terraform/OpenTofu Ecosystem Cleaner (`internal/cleaner/terraform/terraform.go`)**: Implements `domain.Cleaner` with hybrid installation detection (`terraform` or `tofu` in PATH, or `.tf` files in `scanPaths`), strict safeguards against concurrent execution locks (`.terraform.tfstate.lock.info` in parent directory or inside `.terraform/`), safe avoidance of uncommitted local state files (`terraform.tfstate`), global plugin cache discovery (`$TF_PLUGIN_CACHE_DIR` / `~/.terraform.d/plugin-cache`), and bounded concurrent scanning.

Both packages pass hermetic compilation and unit testing under `CGO_ENABLED=0` with test statement coverage exceeding 91% (flutter: 92.5%, terraform: 91.9%).

---

## 2. Tasks Executed

### Task 1: Flutter/Dart Cleaner (`internal/cleaner/flutter/flutter.go` & `flutter_test.go`)
- **TDD Workflow**:
  - **RED**: Created `internal/cleaner/flutter/flutter_test.go` asserting cleaner name, toolchain / pub cache detection, discovery of `.dart_tool` and `build` in projects with `pubspec.yaml`, complete rejection of generic directories without `pubspec.yaml`, global pub cache detection with `$PUB_CACHE` override, clean removal, symlinks and exclusions, `.git` skip, and context cancellation.
  - **GREEN**: Implemented `flutterCleaner` in `internal/cleaner/flutter/flutter.go`:
    - `Name()`: returns `"flutter"`.
    - `IsInstalled()`: detects `flutter` or `dart` in PATH via `exec.LookPath`, or fallback directory `~/.pub-cache` / `$PUB_CACHE`.
    - `Scan()`: concurrent bounded directory walk with `runtime.NumCPU()` semaphore, SHA-256 target IDs, and batch size calculation via `devfs.CalcDirSizes`.
    - `Clean()`: target removal via `os.RemoveAll`.
    - Categorization & Risk: `flutter-dart-tool` (`RiskLow`), `flutter-build` (`RiskMedium`), `pub-cache` (`RiskLow`).
  - **REFACTOR**: Removed unused imports, verified zero CGO compliance, and validated symlink checks.
- **Coverage**: `internal/cleaner/flutter/flutter.go` achieved 92.5% statement coverage.
- **Commit**: `6e9bdad` (`feat(cleaner): add Flutter and Dart cleaner with pubspec validation`).

### Task 2: Terraform/OpenTofu Cleaner (`internal/cleaner/terraform/terraform.go` & `terraform_test.go`)
- **TDD Workflow**:
  - **RED**: Created `internal/cleaner/terraform/terraform_test.go` covering 13 test cases: cleaner name, toolchain detection via LookPath (`tofu`), hybrid detection via `.tf` presence in `scanPaths` (D-10), scanning valid projects with `.tf` files, rejection of generic `.terraform` folders without `.tf` files, active lock safeguards (`.terraform.tfstate.lock.info` in parent or target), local state safeguards (local backend, root state with resources, unconfigured backend), global plugin cache detection, removal, symlink and pattern exclusion handling, `.git` skipping, and context cancellation.
  - **GREEN**: Implemented `terraformCleaner` in `internal/cleaner/terraform/terraform.go`:
    - `Name()`: returns `"terraform"`.
    - `IsInstalled()`: hybrid detection checking `terraform`/`tofu` in PATH, or existence of `.tf` files across `scanPaths` using early-exit `fs.SkipAll`, or global plugin cache.
    - `Scan()`: concurrent bounded directory walk with `runtime.NumCPU()` semaphore, SHA-256 target IDs, and batch size calculation via `devfs.CalcDirSizes`.
    - `hasActiveLockOrLocalState()`: safeguards against active locks and local state corruption (D-09).
    - `Clean()`: target removal via `os.RemoveAll`.
    - Categorization & Risk: `terraform-local` (`RiskMedium`), `terraform-plugin-cache` (`RiskLow`).
  - **REFACTOR**: Verified error handling and path resolution.
- **Coverage**: `internal/cleaner/terraform/terraform.go` achieved 91.9% statement coverage.
- **Commit**: `d824fbf` (`feat(cleaner): add Terraform and OpenTofu cleaner with active lock safeguard`).

---

## 3. Artifacts Created & Modified

| File | Status | Description |
| :--- | :--- | :--- |
| `internal/cleaner/flutter/flutter.go` | Created | Flutter/Dart cleaner implementing `domain.Cleaner` |
| `internal/cleaner/flutter/flutter_test.go` | Created | Unit tests verifying detection, pubspec validation, caches, and cleaning |
| `internal/cleaner/terraform/terraform.go` | Created | Terraform/OpenTofu cleaner implementing `domain.Cleaner` with safeguards |
| `internal/cleaner/terraform/terraform_test.go` | Created | Unit tests covering hybrid detection, active lock and local state safeguards |

---

## 4. Verification & Quality Gates

### Automated Test Suite
```bash
CGO_ENABLED=0 go test -v -count=1 ./internal/cleaner/flutter/... ./internal/cleaner/terraform/...
```
- `internal/cleaner/flutter`: PASS (10 tests, 0.369s)
- `internal/cleaner/terraform`: PASS (13 tests, 0.158s)

### Code Coverage
- `internal/cleaner/flutter/flutter.go`: **92.5%**
- `internal/cleaner/terraform/terraform.go`: **91.9%**

### Static Analysis
```bash
go vet ./internal/cleaner/flutter/... ./internal/cleaner/terraform/...
```
- 0 warnings, clean pass.

---

## 5. Threat Model & Security Compliance

- **T-01-04 (Active Terraform Lock / Race Conditions):** Verified. `hasActiveLockOrLocalState` checks `.terraform.tfstate.lock.info` in both parent project directory and inside `.terraform/`. Folders with active locks are skipped and never scheduled for deletion (`TestTerraformCleaner_Scan_SkipOnActiveLock`).
- **T-01-05 (Local Terraform State Loss):** Verified. Local state in `.terraform/terraform.tfstate` or root `terraform.tfstate` containing managed resources or local backends is inspected and skipped (`TestTerraformCleaner_Scan_SkipOnLocalState`).
- **T-01-06 (False Positive in Generic `build/` or `.dart_tool/` Folders):** Verified. Folders named `.dart_tool/` and `build/` are only claimed if `pubspec.yaml` exists in the parent directory (`TestFlutterCleaner_Scan_IgnoreWithoutPubspec`). Folders named `.terraform` require `.tf` files in the parent directory (`TestTerraformCleaner_Scan_IgnoreWithoutTfFiles`).
- **T-01-SC (Zero CGO Supply Chain Hermeticity):** Verified. Both packages compile and run hermetically under `CGO_ENABLED=0`.

---

## 6. Self-Check Verification

- [x] `internal/cleaner/flutter/flutter.go` exists and implements `domain.Cleaner`
- [x] `internal/cleaner/flutter/flutter_test.go` exists and passes with coverage > 85% (92.5%)
- [x] `internal/cleaner/terraform/terraform.go` exists and implements `domain.Cleaner`
- [x] `internal/cleaner/terraform/terraform_test.go` exists and passes with coverage > 85% (91.9%)
- [x] Strict safeguards for active locks and local state in Terraform implemented
- [x] Manifest validation for Flutter/Dart implemented
- [x] Commits made atomically
- [x] `CGO_ENABLED=0` verified
- [x] `STATE.md` and `ROADMAP.md` left untouched

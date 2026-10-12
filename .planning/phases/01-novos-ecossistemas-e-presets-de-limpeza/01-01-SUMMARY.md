# Phase 01: Novos Ecossistemas e Presets de Limpeza — Plan 01-01 Summary

**Executed At:** 2026-10-12  
**Status:** Completed  
**Requirements Covered:** `ECO-01`, `NFR-01`, `NFR-02`  
**Decisions Implemented:** `D-05`, `D-06`, `D-07`, `D-08`  
**Threat Mitigations Verified:** `T-01-01`, `T-01-02`, `T-01-03`, `T-01-SC`  

---

## 1. Executive Summary

Plan 01-01 implements the foundation for the C/C++ ecosystem cleaning and POSIX read-only permission management within DevClean:
1. **POSIX `ChmodRecursive` Utility (`internal/fs/chmod.go`)**: Developed via TDD as the tracer bullet for the phase. Unlocks read-only trees (`0555` directories and `0444` files) produced by package managers like Conan and vcpkg prior to deletion, preventing `permission denied` (`EACCES`) errors on macOS and Linux.
2. **C/C++ Ecosystem Cleaner (`internal/cleaner/cpp/cpp.go`)**: Implements `domain.Cleaner` with strict CMake disambiguation (`CMakeCache.txt` or sibling `CMakeLists.txt`), local vcpkg manifest detection (`vcpkg.json` or `CMakeLists.txt`), global cache scanning (Conan 1, Conan 2, vcpkg across macOS and Linux), bounded concurrent scanning, symlink skip safeguards, and preventive recursive chmod during cleanup.

Hermetic compilation and tests pass with `CGO_ENABLED=0` with test coverage exceeding 90% across both packages.

---

## 2. Tasks Executed

### Task 1: Tracer Task — Utilitário POSIX `ChmodRecursive` (`internal/fs/chmod.go` & `chmod_test.go`)
- **TDD Workflow**:
  - **RED**: Created `internal/fs/chmod_test.go` asserting read-only tree restoration, non-existent path tolerance, single-file handling, and symlink isolation.
  - **GREEN**: Implemented `ChmodRecursive(root string, dirPerm, filePerm os.FileMode) error` in `internal/fs/chmod.go`. Unlocks root directory mode before `filepath.WalkDir` traversal, skips symlink targets, and applies `dirPerm` (`0755`) and `filePerm` (`0666`).
  - **REFACTOR**: Verified with `t.Cleanup` hooks and tested invalid path/error branches.
- **Coverage**: `internal/fs/chmod.go` achieved 90.5% statement coverage.
- **Commit**: `26df0e1` (`feat(fs): add POSIX ChmodRecursive utility for read-only trees`).

### Task 2: Implementação do Cleaner C/C++ (`internal/cleaner/cpp/cpp.go` & `cpp_test.go`)
- **TDD Workflow**:
  - **RED**: Created `internal/cleaner/cpp/cpp_test.go` covering 12 test cases: cleaner naming, toolchain detection, CMake build detection with cache and sibling lists, generic build rejection, vcpkg local discovery, global cache discovery, symlink/exclusion skipping, context cancellation, and read-only chmod cleanup.
  - **GREEN**: Implemented `cppCleaner` satisfying `domain.Cleaner`:
    - `Name()`: returns `"cpp"`.
    - `IsInstalled()`: detects compiler/build tools (`cmake`, `gcc`, `g++`, `clang`, `conan`, `vcpkg`, `ninja`, `make`) or config directories (`~/.conan`, `~/.conan2`, `$CONAN_USER_HOME`, `$CONAN_HOME`).
    - `Scan()`: concurrent bounded directory walk with `runtime.NumCPU()` semaphore, SHA-256 target IDs, and batch size calculation via `devfs.CalcDirSizes`.
    - `Clean()`: preventive `devfs.ChmodRecursive(t.Path, 0755, 0666)` preceding `os.RemoveAll`.
    - Categorization & Risk: `cmake-build` (`RiskMedium`), `vcpkg-installed` (`RiskMedium`), `conan-cache` (`RiskLow`), `conan2-cache` (`RiskLow`), `vcpkg-cache` (`RiskLow`).
  - **REFACTOR**: Resolved loop variable shadowing in tests and ensured zero CGO compliance.
- **Coverage**: `internal/cleaner/cpp/cpp.go` achieved 90.2% statement coverage (exceeding >85% target).
- **Commit**: `2a7ffff` (`feat(cleaner): add C/C++ CMake and vcpkg cleaner with preventive chmod`).

---

## 3. Artifacts Created & Modified

| File | Status | Description |
| :--- | :--- | :--- |
| `internal/fs/chmod.go` | Created | Exported `ChmodRecursive(root string, dirPerm, filePerm os.FileMode) error` |
| `internal/fs/chmod_test.go` | Created | Unit tests verifying read-only tree traversal and deletion |
| `internal/cleaner/cpp/cpp.go` | Created | C/C++ cleaner implementing `domain.Cleaner` |
| `internal/cleaner/cpp/cpp_test.go` | Created | Comprehensive tests covering detection, disambiguation, caches, and cleaning |

---

## 4. Verification & Quality Gates

### Automated Test Suite
```bash
CGO_ENABLED=0 go test -v -count=1 ./internal/fs/... ./internal/cleaner/cpp/...
```
- `internal/fs`: PASS (18 tests, 0.920s)
- `internal/cleaner/cpp`: PASS (12 tests, 0.855s)

### Code Coverage
- `internal/fs/chmod.go`: **90.5%**
- `internal/cleaner/cpp/cpp.go`: **90.2%**

### Static Analysis
- `go vet ./internal/cleaner/cpp/... ./internal/fs/...`: 0 warnings, clean pass.

---

## 5. Threat Model & Security Compliance

- **T-01-01 (Path Traversal / Symlink Escape):** Verified. Both `ChmodRecursive` and `walkForCppTargets` inspect `d.Type()&os.ModeSymlink != 0` and `safety.IsSymlink(path)`, skipping symlink targets without traversal (`TestChmodRecursive_Symlink`, `TestCppCleaner_Scan_SymlinkAndExclusions`).
- **T-01-02 (Read-Only Permission Denial):** Verified. `ChmodRecursive` enables clean deletion of `0555`/`0444` trees (`TestChmodRecursive_ReadOnlyTree`, `TestCppCleaner_Clean_ReadOnlyChmod`).
- **T-01-03 (Generic Build Collision Avoidance):** Verified. Folders named `build/` without `CMakeCache.txt` or sibling `CMakeLists.txt` are rejected by `isCMakeBuildDir` (`TestCppCleaner_Scan_IgnoreGenericBuildWithoutCMake`).
- **T-01-SC (Zero CGO Supply Chain Hermeticity):** Verified. All packages compile and execute under `CGO_ENABLED=0`.

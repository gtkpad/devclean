---
phase: 10-docker-orbstack-shared-stores
reviewed: 2026-10-16T14:40:00Z
depth: standard
files_reviewed: 17
files_reviewed_list:
  - internal/domain/types.go
  - internal/domain/target_test.go
  - internal/fs/usage.go
  - internal/fs/usage_darwin.go
  - internal/fs/usage_linux.go
  - internal/fs/usage_other.go
  - internal/fs/usage_test.go
  - internal/cleaner/docker/docker.go
  - internal/cleaner/docker/docker_test.go
  - internal/cleaner/docker/orbstack.go
  - internal/cleaner/docker/orbstack_test.go
  - internal/cleaner/dotnet/dotnet.go
  - internal/cleaner/dotnet/dotnet_test.go
  - internal/cleaner/pnpm/pnpm.go
  - internal/cleaner/pnpm/pnpm_test.go
  - internal/engine/register.go
  - internal/deleter/policy_test.go
findings:
  critical: 0
  warning: 4
  info: 5
  total: 9
status: issues_found
---

# Phase 10: Code Review Report

**Reviewed:** 2026-10-16T14:40:00Z  
**Depth:** standard  
**Files Reviewed:** 17  
**Status:** issues_found  

## Summary

An adversarial standard-depth code review was conducted across all 17 source and test files delivered during Phase 10 (`10-docker-orbstack-shared-stores`).

The phase successfully establishes critical safety foundations and zero-CGO filesystem accounting across macOS and Linux:
- **Strict Shared Store Locking**: All shared package stores and container runtime targets (`docker-images`, `docker-containers`, `docker-volumes`, `docker-build-cache`, `docker-vm-disk`, `orbstack-data`, `nuget-packages`, `pnpm-store`) are locked unconditionally to `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonSharedStore`, `EligibleBytes: 0`, and `TTLEligible: false`.
- **Zero Command/Shell Injection Risk**: All subprocess executions (`docker system df`, `pgrep -x OrbStack`, `pnpm store path`) use direct argument token slices via `cleaner.CommandExecutor` without shell interpolation.
- **Fail-Closed Process Guard**: OrbStack active detection via `pgrep -x OrbStack` handles running, stopped, and uncertain (exit code != 1 or missing tool) states with fail-closed semantics, revalidating process state immediately prior to deletion in `Clean()`.
- **Zero-CGO Sparse Accounting and Inode Deduplication**: `CalcUsage` reliably computes physical disk blocks (`st_blocks * 512`) versus apparent size, deduplicating hardlinks by `(dev, ino)` tuple to prevent inflated accounting in package stores.
- **Cryptographic Zero-Mutation Invariant**: `TestCoordinator_DryRun_ZeroMutation_Phase10` validates SHA-256 integrity, byte lengths, and mtimes across all 8 target categories with zero mutations in DryRun mode.
- **Test Coverage**: All affected packages comfortably exceed the 85% requirement (`internal/cleaner/pnpm` at 92.9%, `internal/cleaner/docker` at 88.9%, `internal/cleaner/dotnet` at 88.1%, `internal/fs` at 86.8%, `internal/engine` at 95.3%, `internal/deleter` at 89.7%), and `go vet` passes with 0 diagnostics.

The review surfaced 4 Warnings (symlink handling at `CalcUsage` root, missing `ExcludePaths` enforcement in PNPM and Docker/OrbStack cleaners, missing `safety.IsSymlink` check in `cleanOrbStackCache`, and prefix matching in `dotnet.go:isExcluded`) and 5 Info items (context propagation in `pnpm` and `orbstack` scans, context propagation in `dotnet`, error suppression in `dockerCleaner.Clean`, fallback parsing in `parseDockerSize`, and category guards in `pnpmCleaner.Clean`).

---

## Warnings

### WR-01: `CalcUsageWithContext` uses `os.Lstat` on root path, breaking measurement if target path is a symlink

**File:** `internal/fs/usage.go:31-44`  
**Issue:** `CalcUsageWithContext` calls `os.Lstat(path)` on the entry path. If `path` is a symlink pointing to a directory (e.g. `~/Library/pnpm/store` or `~/.orbstack` symlinked to an external volume or separate partition), `info.IsDir()` returns `false` because `ModeSymlink` is set rather than `ModeDir`. Consequently, `CalcUsage` takes the `!info.IsDir()` single-file branch, returning the symlink file's own string length (e.g., 25–40 bytes) and 0 allocated blocks, completely failing to traverse the directory tree. Furthermore, if `path` is a symlink to a regular file (e.g. `Docker.raw` symlinked to external fast storage), `os.Lstat` measures the symlink rather than the VM disk file, reporting 0 allocated bytes and inverted dimensions. While skipping symlinks *inside* the directory tree (line 71) is necessary to prevent directory escape and recursion cycles, the root target path itself should be resolved using `os.Stat` (or `filepath.EvalSymlinks`).  
**Fix:**
Use `os.Stat` on the root target path so symlinks to directories or files resolve to their actual filesystem target:
```go
func CalcUsageWithContext(ctx context.Context, path string) (Usage, error) {
	if err := ctx.Err(); err != nil {
		return Usage{}, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return Usage{Partial: true, StatusNote: err.Error()}, err
	}

	// Single file case (e.g. Docker.raw or data.img.raw)
	if !info.IsDir() {
		_, _, blocks, apparent, ok := getFileInfoStat(info)
		if !ok {
			return Usage{ApparentBytes: info.Size(), AllocatedBytes: info.Size(), FileCount: 1}, nil
		}
		alloc := blocks * 512
		return Usage{ApparentBytes: apparent, AllocatedBytes: alloc, FileCount: 1}, nil
	}
    // ...
```

---

### WR-02: PNPM and Docker/OrbStack cleaners ignore `rules.ExcludePaths`

**File:** `internal/cleaner/pnpm/pnpm.go:102-191` and `internal/cleaner/docker/orbstack.go:47-181`  
**Issue:** Neither `pnpmCleaner.Scan` nor `dockerCleaner.Scan` (including `scanOrbStack`) checks `rules.ExcludePaths`. If a user configures `ecosystems.pnpm.exclude_paths` or `ecosystems.docker.exclude_paths` in `config.yaml`, the exclusion rules are completely ignored. This is particularly problematic for regenerable caches (`pnpm-cache` at `~/Library/Caches/pnpm` / `~/.cache/pnpm` and `orbstack-cache` at `~/Library/Caches/dev.orbstack.OrbStack` / `~/.cache/orbstack`), which are not `ReviewOnly` and carry `EligibleBytes > 0` with `TTLEligible: true`. If a developer explicitly excludes them to preserve cached downloads, devclean will still discover, report, and delete them.  
**Fix:**
Apply path exclusion checks before calculating sizes and appending targets in `pnpm.go`:
```go
	for _, cacheDir := range candidates {
		if seen[cacheDir] || !devfs.DirExists(cacheDir) || isExcluded(cacheDir, rules.ExcludePaths) {
			continue
		}
		// ...
	}
```
And in `orbstack.go`:
```go
	for _, cPath := range cacheCandidates {
		if isExcluded(cPath, rules.ExcludePaths) {
			continue
		}
		// ...
	}
```

---

### WR-03: `cleanOrbStackCache` omits `safety.IsSymlink` defense-in-depth check

**File:** `internal/cleaner/docker/orbstack.go:184-193`  
**Issue:** In `cleanOrbStackCache`, while the function verifies the process guard (`checkOrbStackProcess`) and core blacklisting (`safety.IsCoreBlacklisted`), it omits the `safety.IsSymlink(t.Path)` check that is consistently enforced across other cleaners (`dotnet` at `internal/cleaner/dotnet/dotnet.go:209` and `pnpm` at `internal/cleaner/pnpm/pnpm.go:202`). If an OrbStack cache directory path has been symlinked to an external path, `cleanOrbStackCache` does not verify symlink status before deletion.  
**Fix:**
Add symlink validation prior to calling `os.RemoveAll`:
```go
func cleanOrbStackCache(ctx context.Context, exec cleaner.CommandExecutor, t domain.Target) error {
	running, uncertain := checkOrbStackProcess(ctx, exec)
	if running || uncertain {
		return nil // Abort deletion if process is running or state is uncertain
	}
	if safety.IsCoreBlacklisted(t.Path) {
		return nil
	}
	if isLink, err := safety.IsSymlink(t.Path); err == nil && isLink {
		return nil
	}
	return os.RemoveAll(t.Path)
}
```

---

### WR-04: Prefix matching without path boundary in `dotnet.go:isExcluded`

**File:** `internal/cleaner/dotnet/dotnet.go:297-304`  
**Issue:** In `dotnet.go`, `isExcluded` checks `if strings.HasPrefix(path, exc) { return true }`. If an exclude path string is `/home/user/.nuget/package`, it will unintentionally match `/home/user/.nuget/packages` or `/home/user/.nuget/package-other` because `strings.HasPrefix` does not verify path separators (`filepath.Separator`). In addition, glob patterns with `doublestar.Match` are not supported for global cache directory exclusions.  
**Fix:**
Ensure prefix matching only triggers when the path is identical or preceded by a path separator, and support doublestar matching:
```go
func isExcluded(path string, excludes []string) bool {
	base := filepath.Base(path)
	for _, exc := range excludes {
		if path == exc || strings.HasPrefix(path, strings.TrimSuffix(exc, string(filepath.Separator))+string(filepath.Separator)) {
			return true
		}
		if matched, _ := doublestar.Match(exc, path); matched {
			return true
		}
		if matched, _ := doublestar.Match(exc, base); matched {
			return true
		}
	}
	return false
}
```

---

## Info

### IN-01: Context cancellation not propagated to `devfs.CalcUsage` in `pnpm` and `orbstack` scans

**File:** `internal/cleaner/pnpm/pnpm.go:112` and `internal/cleaner/docker/orbstack.go:141`  
**Issue:** Both `pnpmCleaner.Scan` and `scanOrbStack` invoke `devfs.CalcUsage(path)` which delegates to `CalcUsageWithContext(context.Background(), path)`. Because the incoming `ctx` is not forwarded, if the scan is cancelled or times out, the recursive inode walk on large shared stores (hundreds of thousands of files in `pnpm`) or VM data directories will continue running in the background and consuming disk I/O instead of aborting immediately.  
**Fix:** Replace `devfs.CalcUsage(path)` with `devfs.CalcUsageWithContext(ctx, path)`.

---

### IN-02: `dotnetCleaner.Scan` and `Clean` discard incoming `context.Context`

**File:** `internal/cleaner/dotnet/dotnet.go:47,159,201`  
**Issue:** `dotnetCleaner.Scan(_ context.Context, rules domain.EcosystemRules)` discards the incoming context and invokes `devfs.CalcDirSizes(context.Background(), allPaths)` at line 159. If the scan is cancelled, local build output sizing will not abort promptly. `dotnetCleaner.Clean(_ context.Context, targets []domain.Target)` also discards `ctx`.  
**Fix:** Forward `ctx` to `scanLocalBuildOutputs(ctx, rules)` and pass `ctx` directly into `CalcDirSizes(ctx, allPaths)`.

---

### IN-03: `dockerCleaner.Clean` discards error returned by `cleanOrbStackCache`

**File:** `internal/cleaner/docker/docker.go:192`  
**Issue:** In `dockerCleaner.Clean`, the error from `cleanOrbStackCache(ctx, c.exec, t)` is ignored (`_ = cleanOrbStackCache(...)`). If `os.RemoveAll` fails due to permission errors or file locks, `dockerCleaner.Clean` returns `nil`, preventing the deletion coordinator from recording failure diagnostics.  
**Fix:** Propagate the error returned by `cleanOrbStackCache`:
```go
		if t.Category == "orbstack-cache" {
			if err := cleanOrbStackCache(ctx, c.exec, t); err != nil {
				return err
			}
		}
```

---

### IN-04: `parseDockerSize` lacks pure integer string fallback

**File:** `internal/cleaner/docker/docker.go:283-315`  
**Issue:** `parseDockerSize` matches size strings against known unit suffixes (`TiB`, `GiB`, `MiB`, `KiB`, `TB`, `GB`, `MB`, `KB`, `kB`, `B`). If a Docker daemon or tool outputs an unformatted integer byte count (e.g., `"1048576"` or `"0"` without suffix), all suffix matches fail and it returns 0.  
**Fix:** Add a direct `strconv.ParseInt(s, 10, 64)` check before the final `return 0`.

---

### IN-05: `pnpmCleaner.Clean` lacks target category check before `os.RemoveAll`

**File:** `internal/cleaner/pnpm/pnpm.go:194-208`  
**Issue:** In `pnpmCleaner.Clean`, while defensive guards protect `pnpm-store`, `virtual-pnpm-store`, and `pnpm://`, any target not matching those filters is forwarded to `os.RemoveAll(t.Path)`. Explicitly checking `if t.Category != "pnpm-cache" { continue }` prevents accidental deletions if unexpected target types are supplied.  
**Fix:** Add `if t.Category != "pnpm-cache" { continue }` at the beginning of the loop in `pnpmCleaner.Clean`.

---

_Reviewed: 2026-10-16T14:40:00Z_  
_Reviewer: Antigravity (gsd-code-reviewer)_  
_Depth: standard_

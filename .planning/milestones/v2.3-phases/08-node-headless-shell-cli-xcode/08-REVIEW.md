---
phase: 08-node-headless-shell-cli-xcode
reviewed: 2026-10-15T19:45:00Z
depth: standard
files_reviewed: 21
files_reviewed_list:
  - internal/cleaner/node/node.go
  - internal/cleaner/node/node_test.go
  - internal/cleaner/pnpm/pnpm.go
  - internal/cleaner/pnpm/pnpm_test.go
  - internal/cleaner/shell/shell.go
  - internal/cleaner/shell/shell_test.go
  - internal/cleaner/xcode/xcode.go
  - internal/cleaner/xcode/xcode_test.go
  - internal/cleaner/executor_mock.go
  - internal/cleaner/cleaner_test.go
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/config/defaults.go
  - internal/config/validate.go
  - internal/config/save.go
  - internal/engine/engine.go
  - internal/engine/register.go
  - internal/engine/register_test.go
  - internal/tui/config.go
  - internal/tui/config_test.go
  - internal/deleter/policy_test.go
findings:
  critical: 0
  warning: 4
  info: 3
  total: 7
status: issues_found
---

# Phase 08: Code Review Report

**Reviewed:** 2026-10-15T19:45:00Z  
**Depth:** standard  
**Files Reviewed:** 21  
**Status:** issues_found  

## Summary

A comprehensive standard-depth adversarial code review was conducted across the 21 source and test files delivered during Phase 08 (`08-node-headless-shell-cli-xcode`).

The implementation demonstrates exceptional quality in its primary safety guarantees:
- **Zero Command/Shell Injection Risk**: All subprocess executions (`pnpm`, `pgrep`, `xcrun simctl`) use direct argv token slices via `cleaner.CommandExecutor` without shell interpolation (`sh -c` / `bash -c`). This is verified by `TestProviders_NoShellInjection`.
- **Strict Symlink Protection**: All four cleaners (`node`, `pnpm`, `shell`, `xcode`) systematically check `safety.IsSymlink` before issuing filesystem deletions, and refuse to traverse or remove external symlink targets.
- **Fail-Closed Process Guard**: Xcode and Simulator active detection is fail-closed, locking sensitive artifacts with `ReviewOnly: true`, `ReviewReason: domain.ReviewReasonActiveProcess`, and 0 eligible bytes upon running or uncertain process status.
- **Canary Zero-Mutation Verification**: The DryRun canary test in `internal/deleter/policy_test.go` cryptographically validates SHA-256 hashes, byte sizes, and timestamps across 9 representative targets with zero mutations.
- **Coverage**: All affected packages comfortably exceed the 85% requirement (ranging from 89.5% to 95.3%), and `go vet` passes with 0 diagnostics.

The review identified 4 Warnings (primarily concerning inconsistent `ExcludePaths` enforcement on global/virtual targets and prefix matching in `isExcluded`) and 3 Info items (context propagation in `nodeCleaner.Scan`, helper duplication, and virtual aggregated target cleanup semantics).

---

## Warnings

### WR-01: Global package and browser caches in Node cleaner ignore `rules.ExcludePaths`

**File:** `internal/cleaner/node/node.go:260-270`  
**Issue:** In `nodeCleaner.Scan`, while `walkScanPath` properly honors `rules.ExcludePaths` for local projects, `scanGlobalCaches` iterates over global candidate directories (`~/.npm/_cacache`, `~/.npm/_npx`, `~/.npm/_logs`, Corepack, Puppeteer, Playwright, Cypress) without checking `rules.ExcludePaths`. If a developer configures `ecosystems.node.exclude_paths` in `config.yaml` to preserve specific browser downloads or package caches (e.g. `**/puppeteer/**` or `**/Cypress/**`), these exclusions are bypassed, resulting in unwanted discovery and potential deletion during cleanup.  
**Fix:**
Integrate path exclusion check before stat and size calculations in `scanGlobalCaches`:
```go
	for _, def := range getGlobalCacheCandidates(home) {
		for _, path := range def.paths {
			if path == "" || seenPaths[path] {
				continue
			}
			if isExcluded(path, rules.ExcludePaths) {
				continue
			}
			seenPaths[path] = true
```

---

### WR-02: PNPM cleaner ignores `rules.ExcludePaths` for store prune and metadata cache

**File:** `internal/cleaner/pnpm/pnpm.go:64-126`  
**Issue:** In `pnpmCleaner.Scan`, neither the virtual store target (`pnpm://store/prune`) nor physical HTTP metadata caches (`~/Library/Caches/pnpm`, `~/.cache/pnpm`) check `rules.ExcludePaths`. If a user specifies exclusion patterns for `pnpm` in `config.yaml`, the targets are still emitted and eligible for cleanup.  
**Fix:**
Apply exclusion checks before emitting targets in `pnpmCleaner.Scan`:
```go
	// 1. Virtual pnpm store prune target
	if !isExcluded("pnpm://store/prune", rules.ExcludePaths) {
		out, err := c.exec.Output(ctx, "pnpm", "store", "path")
		if err == nil {
			// ...
		}
	}

	// 2. Physical HTTP metadata cache
	for _, cacheDir := range candidates {
		if seen[cacheDir] || !devfs.DirExists(cacheDir) || isExcluded(cacheDir, rules.ExcludePaths) {
			continue
		}
		// ...
	}
```

---

### WR-03: Virtual iOS Simulator target in Xcode cleaner ignores `rules.ExcludePaths`

**File:** `internal/cleaner/xcode/xcode.go:258-267`  
**Issue:** In `xcodeCleaner.Scan`, while `DerivedData` (line 188) and expanded physical targets (line 224) correctly invoke `isExcluded(def.path, rules.ExcludePaths)`, the virtual simulator target `xcode://simulators/unavailable` is unconditionally appended without checking `rules.ExcludePaths`. Users cannot exclude unavailable simulator purging via `config.yaml`.  
**Fix:**
Wrap virtual simulator emission with `isExcluded`:
```go
	// 3. Alvo virtual para simuladores iOS indisponíveis (D-10)
	if !isExcluded("xcode://simulators/unavailable", rules.ExcludePaths) {
		targets = append(targets, domain.Target{
			ID:            "xcode-simulators-unavailable",
			Name:          "iOS Simulators (unavailable runtimes)",
			Path:          "xcode://simulators/unavailable",
			SizeBytes:     0,
			EligibleBytes: 0,
			TTLEligible:   true,
			Risk:          domain.RiskLow,
			Category:      "virtual-xcode-simulators",
		})
	}
```

---

### WR-04: `isExcluded` prefix matching without path boundary check

**File:** `internal/cleaner/shell/shell.go:240` and `internal/cleaner/xcode/xcode.go:319`  
**Issue:** `isExcluded` checks `if strings.HasPrefix(path, exc) { return true }`. If an exclude path string is `/var/cache/app`, it will unintentionally match `/var/cache/apple` or `/var/cache/app-other` because `strings.HasPrefix` does not ensure that the match terminates on a path boundary (slash or end-of-string).  
**Fix:**
Ensure prefix matching only triggers when the path is identical or preceded by a path separator:
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

### IN-01: `nodeCleaner.Scan` ignores `ctx context.Context` and passes `context.Background()` to `CalcDirSizes`

**File:** `internal/cleaner/node/node.go:34,75`  
**Issue:** `nodeCleaner.Scan(_ context.Context, rules domain.EcosystemRules)` discards the incoming context and calls `devfs.CalcDirSizes(context.Background(), paths)` at line 75. If the scan is cancelled by timeout or user interrupt (`SIGINT`), directory size calculation will continue consuming disk I/O instead of aborting immediately.  
**Fix:** Accept `ctx context.Context` in `Scan` and pass `ctx` directly to `devfs.CalcDirSizes(ctx, paths)`.

---

### IN-02: Code duplication of `isExcluded` helper across cleaners

**File:** `internal/cleaner/shell/shell.go:238` and `internal/cleaner/xcode/xcode.go:317`  
**Issue:** The `isExcluded` helper function is duplicated across multiple cleaner packages (`shell`, `xcode`, `dotnet`, `homebrew`).  
**Fix:** Centralize `isExcluded` as `cleaner.IsExcluded(path string, excludes []string) bool` in `internal/cleaner/` and reuse across all ecosystem cleaners.

---

### IN-03: Virtual aggregated target `shell://zsh/compdump` bypasses per-dump exclusions at cleanup time

**File:** `internal/cleaner/shell/shell.go:347-374`  
**Issue:** In `shellCleaner.Scan`, individual `.zcompdump*` files matching `rules.ExcludePaths` are excluded from the calculated size and mtime. However, when the aggregated virtual target `shell://zsh/compdump` is cleaned in `Clean()`, `Clean` globs and deletes all matching files without exclusion context because `domain.Cleaner.Clean` receives only `[]domain.Target`. While excluding the virtual target itself (`exclude_paths: ["shell://zsh/compdump"]`) works as intended, granular per-file exclusions inside `~/.zcompdump*` are lost during clean.  
**Fix:** Document this behavior in user documentation or retain explicit target paths where granular exclusions are required.

---

_Reviewed: 2026-10-15T19:45:00Z_  
_Reviewer: Claude (gsd-code-reviewer)_  
_Depth: standard_

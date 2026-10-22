# Safety Architecture

DevClean is built with defense-in-depth safety mechanisms to ensure that user code, secrets, active workspaces, and critical system paths are never destroyed.

---

## 1. Immutable Core Blacklist

The following directories and sensitive files are **hardcoded into the binary** and can never be cleaned, bypassed, or removed:

| Pattern | Rationale |
| :--- | :--- |
| `.git` | Version control repository history, branches, and stashes |
| `.env` | Local environment secrets and credentials |
| `.ssh` | SSH key pairs and authentication configurations |
| `secrets.json` | Local application credentials and token storage |
| System root paths | Critical OS filesystems (`/`, `/System`, `/usr`, `/bin`, `/etc`) |

> [!IMPORTANT]
> The immutable blacklist cannot be overridden by `--force` or `--all`. Calling `devclean whitelist remove` against any immutable blacklist entry will return an explicit error and reject the modification.

---

## 2. Unified Deletion Coordinator (`deleter.Coordinator`)

All deletion workflows (CLI `clean`, CLI `purge`, interactive TUI, and MCP Server) route through a single architectural pipeline: the [`deleter.Coordinator`](../internal/deleter/deleter.go).

- **Domain-level invariance**: Safety checks cannot be circumvented by switching between UI and CLI interfaces.
- **Hook orchestration**: Manages pre-deletion and post-deletion lifecycle hooks deterministically.
- **Audit trail**: Persists structured execution summaries to the historical database.

---

## 3. Virtual Target Isolation

Certain targets represent daemon-managed or CLI-managed resources rather than real filesystem paths (e.g., Docker layers `docker://`, Homebrew bottles `homebrew://`, pnpm stores `pnpm://`).

- Virtual targets are tagged with `IsVirtual() == true`.
- When trash is enabled, virtual targets **bypass filesystem trash staging** with a safety warning instead of raising filesystem I/O errors.
- Cleaners execute daemon-level prune operations directly.

---

## 4. Git Guards & Dirty Tree Protection (`purge`)

The `devclean purge` command removes build artifacts and orphaned dependencies from inactive or merged repositories with several Git-level safeguards:

1. **Dirty Working Tree Guard**:
   - Before purging, DevClean inspects `git status --porcelain`.
   - If uncommitted changes, untracked files, or modified tracked files are detected, the repository is **immediately skipped** with a warning to avoid losing uncommitted work.
2. **Branch Merge Verification (`--merged-only`)**:
   - Ensures that the current branch is fully merged into `main` or `master` upstream before qualifying the directory for purge.
3. **Commit Inactivity Cutoff (`--inactive-days`)**:
   - Reads the repository's latest commit timestamp via `git log -1 --format=%cI`. If recent commits exist within the threshold, the repository is deemed active and spared.

---

## 5. Trash Staging & Safe Restoration

When `safety.trash_enabled` is active (default), DevClean does not delete filesystem files directly:

1. Files are moved atomically into `~/.local/share/devclean/trash/<uuid>/`.
2. A manifest file (`manifest.json`) records the original absolute path, target ID, ecosystem, deletion timestamp, and SHA-256 integrity checksum.
3. Users can list staged items:
   ```bash
   devclean trash list
   ```
4. Files can be restored to their original location:
   ```bash
   devclean trash restore <target-id>
   ```
5. Expired items are purged based on `safety.retention_days` and `safety.max_size_mb`.

---

## 6. Configurable Exclusions & Globs

Users can specify custom protections via `config.yaml` or CLI:

```bash
devclean whitelist add ~/Work/critical-project
devclean whitelist add --pattern "*/production-data/*"
```

- **`exclude_paths`**: Canonical paths checked before scanning.
- **`protected_patterns`**: Glob expressions evaluated against both the directory basename and its full path using [doublestar](https://github.com/bmatcuk/doublestar).

---

## 7. Symlink Traversal Guard

DevClean uses `os.Lstat` instead of `os.Stat`:
- Symlinks are never traversed or resolved recursively.
- Deleting a symlink removes only the link itself, never the destination directory or shared library target.

---

## 8. Network Mount Detection

On macOS, paths located under `/net/` are detected as NFS automounts and skipped by default. This avoids:
- Accidental file removal on remote file servers.
- Unresponsive terminal hangs caused by slow network I/O.

---

## 9. Process Safety Guards (`active_process`)

Deleting active cache or database files while an application is running can cause state corruption, crashes, or sqlite lock conflicts.

- **Fail-Closed Inspection**: DevClean inspects running processes (via `pgrep`) before scanning or cleaning targets for browsers (Chrome, Brave, Arc, Edge), editors (VS Code, Cursor, VSCodium, Positron), and Xcode.
- **Fail-Closed on Ambiguity**: If process status cannot be determined with certainty (e.g., timeout, permission issue, exit code > 1), DevClean treats the state as ambiguous and refuses to delete.
- **Differentiated Daemon Policy**: Always-on Apple background daemons (e.g., `mediaanalysisd`, `geod`) whose temporary cache stores can be emptied safely are unblocked from false process gating, while interactive user applications remain strictly protected.

---

## 10. Consolidated Target Sub-Path Surgical Unlinking (`SubPaths`)

Container directories (such as `~/Library/Caches`, browser profile folders, and macOS sandboxed containers) often contain both disposable caches and persistent user configuration. Deleting the parent root container would destroy permissions and user state.

- **Hierarchical Ownership Bypass**: In `internal/engine/ownership.go`, `ResolveCanonicalOwnership` detects consolidated targets (`IsConsolidated == true` or `len(SubPaths) > 0`) and bypasses child size subtraction, preserving true reporting numbers.
- **Leaf-Only Unlinking**: The deletion coordinator unlinks only the explicit constituent leaf directories listed in `Target.SubPaths`, strictly preserving parent containers, permission bits, and unmanaged sibling folders.

---

## 11. Application Leftovers Multi-Source Verification

Determining whether an application in `~/Library/Application Support` is truly uninstalled requires multi-layered validation to prevent catastrophic false positives.

DevClean verifies candidate directories against four authoritative sources before flagging them as leftovers:
1. **Static System Exclusions**: A hardcoded catalog protecting Apple system services, OS daemons, frameworks, and symbol stores (~36+ GB shielded).
2. **Pure-Go Package Receipts**: Direct parsing of macOS installer receipts in `/var/db/receipts` without subprocess overhead.
3. **Pure-Go Homebrew Indexing**: In-memory inspection of both Homebrew Caskroom and Cellar installed package metadata.
4. **Spotlight & Toolchain Lookups**: Memoized, thread-safe Spotlight queries (`kMDItemCFBundleIdentifier`) combined with CLI `exec.LookPath` validation for developer toolchains.

---

## 12. Inviolable Leftovers Deletion Defense

To provide defense in depth for personal user data:
- All discovered leftovers and LaunchAgents are marked `ReviewOnly: true` (`ReviewReasonAppLeftover`).
- The `leftoversCleaner.Clean` method enforces an inviolable internal guard that unconditionally returns `nil`.
- This ensures that leftover application directories are surfaced strictly for user inspection and can never be deleted automatically during automated or bulk clean runs.

---

## 13. Location-Aware LaunchAgent Ownership

LaunchAgent orphan detection validates filesystem ownership relative to daemon location:
- `/Library/LaunchAgents`: Requires `root:wheel` (`UID 0`) ownership.
- `~/Library/LaunchAgents`: Requires current user ownership (`UID == os.Getuid()`).

Daemons with mismatched ownership or broken script targets are handled with strict location-aware security checks.

---

## 14. POSIX Recursive Chmod (`ChmodRecursive`)

Package managers such as Conan (`~/.conan2/p`) and vcpkg (`vcpkg_installed`) frequently mark downloaded source and binary archives with read-only permissions (`0400` / `0500`).

- Attempting standard deletion on read-only trees results in `EACCES` (permission denied) errors.
- DevClean implements `internal/fs.ChmodRecursive`, which safely elevates write permissions on target child directories immediately prior to unlinking, ensuring clean deletion without requiring elevated `sudo` privileges.

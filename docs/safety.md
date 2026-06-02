# Safety

DevClean implements multiple layers of protection to prevent accidental deletion of important files.

## Immutable Blacklist

The following paths are **never** deleted, regardless of configuration or flags (including `--force`):

| Pattern        | Reason                  |
| -------------- | ----------------------- |
| `.git`         | Version control history |
| `.env`         | Environment secrets     |
| `.ssh`         | SSH keys and config     |
| `secrets.json` | Application secrets     |

This blacklist is compiled into the binary and cannot be overridden.

## Configurable Protected Patterns

Additional paths can be protected via `protected_patterns` in the config file:

```yaml
protected_patterns:
  - "*/proprietary/*"
  - "*/sensitive/*"
  - "*/my-important-project/*"
```

Patterns use [doublestar](https://github.com/bmatcuk/doublestar) glob syntax and are matched against both:

- The file/directory **basename**
- The **full path**

## Risk Classification

Every scan target is assigned a risk level:

| Level      | Meaning                              | Examples                                             |
| ---------- | ------------------------------------ | ---------------------------------------------------- |
| **Low**    | Rebuilt without effort; no data loss | Go caches, NuGet cache, uv cache, Docker build cache |
| **Medium** | Requires time to rebuild (minutes)   | `node_modules`, .NET `bin/obj`                       |
| **High**   | Potential data loss                  | Docker volumes                                       |

Risk levels are displayed in the TUI and scan output so you can make informed decisions. The `l` key in the TUI selects only Low-risk targets for safe batch cleanup.

## Symlink Detection

DevClean uses `os.Lstat()` (not `os.Stat()`) when resolving paths. This means:

- Symlinks are **not followed** — only the link itself is evaluated
- Deleting a symlink removes the link, not the target
- This prevents accidentally wiping shared directories that happen to be linked into a project

## Network Mount Detection

On macOS, paths under `/net/` are automatically detected as NFS automounts and excluded from scanning. This prevents:

- Cleaning shared network drives
- Triggering slow network I/O during scans
- Accidentally deleting files on remote filesystems

## TTL as Safety Net

The `--auto` flag provides an additional safety layer by only cleaning targets that haven't been accessed within `max_age_days`. Active projects are left untouched.

```bash
# Only cleans targets older than their ecosystem's TTL
devclean clean --auto

# Bypasses TTL (but NOT blacklist or protected patterns)
devclean clean --force
```

## Fail Modes

Per-ecosystem `fail_mode` controls behavior on errors:

| Mode       | Behavior                                    |
| ---------- | ------------------------------------------- |
| `continue` | Log error, skip target, proceed with others |
| `abort`    | Stop immediately on first error             |

Default is `continue` — a single failed deletion doesn't prevent cleaning other valid targets.

## Summary of Layers

```
Request to delete path
  │
  ├── Immutable blacklist? ──→ BLOCKED (always)
  │
  ├── Protected pattern match? ──→ BLOCKED
  │
  ├── Is symlink? ──→ Only remove link, not target
  │
  ├── Is network mount? ──→ SKIPPED
  │
  ├── TTL check (--auto only) ──→ SKIPPED if too recent
  │
  └── DELETE
```

# Configuration

DevClean uses a YAML configuration file to control scan behavior, TTL rules, and safety patterns.

## File Location

```
~/.config/devclean/config.yaml
```

Override with `--config /path/to/file.yaml` on any command.

## Full Reference

```yaml
# Directories to scan for ecosystem artifacts
scan_paths:
  - ~/Code
  - ~/Work
  - ~/Projects

# Glob patterns for paths that should never be cleaned
# Matched against both basename and full path
protected_patterns:
  - "*/proprietary/*"
  - "*/sensitive/*"
  - "*/do-not-delete/*"

# Per-ecosystem settings
ecosystems:
  docker:
    enabled: true
    max_age_days: 30
    fail_mode: continue # continue | abort
    exclude_paths: []

  node:
    enabled: true
    max_age_days: 14
    fail_mode: continue
    exclude_paths:
      - "*/legacy-app/*"

  go:
    enabled: true
    max_age_days: 60
    fail_mode: continue
    exclude_paths: []

  dotnet:
    enabled: true
    max_age_days: 45
    fail_mode: continue
    exclude_paths: []

  python:
    enabled: true
    max_age_days: 30
    fail_mode: continue
    exclude_paths: []
```

## Fields

### `scan_paths`

List of directories where DevClean searches for ecosystem artifacts (e.g., `node_modules`, `bin/obj`). Supports `~` expansion.

Docker and Go global caches are resolved independently via environment variables and don't depend on `scan_paths`.

### `protected_patterns`

Glob patterns (using [doublestar](https://github.com/bmatcuk/doublestar) syntax) that mark paths as untouchable. Patterns are tested against both the file basename and the full path.

### `ecosystems.<name>.enabled`

Toggle individual ecosystems on/off. Disabled ecosystems are skipped during scan.

### `ecosystems.<name>.max_age_days`

TTL threshold used by `devclean clean --auto`. Targets last accessed more recently than this value are kept; older targets are deleted.

### `ecosystems.<name>.fail_mode`

How to handle errors during scan/clean:

- `continue` — Log the error and proceed with remaining targets
- `abort` — Stop immediately on first error

### `ecosystems.<name>.exclude_paths`

Glob patterns for paths to skip within this ecosystem's scan. Useful for excluding specific projects from cleanup.

## Defaults (No Config File)

If no config file exists, DevClean uses these compiled-in defaults:

| Ecosystem | max_age_days | fail_mode |
| --------- | ------------ | --------- |
| Docker    | 30           | continue  |
| Node      | 14           | continue  |
| Go        | 60           | continue  |
| .NET      | 45           | continue  |
| Python    | 30           | continue  |

Default `scan_paths`: `~/Code`, `~/Projects`

## Precedence

Values are resolved in this order (highest wins):

1. CLI flags (when available)
2. Config file (`~/.config/devclean/config.yaml`)
3. Compiled-in defaults

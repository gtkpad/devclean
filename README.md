# DevClean

A CLI tool that reclaims disk space across polyglot development environments. Scans caches, build artifacts, and orphaned dependencies from **Docker, Go, Node.js, .NET, and Python/uv** in a single run.

## Features

- **Multi-ecosystem scanning** — Docker images/volumes/build-cache, Go mod/build cache, node_modules, NuGet/.NET bin/obj, Python uv cache
- **Interactive TUI** — Browse targets grouped by ecosystem, toggle selections, see real-time size totals
- **Risk classification** — Each target is rated Low/Medium/High so you know what's safe to delete
- **TTL-based automation** — Configure max age per ecosystem; `--auto` deletes only stale targets
- **Safety mechanisms** — Immutable blacklist (.git, .env, .ssh), symlink detection, network mount detection
- **Scheduling** — Register recurring cleanup via Launchd (macOS) or Cron (Linux)
- **Zero dependencies at runtime** — Single static binary (CGO_ENABLED=0)

## Installation

```bash
go install github.com/gtkpad/devclean@latest
```

Or build from source:

```bash
git clone https://github.com/gtkpad/devclean.git
cd devclean
go build -o devclean .
```

## Quick Start

```bash
# Interactive TUI — scan and select what to clean
devclean

# Scan only, print results as table
devclean scan

# Auto-clean stale targets based on TTL config
devclean clean --auto

# Force-clean everything found (no TTL filter)
devclean clean --force

# Schedule daily cleanup at noon
devclean schedule --daily
```

## Commands

| Command               | Description                                     |
| --------------------- | ----------------------------------------------- |
| `devclean`            | Launch interactive TUI (default)                |
| `devclean scan`       | Scan and display targets                        |
| `devclean clean`      | Delete targets (requires `--auto` or `--force`) |
| `devclean schedule`   | Register recurring cleanup                      |
| `devclean unschedule` | Remove scheduled cleanup                        |
| `devclean version`    | Show version info                               |

### Global Flags

| Flag        | Description                                                     |
| ----------- | --------------------------------------------------------------- |
| `--config`  | Path to config file (default: `~/.config/devclean/config.yaml`) |
| `--verbose` | Verbose output                                                  |
| `--quiet`   | Suppress non-essential output                                   |

### `scan`

| Flag              | Description                                            |
| ----------------- | ------------------------------------------------------ |
| `-e, --ecosystem` | Filter by ecosystem (docker, go, node, dotnet, python) |
| `--format`        | Output format: `table` (default) or `json`             |

### `clean`

| Flag              | Description                              |
| ----------------- | ---------------------------------------- |
| `--auto`          | Delete targets older than configured TTL |
| `--force`         | Delete all targets regardless of age     |
| `-e, --ecosystem` | Filter by ecosystem                      |

### `schedule`

| Flag       | Description                      |
| ---------- | -------------------------------- |
| `--daily`  | Schedule at noon daily           |
| `--cron`   | Custom cron expression (5-field) |
| `--status` | Show current schedule status     |

## Configuration

Default config location: `~/.config/devclean/config.yaml`

```yaml
scan_paths:
  - ~/Code
  - ~/Work

protected_patterns:
  - "*/proprietary/*"

ecosystems:
  docker:
    enabled: true
    max_age_days: 30
  node:
    enabled: true
    max_age_days: 14
  go:
    enabled: true
    max_age_days: 60
  dotnet:
    enabled: true
    max_age_days: 45
  python:
    enabled: true
    max_age_days: 30
```

See [docs/configuration.md](docs/configuration.md) for full reference.

## Safety

DevClean never deletes:

- `.git` directories
- `.env` files
- `.ssh` directories
- `secrets.json` files

These are protected by an immutable blacklist that cannot be overridden, even with `--force`.

Additional safety: symlink detection (won't follow links), network mount detection (skips NFS automounts on macOS), and configurable glob patterns via `protected_patterns`.

See [docs/safety.md](docs/safety.md) for details.

## Documentation

- [Configuration](docs/configuration.md) — Full config reference, defaults, precedence
- [Cleaners](docs/cleaners.md) — What each ecosystem cleaner targets
- [Scheduling](docs/scheduling.md) — Launchd/Cron setup, flags, history
- [Safety](docs/safety.md) — Blacklist, risk levels, protections
- [TUI](docs/tui.md) — Interactive interface guide

## Requirements

- Go 1.25+ (build)
- macOS or Linux
- Docker CLI (optional, for Docker cleaner)
- uv CLI (optional, for Python cleaner)

## License

MIT

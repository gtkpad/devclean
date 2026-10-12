# DevClean

A high-performance CLI tool and interactive TUI that reclaims disk space across polyglot development environments. Scans caches, build artifacts, and orphaned dependencies across **13 ecosystems** in parallel.

---

## Features

- **13 Ecosystems Supported** — Docker, Go, Node.js, .NET, Python (uv), Rust (Cargo), Java/Kotlin (Gradle & Maven), AI Tools & Editors (Claude Code, VS Code, Cursor, Ollama, HuggingFace), pnpm, Bun, Next.js, Xcode (DerivedData & iOS Simulators), and Homebrew.
- **Interactive TUI** — Fullscreen [Bubble Tea](https://github.com/charmbracelet/bubbletea) interface with fuzzy filtering (`/`), multi-criteria sorting (`s`), risk filtering (`r`), mass selection (`m`), and instant real-time space calculation.
- **Mole-Inspired CLI Purge** — `devclean purge` expunges build artifacts and caches from inactive or merged Git repositories while automatically preserving dirty working trees.
- **Whitelist & Blacklist Governance** — `devclean whitelist` CLI command to add, list, and remove exclusions in `config.yaml`, strictly guarded by the immutable system blacklist (`.git`, `.env`, `.ssh`, `secrets.json`).
- **Visual Lip Gloss Report Card** — Beautiful post-cleanup summary with real filesystem disk delta via Statfs (zero CGO dependencies), proportional mini-bars per ecosystem, and machine-readable `--format json` support.
- **Unified Deletion Coordinator** — Centralized architecture ensuring consistent safety, pre/post deletion hooks, history recording, and virtual target isolation across CLI, TUI, and MCP.
- **Trash Staging & Instant Recovery** — Safely stage deleted files to trash with UUID-indexed manifests and restore them on demand (`devclean trash restore`).
- **Heuristic Smart Scoring** — Intelligent reclaim scoring (0.0–1.0) taking into account artifact age, size, Git commit activity, and branch merge status.
- **Background Watch Daemon & Scheduling** — Continuous filesystem threshold monitoring with desktop alerts (`devclean watch`) and native OS scheduling via Launchd (macOS) or Cron (Linux).
- **MCP Server for AI Agents** — Model Context Protocol (stdio JSON-RPC) server exposing scan, proposal, explanation, and deletion tools to AI agents like Claude Code and Cursor.
- **Zero Runtime Dependencies** — Single, portable static binary compiled with `CGO_ENABLED=0`.

---

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

---

## Quick Start

```bash
# Launch interactive TUI (default)
devclean

# Preview cleanable targets in terminal
devclean scan

# Auto-clean stale targets using configured TTLs
devclean clean --auto

# Force clean all discovered targets (bypass TTL)
devclean clean --force

# Preview cleanup without deleting anything (dry-run)
devclean clean --auto --dry-run

# Output cleanup report as JSON
devclean clean --auto --format json

# Purge build artifacts from repositories inactive for 60+ days
devclean purge --inactive-days 60

# Purge only repositories with branches already merged into main
devclean purge --merged-only

# Manage excluded paths and protected patterns
devclean whitelist list
devclean whitelist add ~/Projects/legacy-app
devclean whitelist add --pattern "*/do-not-touch/*"

# List or restore staged files from trash
devclean trash list
devclean trash restore <target-id>

# Schedule recurring daily cleanup
devclean schedule --daily
```

---

## Commands

| Command | Description |
| :--- | :--- |
| `devclean` | Launch interactive TUI (default entrypoint) |
| `devclean scan` | Scan ecosystems and display targets without deleting |
| `devclean clean` | Clean targets (`--auto` with TTL or `--force`) |
| `devclean purge` | Purge build artifacts from inactive or merged Git repositories |
| `devclean whitelist` | View, add, or remove paths and glob patterns from whitelist |
| `devclean trash` | Manage trash staging (`list`, `restore <id>`, `empty`) |
| `devclean schedule` | Register recurring cleanup via Launchd (macOS) or Cron (Linux) |
| `devclean unschedule` | Remove registered cleanup schedule |
| `devclean watch` | Monitor disk threshold continuously and trigger notifications |
| `devclean why <path>` | Explain why a path is flagged and display its risk analysis |
| `devclean history` | Display audit history of past cleanup runs |
| `devclean stats` | Show cumulative disk space reclaimed over time |
| `devclean init` | Generate initial `config.yaml` configuration file |
| `devclean mcp` | Start Model Context Protocol (MCP) server for AI agents |
| `devclean version` | Show current version and build information |

### Global Flags

| Flag | Description |
| :--- | :--- |
| `--config <path>` | Path to custom config file (default: `~/.config/devclean/config.yaml`) |
| `--profile <name>` | Apply named configuration profile from `profiles/<name>.yaml` |
| `--verbose` | Enable verbose diagnostic logging |
| `--quiet` | Suppress non-essential output |

### `clean` Flags

| Flag | Description |
| :--- | :--- |
| `--auto` | Delete targets older than configured TTL rules |
| `--force` | Delete targets regardless of age (bypasses TTL) |
| `--dry-run` | Simulate deletion and preview space without removing files |
| `-e, --ecosystem` | Filter cleanup to a specific ecosystem |
| `--smart` | Only delete targets exceeding heuristic score threshold (with `--auto`) |
| `--trash` | Move targets to trash instead of permanent deletion |
| `--permanent` | Force permanent removal even when trash is globally enabled |
| `--all` | Set zero TTL across all ecosystems for this run |
| `--format` | Output format: `table` (default) or `json` |

### `purge` Flags

| Flag | Description |
| :--- | :--- |
| `--inactive-days <n>` | Minimum days without commits to treat repo as inactive (default: `60`) |
| `--merged-only` | Purge only repos whose branch is merged into `main`/`master` |
| `--dry-run` | Preview targets to be purged without deleting |
| `-y, --yes` | Skip interactive confirmation prompt |
| `--trash` | Move purged artifacts to trash |
| `--permanent` | Force permanent deletion of purged artifacts |
| `--format` | Output format: `table` (default) or `json` |

### `whitelist` Subcommands

| Subcommand | Description |
| :--- | :--- |
| `whitelist list` | Show excluded paths, protected patterns, and immutable blacklist |
| `whitelist add <path>` | Add canonical path to `exclude_paths` in `config.yaml` |
| `whitelist add --pattern <glob>` | Add glob pattern to `protected_patterns` in `config.yaml` |
| `whitelist remove <path>` | Remove path from `exclude_paths` |
| `whitelist remove --pattern <glob>` | Remove pattern from `protected_patterns` |

---

## Supported Ecosystems

| Ecosystem | Scope & What It Cleans | Risk |
| :--- | :--- | :--- |
| **Docker** | Dangling images, orphan volumes, build cache (via CLI/API) | Low–High |
| **Go** | `$GOMODCACHE` (`~/go/pkg/mod`), `$GOCACHE` (`~/.cache/go-build`) | Low |
| **Node.js** | `node_modules` across project scan paths | Medium |
| **.NET** | Global NuGet packages/cache, project `bin/` and `obj/` | Low–Medium |
| **Python** | Global uv cache (`~/.cache/uv`), `.venv`, `__pycache__` | Low |
| **Rust** | Project `target/` directories containing `Cargo.toml` | Medium |
| **Java / Kotlin** | Gradle caches, Maven repo (`~/.m2`), project `build/` & `target/` | Low–Medium |
| **AI Tools** | Claude Code cache, VS Code / Cursor caches & logs, Ollama orphan blobs, HuggingFace hub | Low |
| **pnpm** | Global pnpm content-addressable store (`pnpm store prune`) | Low |
| **Bun** | Bun global install cache (`~/.bun/install/cache`) | Low |
| **Next.js** | Dedicated `.next/cache` build cache directories | Low |
| **Xcode** | `~/Library/Developer/Xcode/DerivedData` and unavailable iOS simulators | Low |
| **Homebrew** | Formula & cask download cache (`brew cleanup -s --prune=all`) | Low |

---

## Configuration

Default configuration file location: `~/.config/devclean/config.yaml`

```yaml
scan_paths:
  - ~/Code
  - ~/Work
  - ~/Projects

exclude_paths:
  - ~/Work/critical-production-service

protected_patterns:
  - "*/proprietary/*"
  - "*/sensitive/*"
  - "*/do-not-delete/*"

ecosystems:
  docker:
    enabled: true
    max_age_days: 30
    fail_mode: continue
  node:
    enabled: true
    max_age_days: 14
    fail_mode: continue
  go:
    enabled: true
    max_age_days: 60
    fail_mode: continue
  dotnet:
    enabled: true
    max_age_days: 45
    fail_mode: continue
  python:
    enabled: true
    max_age_days: 30
    fail_mode: continue
  rust:
    enabled: true
    max_age_days: 30
    fail_mode: continue
  java:
    enabled: true
    max_age_days: 30
    fail_mode: continue
  ai:
    enabled: true
    max_age_days: 14
    fail_mode: continue
  pnpm:
    enabled: true
    max_age_days: 30
    fail_mode: continue
  bun:
    enabled: true
    max_age_days: 30
    fail_mode: continue
  nextjs:
    enabled: true
    max_age_days: 7
    fail_mode: continue
  xcode:
    enabled: true
    max_age_days: 14
    fail_mode: continue
  homebrew:
    enabled: true
    max_age_days: 30
    fail_mode: continue

safety:
  trash_enabled: true
  retention_days: 7
  max_size_mb: 10240

watch:
  interval: 6h
  threshold_mb: 5120
```

See [docs/configuration.md](docs/configuration.md) for complete options and profiles reference.

---

## Safety Guarantees

1. **Immutable Core Blacklist** — `.git`, `.env`, `.ssh`, `secrets.json`, and core system paths cannot be modified or deleted under any circumstance (even with `--force`). Attempting to remove them via `devclean whitelist remove` is rejected.
2. **Git Dirty Tree Protection** — `devclean purge` inspects Git working trees and automatically skips any repository with uncommitted or unstaged changes.
3. **Virtual Target Isolation** — Non-filesystem targets (`docker://`, `pnpm://`, `homebrew://`) safely bypass trash staging without causing filesystem I/O errors.
4. **Symlink & Network Mount Guard** — Evaluates paths using `os.Lstat` without traversing symlinks; network automounts (`/net/` on macOS) are bypassed.

See [docs/safety.md](docs/safety.md) for full details.

---

## Documentation

- [Cleaners Guide](docs/cleaners.md) — Exhaustive details on all 13 cleaners, targets, and mechanisms
- [Configuration Reference](docs/configuration.md) — Complete YAML schema, defaults, hooks, and profiles
- [Safety Architecture](docs/safety.md) — Blacklist, risk scoring, trash staging, and Git guards
- [Interactive TUI](docs/tui.md) — Keybindings, search, filtering, and navigation
- [Scheduling & Automation](docs/scheduling.md) — Launchd & Cron setup, watch daemon
- [MCP Server](docs/mcp.md) — Model Context Protocol integration for AI agents

---

## Requirements

- Go 1.25+ (for source builds)
- macOS or Linux (amd64 / arm64)
- Optional CLI tools detected automatically: Docker, uv, pnpm, bun, xcode/xcrun, brew

## License

MIT

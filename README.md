# DevClean

A high-performance CLI tool and interactive TUI that reclaims disk space across polyglot development environments. Scans caches, build artifacts, and orphaned dependencies across **22 ecosystems** in parallel.

---

## Features

- **22 Ecosystems Supported** — Docker, Go, Node.js, .NET, Python, Rust, Java/Kotlin, AI Tools & Editors (Claude, VS Code, Cursor, Ollama, HuggingFace, Antigravity), pnpm, Bun, Next.js, Xcode, Homebrew, C++, Flutter, Terraform, CocoaPods, Elixir, Shell, macOS System & User Caches, Web Browsers, and Application Leftovers & LaunchAgents.
- **Interactive TUI** — Fullscreen [Bubble Tea](https://github.com/charmbracelet/bubbletea) interface with fuzzy filtering (`/`), multi-criteria sorting (`s`), risk filtering (`r`), mass selection (`m`), smart mode recommendations (`⚡`/`🛡️`), and instant real-time space calculation.
- **Visual Disk Explorer (`devclean analyze`)** — Ncdu-style full-screen directory navigator with concurrent sizing, Lip Gloss proportion bars, large files detection, and protected trash deletion.
- **TUI Configuration Manager (`devclean config`)** — 5-tab interactive settings manager with YAML AST comment preservation and live OS daemon synchronization.
- **Installer Sanitizer (`devclean installer`)** — Automated discovery and cleaning of orphaned disk images (`.dmg`, `.iso`), installer packages (`.pkg`, `.deb`, `.rpm`), and installation archives.
- **Cleanup Presets** — `--preset safe` (restricts to global low-risk caches, sparing project workspaces), `--preset standard` (configured TTLs), and `--preset aggressive` (authoritative 0d TTL across all 22 ecosystems).
- **CLI Purge** — `devclean purge` expunges build artifacts and caches from inactive or merged Git repositories while automatically preserving dirty working trees.
- **Whitelist & Blacklist Governance** — `devclean whitelist` CLI command to add, list, and remove exclusions in `config.yaml`, strictly guarded by the immutable system blacklist (`.git`, `.env`, `.ssh`, `secrets.json`).
- **Visual Lip Gloss Report Card** — Beautiful post-cleanup summary with real filesystem disk delta via Statfs (zero CGO dependencies), proportional mini-bars per ecosystem, and machine-readable `--format json` support.
- **Unified Deletion Coordinator** — Centralized architecture ensuring consistent safety, pre/post deletion hooks, history recording, and virtual target isolation across CLI, TUI, and MCP.
- **Trash Staging & Instant Recovery** — Safely stage deleted files to trash with UUID-indexed manifests, quota caps, retention policies, and restore them on demand (`devclean trash restore`).
- **Heuristic Smart Scoring** — Intelligent reclaim scoring (0.0–1.0) taking into account artifact age, size, Git commit activity, and branch merge status.
- **Background Watch Daemon & Scheduling** — Continuous filesystem threshold monitoring with desktop alerts (`devclean watch`) and native OS scheduling via Systemd User Timers (Linux) or Launchd (macOS).
- **MCP Server for AI Agents** — Model Context Protocol (stdio JSON-RPC) server exposing 8 scan, proposal, explanation, statistics, and deletion tools to AI agents like Claude Code and Cursor.
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

# Clean using safe preset (global caches only, skips project builds)
devclean clean --auto --preset safe

# Force clean all discovered targets (bypass TTL)
devclean clean --force

# Preview cleanup without deleting anything (dry-run)
devclean clean --auto --dry-run

# Output cleanup report as JSON
devclean clean --auto --format json

# Interactive visual disk explorer (ncdu-style)
devclean analyze

# Manage settings in full-screen TUI
devclean config

# Find and clean orphaned installer archives (.dmg, .pkg, .iso)
devclean installer

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

# Schedule recurring daily cleanup via systemd/launchd
devclean schedule --daily
```

---

## Commands

| Command | Description |
| :--- | :--- |
| `devclean` | Launch interactive TUI (default entrypoint) |
| `devclean scan` | Scan ecosystems and display targets without deleting |
| `devclean clean` | Clean targets (`--auto` with TTL, `--force`, or `--preset`) |
| `devclean analyze [path]` | Interactive visual disk explorer (ncdu style) |
| `devclean purge` | Purge build artifacts from inactive or merged Git repositories |
| `devclean installer` | Scan and clean orphaned installer files and disk images |
| `devclean config` | Interactive TUI configuration manager |
| `devclean whitelist` | View, add, or remove paths and glob patterns from whitelist |
| `devclean trash` | Manage trash staging (`list`, `restore <id>`, `clean --expired`, `empty`) |
| `devclean schedule` | Register recurring cleanup via Systemd (Linux) or Launchd (macOS) |
| `devclean schedule run` | Immediately trigger scheduled cleanup execution |
| `devclean schedule logs` | View, stream, or clear scheduler execution audit logs |
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
| `--preset <preset>` | Apply cleanup risk preset: `safe`, `standard` (default), or `aggressive` |
| `--smart` | Only delete targets exceeding heuristic score threshold (with `--auto`) |
| `--trash` | Move targets to trash instead of permanent deletion |
| `--permanent` | Force permanent removal even when trash is globally enabled |
| `--all` | Set zero TTL across all 22 ecosystems for this run |
| `--format` | Output format: `table` (default) or `json` |

### `schedule` Flags

| Flag | Description |
| :--- | :--- |
| `--daily` | Register cleanup daily at noon |
| `--weekly` | Register cleanup weekly on Sunday at midnight |
| `--hourly` | Register cleanup every hour |
| `--at <HH:MM>` | Register cleanup at specified time of day |
| `--cron <expr>` | Custom 5-field cron expression |
| `--watch` | Register threshold watch daemon rather than batch cleanup |
| `--status` | Display current schedule and last run info |

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
| **AI Tools** | Claude Desktop/Code, VS Code / Cursor / VSCodium / Positron caches & logs, Ollama blobs, HuggingFace hub, Antigravity browser profile | Low |
| **pnpm** | Global pnpm content-addressable store (`pnpm store prune`) | Low |
| **Bun** | Bun global install cache (`~/.bun/install/cache`) | Low |
| **Next.js** | Dedicated `.next/cache` build cache directories | Low |
| **Xcode** | `DerivedData`, unavailable iOS simulators, Xcode App Cache (`com.apple.dt.Xcode`), CoreSimulator logs | Low |
| **Homebrew** | Formula & cask download cache (`brew cleanup -s --prune=all`) | Low |
| **C++** | Conan cache (`~/.conan2/p`), vcpkg downloads/installed, project `build/` and `cmake-build-*` | Low–Medium |
| **Flutter** | Pub cache (`~/.pub-cache`), `.dart_tool`, project `build/` artifacts | Low–Medium |
| **Terraform** | `.terraform/providers` caches and plan files across scan paths | Low |
| **CocoaPods** | `~/Library/Caches/CocoaPods`, project `Pods/` directories | Low–Medium |
| **Elixir** | Hex package cache (`~/.hex/packages`), project `_build` and `deps` | Low–Medium |
| **Shell** | Zsh completion caches (`~/.zcompdump*`), history backups (0d TTL) | Low |
| **macOS** | User caches (`~/Library/Caches`), system logs (`~/Library/Logs`), Siri Suggestions, sandboxed daemon temporary stores | Low |
| **Browsers** | Chromium (Chrome, Brave, Arc, Edge) deep cache & on-device AI models, Firefox, Safari | Low |
| **Leftovers** | Uninstalled app remnants in `~/Library/Application Support`, orphaned LaunchAgents | Low |

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
  cpp:
    enabled: true
    max_age_days: 30
    fail_mode: continue
  flutter:
    enabled: true
    max_age_days: 30
    fail_mode: continue
  terraform:
    enabled: true
    max_age_days: 30
    fail_mode: continue
  cocoapods:
    enabled: true
    max_age_days: 14
    fail_mode: continue
  elixir:
    enabled: true
    max_age_days: 14
    fail_mode: continue
  shell:
    enabled: true
    max_age_days: 0
    fail_mode: continue
  macos:
    enabled: true
    max_age_days: 0
    fail_mode: continue
  browsers:
    enabled: true
    max_age_days: 0
    fail_mode: continue
  leftovers:
    enabled: true
    max_age_days: 0
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
2. **Process Safety Guards** — Fail-closed protection checks if target applications (browsers, IDEs, Xcode, daemons) are actively running before cleaning to prevent lock conflicts or corruption.
3. **Consolidated SubPath Leaf Unlinking** — For container directories (e.g. `~/Library/Caches`, browser profiles, sandboxed containers), DevClean cleans only constituent leaf sub-paths without destroying parent roots or unmanaged siblings.
4. **Git Dirty Tree Protection** — `devclean purge` inspects Git working trees and automatically skips any repository with uncommitted or unstaged changes.
5. **App Leftovers Multi-Source Verification** — Quad-source verification combining static exclusions, pure-Go `/var/db/receipts` indexing, Homebrew Cask/Cellar indexing, CLI lookups, and Spotlight ensures no installed software is misidentified.
6. **Inviolable Leftovers Deletion Defense** — Refusal guard in leftovers cleaner returns `nil` to prevent accidental removal of user application data.
7. **Virtual Target Isolation** — Non-filesystem targets (`docker://`, `pnpm://`, `homebrew://`) safely bypass trash staging without causing filesystem I/O errors.
8. **Symlink & Network Mount Guard** — Evaluates paths using `os.Lstat` without traversing symlinks; network automounts (`/net/` on macOS) are bypassed.

See [docs/safety.md](docs/safety.md) for full details.

---

## Documentation

- [Cleaners Guide](docs/cleaners.md) — Exhaustive details on all 22 cleaners, targets, and mechanisms
- [Configuration Reference](docs/configuration.md) — Complete YAML schema, defaults, presets, hooks, and profiles
- [Safety Architecture](docs/safety.md) — Blacklist, process guards, subpath unlinking, trash staging, and Git guards
- [Interactive TUI](docs/tui.md) — Keybindings, visual disk analyzer (`analyze`), TUI purge, and settings manager (`config`)
- [Scheduling & Automation](docs/scheduling.md) — Systemd user timers, Launchd, logs, and watch daemon
- [MCP Server](docs/mcp.md) — Model Context Protocol integration for AI agents (8 tools across 22 ecosystems)

---

## Requirements

- Go 1.25+ (for source builds)
- macOS or Linux (amd64 / arm64)
- Optional CLI tools detected automatically: Docker, uv, pnpm, bun, xcode/xcrun, brew

## License

MIT
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

# Cleaners

DevClean ships with **13 ecosystem cleaners** that execute in parallel during discovery. Each cleaner targets ecosystem-specific cache and artifact directories while adhering to strict safety rules.

---

## Overview

| Ecosystem | What It Cleans | Risk | Rebuild Ease |
| :--- | :--- | :--- | :--- |
| **Docker** | Dangling images, orphan volumes, build cache | Low–High | Trivial (cache/images), Irreversible (volumes) |
| **Go** | Module cache (`pkg/mod`), build cache (`go-build`) | Low | Automatic (`go build` / `go mod download`) |
| **Node.js** | Project `node_modules` | Medium | Fast to moderate (`npm install` / `yarn install`) |
| **.NET** | Global NuGet caches, local `bin/` and `obj/` | Low–Medium | Fast (`dotnet restore` / `dotnet build`) |
| **Python** | uv cache (`~/.cache/uv`), `.venv`, `__pycache__` | Low | Automatic (`uv sync` / `uv pip install`) |
| **Rust** | Project `target/` directories containing `Cargo.toml` | Medium | Moderate (`cargo build`) |
| **Java / Kotlin** | Gradle caches, Maven repo, project `build/` & `target/` | Low–Medium | Moderate (`gradle build` / `mvn package`) |
| **AI & Editors** | Claude Code cache, VS Code/Cursor caches, Ollama blobs, HuggingFace hub | Low | Re-cached on demand by respective tools |
| **pnpm** | Global content-addressable store (`pnpm store prune`) | Low | Restored via `pnpm install` |
| **Bun** | Global install cache (`~/.bun/install/cache`) | Low | Re-downloaded on next `bun install` |
| **Next.js** | Project-specific `.next/cache` build outputs | Low | Rebuilt on next `next build` / `next dev` |
| **Xcode** | `DerivedData` build caches and unavailable iOS simulators | Low | Rebuilt on next Xcode compile |
| **Homebrew** | Formula & cask download cache (`brew cleanup -s --prune=all`) | Low | Packages remain installed; cache re-downloaded if needed |

---

## 1. Docker

Targets Docker daemon artifacts that accumulate over time.

### Targets

| Category | Description | Target URI / Path | Risk |
| :--- | :--- | :--- | :--- |
| **Dangling images** | Unreferenced images tagged `<none>:<none>` | `docker://image/<id>` | Low |
| **Orphan volumes** | Anonymous volumes not attached to running containers | `docker://volume/<name>` | High |
| **Build cache** | BuildKit layer cache | `docker://build-cache` | Low |

### How It Works

- Communicates with the Docker daemon via Docker SDK API with CLI fallback (`docker system prune`).
- Implements virtual target handling (`docker://` URI scheme): deletion coordinates directly with daemon APIs without staging to filesystem trash.
- 30-second timeout per scan operation.

### Risk Rationale

- Volumes are rated **High** because they might store persistent application state (databases, user uploads).
- Dangling images and build cache are **Low** because layers can be re-pulled or rebuilt deterministically.

---

## 2. Go

Cleans global Go compilation and module download caches.

### Targets

| Category | Path | Risk |
| :--- | :--- | :--- |
| **Module cache** | `$GOMODCACHE` (default: `~/go/pkg/mod`) | Low |
| **Build cache** | `$GOCACHE` (default: `~/.cache/go-build`) | Low |

### How It Works

- Resolves exact paths via `go env GOMODCACHE` and `go env GOCACHE` with platform-appropriate fallbacks.
- Modifies permissions (`chmod 0o666`) before deletion because Go stores module files as read-only.
- Independent of `scan_paths` — discovers caches globally.

### Risk Rationale

- Rated **Low**; dependencies are re-downloaded on the next `go mod download`, and compilation cache is repopulated on `go build`.

---

## 3. Node.js

Discovers and cleans project-level dependency folders.

### Targets

| Category | Path | Risk |
| :--- | :--- | :--- |
| **node_modules** | `<scan_path>/**/node_modules` | Medium |

### How It Works

- Traverses configured `scan_paths` with bounded concurrency (`runtime.NumCPU()` goroutines).
- Uses `filepath.SkipDir` upon encountering `node_modules` to avoid redundant traversal into nested dependency trees.
- Validates that parent directories do not violate blacklist or exclusion rules.

### Risk Rationale

- Rated **Medium**; while no source code is lost, re-running `npm install` or `yarn install` requires network bandwidth and time.

---

## 4. .NET

Cleans both global NuGet caches and per-project build outputs.

### Targets

| Category | Path | Risk |
| :--- | :--- | :--- |
| **NuGet packages** | `~/.nuget/packages` | Low |
| **HTTP cache** | `~/.local/share/NuGet/http-cache` | Low |
| **Temp files** | `~/.local/share/NuGet/temp` | Low |
| **Build outputs** | `bin/` adjacent to `.csproj`, `.fsproj`, or `.sln` | Medium |
| **Intermediate files** | `obj/` adjacent to `.csproj`, `.fsproj`, or `.sln` | Medium |

### How It Works

- Global caches are discovered using platform resolvers.
- Local `bin/` and `obj/` directories are only flagged if an adjacent project manifest (`.csproj`, `.fsproj`, `.sln`) is detected.

### Risk Rationale

- Global package caches are **Low** (re-downloaded on `dotnet restore`). Project `bin/obj` are **Medium** because full recompilation is required.

---

## 5. Python (uv & Virtual Environments)

Cleans Astral `uv` caches and Python project artifacts.

### Targets

| Category | Path | Risk |
| :--- | :--- | :--- |
| **uv global cache** | Platform uv cache (default: `~/.cache/uv`) | Low |
| **Virtualenvs** | `<scan_path>/**/.venv` | Medium |
| **Bytecode cache** | `<scan_path>/**/__pycache__` | Low |

### How It Works

- Resolves `uv` cache using the `UV_CACHE_DIR` environment variable or standard OS cache paths.
- Scans `scan_paths` for `.venv` and `__pycache__` directories.

### Risk Rationale

- Global cache and bytecode are **Low**. Virtual environments (`.venv`) are **Medium** because lockfile synchronization is required to recreate them.

---

## 6. Rust (Cargo)

Cleans build artifact folders from Cargo projects.

### Targets

| Category | Path | Risk |
| :--- | :--- | :--- |
| **Build targets** | `<scan_path>/**/target` | Medium |

### How It Works

- Scans configured `scan_paths` for `target/` directories.
- Confirms the directory belongs to a Rust project by checking for an adjacent `Cargo.toml`.
- Uses `SkipDir` to avoid recursing inside the build tree.

### Risk Rationale

- Rated **Medium**; source code is untouched, but rebuilding large Rust crates can consume significant CPU time.

---

## 7. Java / Kotlin (Gradle & Maven)

Cleans global build caches and local compilation artifacts.

### Targets

| Category | Path | Risk |
| :--- | :--- | :--- |
| **Gradle caches** | `~/.gradle/caches` | Low |
| **Maven repository** | `~/.m2/repository` | Low |
| **Gradle build dir** | `build/` adjacent to `build.gradle` / `build.gradle.kts` | Medium |
| **Maven target dir** | `target/` adjacent to `pom.xml` | Medium |

### How It Works

- Resolves `~/.gradle` and `~/.m2` home directories.
- Inspects `scan_paths` for project root markers (`pom.xml`, `build.gradle`) before marking local `target/` or `build/` folders.

### Risk Rationale

- Global repository caches are **Low** (re-downloaded automatically). Local outputs are **Medium** because clean compilation is required.

---

## 8. AI Tools & Editors

Cleans caches, telemetry logs, and orphan model blobs left by modern developer AI tools.

### Targets

| Category | Path | Risk |
| :--- | :--- | :--- |
| **Claude Code** | `~/.claude/cache`, `~/.claude/todos` (whitelists config/auth) | Low |
| **VS Code & Cursor** | `CachedData`, `logs`, `.obsolete` in editor config directories | Low |
| **Ollama** | Orphan blobs under `~/.ollama/models/blobs` not referenced by manifests | Low |
| **HuggingFace** | Hub snapshots under `~/.cache/huggingface/hub` older than TTL | Low |

### How It Works

- **Claude Code**: Preserves authentication credentials (`~/.claude/credentials.json`) while purging transient caches and session metadata.
- **VS Code / Cursor**: Discovers application cache directories across macOS (`~/Library/Application Support/Code`) and Linux (`~/.config/Code`).
- **Ollama**: Parses model manifest JSON files to identify and remove unreferenced orphan SHA256 blob layers.
- **HuggingFace**: Identifies model snapshot directories and applies configured TTL thresholds.

### Risk Rationale

- Rated **Low**; configuration and model manifests are preserved, and transient caches are regenerated on next run.

---

## 9. pnpm

Cleans the global pnpm content-addressable package store.

### Targets

| Category | Target URI | Risk |
| :--- | :--- | :--- |
| **pnpm Store** | `pnpm://store-prune` (virtual target) | Low |

### How It Works

- Checks if `pnpm` CLI is available.
- Registers a virtual target that invokes `pnpm store prune` via the command executor.
- Bypasses filesystem trash staging safely to avoid I/O errors on virtual targets.

### Risk Rationale

- Rated **Low**; unreferenced packages in the store are removed, while packages currently linked in active projects remain intact.

---

## 10. Bun

Cleans the global package installation cache of the Bun runtime.

### Targets

| Category | Path | Risk |
| :--- | :--- | :--- |
| **Bun install cache** | `~/.bun/install/cache` | Low |

### How It Works

- Locates the default Bun cache directory or resolves `BUN_INSTALL_CACHE_DIR`.
- Cleans stale package archives.

### Risk Rationale

- Rated **Low**; packages are automatically re-fetched during subsequent `bun install` executions.

---

## 11. Next.js

Cleans isolated Next.js compiler and bundling caches across projects.

### Targets

| Category | Path | Risk |
| :--- | :--- | :--- |
| **Next.js cache** | `<scan_path>/**/.next/cache` | Low |

### How It Works

- Restricts scanning strictly to `.next/cache` (leaving server bundles, static pages, and standalone outputs in `.next` untouched).
- Validates the presence of `package.json` or `next.config.{js,mjs,ts}` in the repository root.

### Risk Rationale

- Rated **Low**; webpack and SWC build caches are repopulated during the next `next dev` or `next build` cycle.

---

## 12. Xcode (macOS)

Reclaims storage from Apple developer tools and iOS simulators.

### Targets

| Category | Path / URI | Risk |
| :--- | :--- | :--- |
| **DerivedData** | `~/Library/Developer/Xcode/DerivedData` | Low |
| **Unavailable Simulators** | `xcode://simulators/unavailable` (virtual target) | Low |

### How It Works

- Discovers the default Xcode `DerivedData` directory.
- Detects broken, orphaned, or unavailable iOS/watchOS/tvOS runtime simulators and prunes them via `xcrun simctl delete unavailable`.

### Risk Rationale

- Rated **Low**; Xcode automatically indexes and regenerates `DerivedData` upon project build.

---

## 13. Homebrew (macOS & Linux)

Cleans downloaded bottle archives and stale cache files.

### Targets

| Category | Target URI / Path | Risk |
| :--- | :--- | :--- |
| **Homebrew cache** | `~/Library/Caches/Homebrew` or `~/.cache/Homebrew` | Low |
| **Brew cleanup** | `homebrew://cleanup` (virtual target) | Low |

### How It Works

- Locates standard Homebrew cache directories across macOS and Linux.
- Optionally invokes `brew cleanup -s --prune=all` via virtual target execution to purge outdated download archives.

### Risk Rationale

- Rated **Low**; installed software is unaffected; only cached `.bottle.tar.gz` download installers are deleted.

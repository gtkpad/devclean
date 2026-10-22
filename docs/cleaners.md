# Cleaners

DevClean ships with **22 ecosystem cleaners** that execute in parallel during discovery. Each cleaner targets ecosystem-specific cache and artifact directories while adhering to strict safety rules.

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
| **AI & Editors** | Claude Desktop/Code, VS Code / Cursor / VSCodium / Positron, Ollama, HuggingFace, Antigravity | Low | Re-cached on demand by respective tools |
| **pnpm** | Global content-addressable store (`pnpm store prune`) | Low | Restored via `pnpm install` |
| **Bun** | Global install cache (`~/.bun/install/cache`) | Low | Re-downloaded on next `bun install` |
| **Next.js** | Project-specific `.next/cache` build outputs | Low | Rebuilt on next `next build` / `next dev` |
| **Xcode** | `DerivedData`, simulators, App Cache, and CoreSimulator logs | Low | Rebuilt on next Xcode compile / run |
| **Homebrew** | Formula & cask download cache (`brew cleanup -s --prune=all`) | Low | Packages remain installed; cache re-downloaded if needed |
| **C++** | Conan packages (`~/.conan2/p`), vcpkg downloads/installed, project `build/` | Low–Medium | Rebuilt via CMake/Ninja or re-fetched by package manager |
| **Flutter** | Pub cache (`~/.pub-cache`), `.dart_tool`, project `build/` | Low–Medium | Rebuilt via `flutter pub get` and `flutter build` |
| **Terraform** | `.terraform/providers` cache and temporary plan outputs | Low | Re-downloaded on next `terraform init` |
| **CocoaPods** | Global cache (`~/Library/Caches/CocoaPods`), project `Pods/` | Low–Medium | Reinstalled on `pod install` |
| **Elixir** | Hex package cache (`~/.hex/packages`), project `_build` & `deps` | Low–Medium | Recompiled via `mix deps.get` and `mix compile` |
| **Shell** | Zsh completion dumps (`~/.zcompdump*`), history backup files | Low | Regenerated automatically on next shell launch (0d TTL) |
| **macOS** | User caches (`~/Library/Caches`), system logs (`~/Library/Logs`), Siri Suggestions, daemon temp | Low | Cleansed safely with child-only unlinking (0d TTL) |
| **Browsers** | Chromium (Chrome, Brave, Arc, Edge) deep cache/models, Firefox, Safari | Low | Re-cached by browser; sessions and extensions protected (0d TTL) |
| **Leftovers** | Uninstalled app remnants in `~/Library/Application Support`, orphaned LaunchAgents | Low | Cleaned only after multi-source non-installed verification |

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

Cleans caches, telemetry logs, and orphan model blobs left by modern developer AI tools, coding assistants, and local AI runtimes.

### Targets

| Category | Path | Risk |
| :--- | :--- | :--- |
| **Claude Code & Desktop** | `~/.claude/cache`, `~/.claude/todos`, `~/Library/Logs/Claude` | Low |
| **VS Code & Derivatives** | `CachedData`, `logs`, `.obsolete`, `WebStorage/*/CacheStorage` in VS Code, VS Code Insiders, Cursor, VSCodium, and Positron | Low |
| **Antigravity Profile** | `~/.gemini/antigravity-browser-profile` (`Cache`, `Code Cache`, `GPUCache`, `DawnGraphiteCache`, `Service Worker/CacheStorage`) | Low |
| **Ollama** | Orphan blobs under `~/.ollama/models/blobs` not referenced by manifests | Low |
| **HuggingFace** | Hub snapshots under `~/.cache/huggingface/hub` older than TTL | Low |

### How It Works

- **Claude**: Preserves authentication credentials (`~/.claude/credentials.json`) while purging transient caches and session metadata. Scans Claude Desktop application logs with immediate retention eligibility.
- **VS Code Ecosystem**: Discovers application cache and storage directories across macOS (`~/Library/Application Support/*`) and Linux (`~/.config/*`). Cleans stale `CacheStorage` within `WebStorage` partitions.
- **Antigravity Browser Profile**: Aggregates Chromium-based profile ephemeral caches under `~/.gemini/antigravity-browser-profile` using surgical child-only unlinking.
- **Process Guard**: Checks whether editor processes (`Code`, `Cursor`, `VSCodium`, `Positron`) are running and safely defers cleaning of active instances.
- **Ollama**: Parses model manifest JSON files to identify and remove unreferenced orphan SHA256 blob layers.
- **HuggingFace**: Identifies model snapshot directories and applies configured TTL thresholds.

### Risk Rationale

- Rated **Low**; configuration, credentials, installed extensions, and active model manifests are strictly preserved.

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

Reclaims storage from Apple developer tools, build caches, and iOS simulators.

### Targets

| Category | Path / URI | Risk |
| :--- | :--- | :--- |
| **DerivedData** | `~/Library/Developer/Xcode/DerivedData` | Low |
| **Xcode App Cache** | `~/Library/Caches/com.apple.dt.Xcode` | Low |
| **CoreSimulator Logs** | `~/Library/Logs/CoreSimulator` (child-only unlinking) | Low |
| **CoreSimulator Temp & Caches** | `~/Library/Developer/CoreSimulator/Caches`, `Temp` | Low |
| **DeviceSupport** | `~/Library/Developer/Xcode/iOS DeviceSupport`, `watchOS DeviceSupport` | Low |
| **Unavailable Simulators** | `xcode://simulators/unavailable` (virtual target) | Low |
| **Xcode Archives** | `~/Library/Developer/Xcode/Archives` | High |

### How It Works

- Scans Xcode `DerivedData` and iOS/watchOS device support caches.
- Integrates dedicated targets for Xcode App Cache and CoreSimulator Logs with immediate 0d retention eligibility.
- For CoreSimulator Logs, uses surgical child-only unlinking (`childOnly: true`) to preserve the parent logging folder.
- Prunes broken or unavailable simulators via `xcrun simctl delete unavailable`.
- Excluded from generic macOS user cache scanning to eliminate double-counting.

### Risk Rationale

- Derived data, simulator temp/logs, and caches are rated **Low**; Xcode automatically regenerates them on build. Archives are rated **High** to avoid accidental loss of release artifacts.

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

---

## 14. C++ (Conan, vcpkg & CMake)

Cleans package manager caches and local build artifacts for C and C++ projects.

### Targets

| Category | Path | Risk |
| :--- | :--- | :--- |
| **Conan Package Cache** | `~/.conan/data`, `~/.conan2/p` | Low |
| **vcpkg Downloads** | `~/.vcpkg/downloads`, `$VCPKG_ROOT/downloads` | Low |
| **Project vcpkg_installed** | `<scan_path>/**/vcpkg_installed` | Medium |
| **Project CMake/Build dirs** | `<scan_path>/**/build`, `cmake-build-debug`, `cmake-build-release` | Medium |

### How It Works

- Discovers toolchains via `CMakeLists.txt`, `Makefile`, `meson.build`, `conanfile.txt`, or `vcpkg.json`.
- Implements POSIX-compliant recursive permission elevation (`internal/fs.ChmodRecursive`) prior to deletion to safely handle read-only package trees (e.g., Conan package files mode `0400/0500`) without `EACCES` failures.

### Risk Rationale

- Global download caches are **Low**; local `build/` and `vcpkg_installed` directories are **Medium** because full recompilation is required.

---

## 15. Flutter

Cleans global Pub package caches and project-level Flutter compilation outputs.

### Targets

| Category | Path | Risk |
| :--- | :--- | :--- |
| **Pub Cache** | `~/.pub-cache/hosted`, `~/.pub-cache/git` (preserves `bin`) | Low |
| **Project .dart_tool** | `<scan_path>/**/.dart_tool` | Medium |
| **Project build dir** | `<scan_path>/**/build` (adjacent to `pubspec.yaml`) | Medium |
| **Ephemeral Engine Caches** | `~/.flutter` engine downloads | Low |

### How It Works

- Scans for Flutter project indicators (`pubspec.yaml`).
- Preserves the executable directory `~/.pub-cache/bin` while purging stale hosted package archives.

### Risk Rationale

- Rated **Low** for global caches (`flutter pub get` restores them) and **Medium** for project build folders.

---

## 16. Terraform

Cleans cached provider plugins, modules, and transient plan files across infrastructure repositories.

### Targets

| Category | Path | Risk |
| :--- | :--- | :--- |
| **Terraform Providers** | `<scan_path>/**/.terraform/providers` | Low |
| **Terraform Modules** | `<scan_path>/**/.terraform/modules` | Low |
| **Transient Plan Files** | `<scan_path>/**/tfplan`, `*.tfplan` | Medium |

### How It Works

- Discovers `.terraform` directories adjacent to `.tf` or `.tf.json` files.
- Strictly preserves local state files (`terraform.tfstate`, `terraform.tfstate.backup`) and `.terraform.lock.hcl`.

### Risk Rationale

- Rated **Low** for provider plugins (re-downloaded via `terraform init`); plan files are rated **Medium**.

---

## 17. CocoaPods (macOS)

Cleans the CocoaPods global cache and project-level Pods installations.

### Targets

| Category | Path | Risk |
| :--- | :--- | :--- |
| **CocoaPods Cache** | `~/Library/Caches/CocoaPods` | Low |
| **Project Pods dir** | `<scan_path>/**/Pods` (adjacent to `Podfile`) | Medium |

### How It Works

- Scans user cache for downloaded pod tarballs.
- Scans project paths containing `Podfile` and targets the generated `Pods/` directory.

### Risk Rationale

- Global cache is **Low**; project `Pods/` is **Medium** (`pod install` restores the directory).

---

## 18. Elixir / Erlang

Cleans Hex package caches and project-level Mix compilation artifacts.

### Targets

| Category | Path | Risk |
| :--- | :--- | :--- |
| **Hex Package Cache** | `~/.hex/packages`, `~/.hex/cache.ets` | Low |
| **Project _build dir** | `<scan_path>/**/_build` (adjacent to `mix.exs`) | Medium |
| **Project deps dir** | `<scan_path>/**/deps` (adjacent to `mix.exs`) | Medium |

### How It Works

- Scans `scan_paths` for `mix.exs` project roots.
- Identifies compiled BEAM byte code in `_build` and fetched dependencies in `deps`.

### Risk Rationale

- Global Hex caches are **Low**; local `_build` and `deps` are **Medium** (`mix deps.get` and `mix compile` restore them).

---

## 19. Shell

Cleans transient shell compilation dumps and editor history backups.

### Targets

| Category | Path | Risk |
| :--- | :--- | :--- |
| **Zsh Compdump** | `~/.zcompdump*` completion cache files | Low |
| **Bash History Backups** | `~/.bash_history.*.tmp`, `.bash_sessions/` | Low |
| **Fish Completion Cache** | `~/.cache/fish` | Low |

### How It Works

- Default `max_age_days: 0` (immediate eligibility).
- Scans user home directory for stale completion cache dumps.

### Risk Rationale

- Rated **Low**; shell completion dumps are automatically re-created on the next interactive shell startup.

---

## 20. macOS System & User Caches

Cleans disposable user-space caches, system logs, diagnostic reports, and sandboxed daemon temporary stores.

### Targets

| Category | Path | Risk |
| :--- | :--- | :--- |
| **User Caches** | `~/Library/Caches/*` (surgical child-only unlinking) | Low |
| **User Logs** | `~/Library/Logs/*` | Low |
| **User Trash** | `~/.Trash/*` | Low |
| **Apple Daemons** | `~/Library/Suggestions`, `mediaanalysisd/Data/tmp`, `geod/Data/tmp`, `GeoServices`, `shared-pasteboard` | Low |
| **Group Containers** | Approved allowlist under `~/Library/Group Containers` | Low |
| **Diagnostic Reports** | `~/Library/Logs/DiagnosticReports` (crash logs) | Low |

### How It Works

- Default `max_age_days: 0` for ephemeral caches and logs.
- Uses consolidated target handling: constituent leaf paths are tracked in `Target.SubPaths` and deleted surgically without deleting container roots.
- Deduplicates dedicated targets (Xcode, AI editors, browsers) to eliminate overlapping scans and double-counting.
- Unconditionally unblocks always-on Apple daemons (e.g. `mediaanalysisd`, `geod`) from false `active_process` gating while retaining strict process gates on interactive applications.

### Risk Rationale

- Rated **Low**; targets are strictly ephemeral caches and diagnostics. Core user settings and application databases are excluded.

---

## 21. Web Browsers (macOS & Linux)

Discovers and cleans deep browser caches, shader stores, and on-device AI model weights across Chromium-based browsers, Firefox, and Safari.

### Targets

| Category | Path | Risk |
| :--- | :--- | :--- |
| **Chromium Family** | Google Chrome, Brave, Arc, Microsoft Edge | Low |
| **Deep Profile Caches** | `<Profile>/Cache`, `Code Cache`, `GPUCache`, `DawnGraphiteCache`, `Service Worker/CacheStorage` | Low |
| **Root Model & Shaders** | `OptGuideOnDeviceModel`, `GraphiteDawnCache`, `GrShaderCache` | Low |
| **Firefox** | `~/Library/Caches/Firefox/Profiles/*/cache2`, `~/.cache/mozilla/firefox` | Low |
| **Safari** | `~/Library/Caches/com.apple.Safari` | Low |

### How It Works

- Default `max_age_days: 0` for immediate cache eligibility parity.
- Traverses multi-profile directory layouts (`Default/`, `Profile 1/`, etc.) and discovers nested OS-level caches.
- Safeguards user sessions, cookies, history, saved passwords, and installed browser extensions (`Extensions/` and extension CRX caches are protected).
- Enforces strict process safety gating: if the browser process is running, scanning safely fails closed or defers cleaning.

### Risk Rationale

- Rated **Low**; only transient HTTP caches, GPU shaders, and discarded model weights are removed. User state remains intact.

---

## 22. Application Leftovers & LaunchAgents

Identifies and cleans residual data left behind by uninstalled applications and orphaned LaunchAgent daemon plists.

### Targets

| Category | Path | Risk |
| :--- | :--- | :--- |
| **Application Leftovers** | Orphaned folders under `~/Library/Application Support` | Low |
| **Orphaned LaunchAgents** | Unlinked `.plist` files in `~/Library/LaunchAgents` and `/Library/LaunchAgents` | Low |

### How It Works

- **Quad-Source Verification**: To ensure no active software is misidentified as an orphan, DevClean combines:
  1. Static exclusions catalog (shielding ~36+ GB of Apple system services and development runtimes).
  2. Pure-Go `/var/db/receipts` package receipt indexing.
  3. Pure-Go Homebrew Caskroom and Cellar package indexing.
  4. LookPath binary checks and memoized thread-safe Spotlight metadata queries (`kMDItemCFBundleIdentifier`).
- **Location-Aware LaunchAgent Ownership**: Correctly validates `root:wheel` (`UID 0`) ownership for `/Library/LaunchAgents` versus user ownership for `~/Library/LaunchAgents`.
- **ReviewOnly & Non-Deleting Defense**: All leftover targets are marked `ReviewOnly: true` (`ReviewReasonAppLeftover`). In `leftoversCleaner.Clean`, an inviolable refusal guard returns `nil` to prevent accidental deletion under any circumstances.

### Risk Rationale

- Rated **Low**; targets are orphaned residues, and defense-in-depth safety prevents unauthorized mutations.

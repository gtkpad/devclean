# Cleaners

DevClean ships with five ecosystem cleaners. Each runs in parallel during scan and targets specific cache/artifact directories.

## Overview

| Ecosystem | What it cleans                               | Risk       | Rebuilds easily?                 |
| --------- | -------------------------------------------- | ---------- | -------------------------------- |
| Docker    | Dangling images, orphan volumes, build cache | Low–High   | Yes (images/cache), No (volumes) |
| Go        | Module cache, build cache                    | Low        | Yes                              |
| Node      | `node_modules` directories                   | Medium     | Yes (npm/yarn install)           |
| .NET      | NuGet packages, HTTP cache, `bin/`, `obj/`   | Low–Medium | Yes                              |
| Python    | uv global cache                              | Low        | Yes                              |

## Docker

Targets Docker daemon artifacts that accumulate over time.

### Targets

| Category        | Description                                | Risk |
| --------------- | ------------------------------------------ | ---- |
| Dangling images | Images tagged `<none>:<none>`              | Low  |
| Orphan volumes  | Volumes not attached to running containers | High |
| Build cache     | Layer cache from `docker build`            | Low  |

### How it works

- Uses the Docker API natively with fallback to CLI (`docker system prune`)
- 30-second timeout per scan operation
- Requires Docker daemon to be running

### Risk rationale

Volumes are rated **High** because they may contain persistent data (databases, uploads). Images and build cache are **Low** because they can be rebuilt/re-pulled.

---

## Go

Cleans global Go caches that grow unbounded.

### Targets

| Category     | Path                                      | Risk |
| ------------ | ----------------------------------------- | ---- |
| Module cache | `$GOMODCACHE` (default: `~/go/pkg/mod`)   | Low  |
| Build cache  | `$GOCACHE` (default: `~/.cache/go-build`) | Low  |

### How it works

- Resolves paths via `go env GOMODCACHE` and `go env GOCACHE`
- Applies `chmod 0o666` before removal (Go modules are stored read-only)
- Independent of `scan_paths` — uses Go environment variables

### Risk rationale

Both caches are fully reconstructible. Module cache is re-downloaded on next `go mod download`; build cache is rebuilt on next `go build`.

---

## Node

Finds and cleans `node_modules` directories recursively within scan paths.

### Targets

| Category     | Path                          | Risk   |
| ------------ | ----------------------------- | ------ |
| node_modules | `<scan_path>/**/node_modules` | Medium |

### How it works

- Recursively walks `scan_paths` using `filepath.WalkDir`
- Short-circuits traversal with `SkipDir` when `node_modules` is found (doesn't descend into nested node_modules)
- Bounded parallelism: `runtime.NumCPU()` goroutines
- Each target identified by SHA256 hash of its path (first 6 bytes hex)

### Risk rationale

Rated **Medium** because restoration requires `npm install` / `yarn install` which can take 5–30 minutes depending on project size and network speed. No data is lost.

---

## .NET

Cleans both global NuGet caches and per-project build outputs.

### Global Targets

| Category       | Path                              | Risk |
| -------------- | --------------------------------- | ---- |
| NuGet packages | `~/.nuget/packages`               | Low  |
| HTTP cache     | `~/.local/share/NuGet/http-cache` | Low  |
| Temp files     | `~/.local/share/NuGet/temp`       | Low  |

### Local Targets (per scan_path)

| Category      | Path                                    | Risk   |
| ------------- | --------------------------------------- | ------ |
| Build outputs | `bin/` adjacent to .csproj/.fsproj/.sln | Medium |
| Intermediate  | `obj/` adjacent to .csproj/.fsproj/.sln | Medium |

### How it works

- Global caches are cleaned unconditionally (platform-resolved paths)
- Local `bin/` and `obj/` are only targeted when a project manifest (`.csproj`, `.fsproj`, or `.sln`) exists in the same or parent directory
- Skips `node_modules` and `.git` during traversal
- Bounded parallelism: `runtime.NumCPU()` goroutines

### Risk rationale

Global caches are **Low** (re-downloaded on next `dotnet restore`). Local `bin/obj` are **Medium** because they contain compiled assemblies that require a full rebuild.

---

## Python (uv)

Cleans the global cache of the [uv](https://github.com/astral-sh/uv) package manager.

### Targets

| Category | Path          | Risk |
| -------- | ------------- | ---- |
| uv cache | `~/.cache/uv` | Low  |

### How it works

- Checks if `uv` CLI is available or if cache directory exists
- Single directory removal

### Risk rationale

Rated **Low** — the cache is fully reconstructed on next `uv pip install` or `uv sync`.

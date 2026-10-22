# TUI (Terminal User Interface)

DevClean offers rich interactive terminal interfaces built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and styled with [Lip Gloss](https://github.com/charmbracelet/lipgloss).

---

## 1. Main Cleanup Interface (`devclean`)

Launched as the default entrypoint when running `devclean` without subcommands.

### State Flow

```
scanning ➔ selecting ➔ confirming ➔ deleting ➔ done
```

### Screens

#### Scanning Screen

```
⠙ Scanning ecosystems...
  • docker: done
  • node: done
  • rust: done
  • ai: scanning
```

A live spinner and per-ecosystem status indicators show progress as all **22 cleaners** execute in parallel.

#### Selecting Screen

```
DevClean                                        Selected: 12 targets (6.8 GB)
──────────────────────────────────────────────────────────────────────────────

── docker (3 targets, 2.3 GB) ────────────────────────────────────────────────
> [x] dangling-image   abc123f       2.1 GB   LOW
  [ ] orphan-volume    pgdata        200 MB   HIGH

── node (4 targets, 1.8 GB) ──────────────────────────────────────────────────
  [x] node_modules     ~/Work/web    850 MB   MED
  [x] node_modules     ~/Work/api    950 MB   MED

── ai (2 targets, 1.2 GB) ────────────────────────────────────────────────────
  [x] claude-cache     ~/.claude     400 MB   LOW
  [x] ollama-blobs     ~/.ollama     800 MB   LOW

[12/48] | Filter: none | Sort: size

↑↓/jk: navigate | space: toggle | /: filter | s: sort | r: risk | m: mass | l: low-risk | enter: confirm | q: quit
```

Features:
- Targets grouped by ecosystem with collapsible visual headers.
- Real-time selection counter displaying live aggregated byte totals.
- Smart mode indicators when enabled (`⚡` high reclaim score, `🛡️` preserve candidate).
- Fuzzy search bar (`/`) filtering paths and names in real time.
- Interactive multi-criteria sorting (`s`) and risk cycling (`r`).

#### Confirming Screen

```
Confirm deletion?

Targets: 12 selected
Space to free: 6.8 GB
Mode: Move to trash (~/.local/share/devclean/trash)

Paths:
  docker://image/abc123f
  ~/Work/web/node_modules
  ~/Work/api/node_modules
  ~/.claude/cache
  + 8 more

[y] confirm  [n] cancel
```

#### Done Screen (Report Card)

```
DevClean Cleanup Summary
──────────────────────────────────────────────────────────────────────────────
Freed: 6.8 GB across 12 targets (staged in trash)
Elapsed: 1.42s
Disk Space: 142.3 GB free ➔ 149.1 GB free (+6.8 GB)
```

Displays the post-cleanup summary with real Statfs disk recovery metrics and exits cleanly.

### Keybindings (Main Interface)

| Key | Action | Description |
| :--- | :--- | :--- |
| `j` / `↓` | Move down | Advance cursor to the next item |
| `k` / `↑` | Move up | Move cursor to previous item |
| `Tab` | Next group | Jump cursor directly to the start of the next ecosystem section |
| `Space` | Toggle | Select or deselect the highlighted target |
| `/` | Search filter | Open live fuzzy filter prompt to search by path or target name |
| `s` | Cycle sort | Cycle sorting criteria: **Size** ➔ **Path** ➔ **Risk** ➔ **Age** |
| `r` | Cycle risk | Cycle risk visibility: **All** ➔ **Low Only** ➔ **Medium Only** ➔ **High Only** |
| `m` | Mass select | Select all visible targets exceeding the mass threshold (e.g. >= 1 GB) |
| `l` | Low risk | Select only Low-risk targets (safe cache purge) |
| `a` | Select all | Select all currently visible targets |
| `n` | Deselect all | Clear all active selections |
| `Esc` | Reset | Exit search input mode and reset filters to default |
| `Enter` | Proceed | Advance to confirmation screen |
| `q` / `Ctrl-C` | Quit | Exit DevClean without making changes |

---

## 2. Visual Disk Explorer (`devclean analyze`)

An interactive visual disk usage explorer inspired by `ncdu`, designed for investigating deep directories, identifying disk hogs, and deleting items safely with trash staging.

```bash
# Explore current directory
devclean analyze

# Explore specific directory path
devclean analyze ~/Projects
```

```
devclean analyze: /Users/user/Projects
──────────────────────────────────────────────────────────────────────────────
   3.4 GB  [████████████████████]  /legacy-monorepo
   1.8 GB  [██████████          ]  /frontend-app
 850.0 MB  [█████               ]  /api-gateway
  42.0 MB  [                    ]  /mobile-app
   1.2 KB  [                    ]  .gitignore

[Total: 6.0 GB in 5 items] | Sort: Size | Depth: 1
↑/↓/jk: Navigate | Enter/l: Open | Esc/h: Parent | s: Sort | L: Large Files | d: Delete | q: Quit
```

### Key Features
- **Concurrent Sizing**: Background directory scanning with live progress spinners.
- **Proportion Bars**: Lip Gloss visual percentage bars showing relative size contribution.
- **Large Files View (`L`)**: Instantly inspect individual large files (>100 MB) across subdirectories.
- **Protected Trash Deletion (`d`)**: Deletes highlighted folders/files directly into DevClean's managed trash, protected by the immutable blacklist and confirmation dialogs.

### Keybindings (Analyzer)

| Key | Action | Description |
| :--- | :--- | :--- |
| `j` / `↓` | Down | Move cursor down |
| `k` / `↑` | Up | Move cursor up |
| `Enter` / `l` | Open | Drill into highlighted directory |
| `Esc` / `h` | Back | Return to parent directory |
| `s` | Sort | Toggle sort mode: **Size Descending** ➔ **Name Ascending** ➔ **Item Count** |
| `L` | Large files | Toggle large files discovery list |
| `d` | Delete | Stage highlighted directory/file into trash (with confirmation) |
| `?` | Help | Toggle keybindings help overlay |
| `q` / `Ctrl-C` | Quit | Exit disk analyzer |

---

## 3. Interactive Purge Interface (`devclean purge`)

When run in an interactive terminal, `devclean purge` launches a specialized Bubble Tea interface for cleaning Git repositories:

- **Repository Grouping**: Visual headers per repository showing latest commit date and branch merge status.
- **Dirty Tree Protection**: Repositories with uncommitted changes or active branch modifications are marked with security warnings and spared by default.
- **Fuzzy Search (`/`)**: Instantly filter repositories by folder name or remote repository name.

---

## 4. TUI Configuration Manager (`devclean config`)

Opens a full-screen 5-tab settings manager:

- **Tab 1: Ecosystems** — Enable/disable any of the 22 cleaners and adjust individual `max_age_days`.
- **Tab 2: Scan Paths** — Add or remove directories searched during project traversal.
- **Tab 3: Safety & Trash** — Configure trash quota limits, auto-expiration days, and view the immutable blacklist.
- **Tab 4: Watcher** — Configure background threshold alerting limits and scan intervals.
- **Tab 5: Scheduler** — Select automated schedule frequencies, customize execution times, and sync with systemd user timers or Launchd.

---

## Pro Tips

- **Safe One-Key Cleanup**: Press `l` immediately after scan completes in `devclean` to select only low-risk caches across all 22 ecosystems, then hit `Enter`.
- **Target Heavy Hogs**: Press `m` to select only artifacts above the configured `mass_select_threshold_mb` (default: 1 GB).
- **Fast Jump**: Use `Tab` to leap across huge lists without holding down the arrow keys.

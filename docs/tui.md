# TUI (Terminal User Interface)

DevClean's default mode launches an interactive terminal interface built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and styled with [Lip Gloss](https://github.com/charmbracelet/lipgloss).

---

## Launch

```bash
devclean
```

---

## State Flow

```
scanning ➔ selecting ➔ confirming ➔ deleting ➔ done
```

---

## Screens

### 1. Scanning Screen

```
⠙ Scanning ecosystems...
  • docker: done
  • node: done
  • rust: done
  • ai: scanning
```

A live spinner and per-ecosystem status indicators show progress as all 13 cleaners execute in parallel.

---

### 2. Selecting Screen

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

---

### 3. Confirming Screen

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

---

### 4. Done Screen (Report Card)

```
DevClean Cleanup Summary
──────────────────────────────────────────────────────────────────────────────
Freed: 6.8 GB across 12 targets (staged in trash)
Elapsed: 1.42s
Disk Space: 142.3 GB free ➔ 149.1 GB free (+6.8 GB)
```

Displays the post-cleanup summary with real Statfs disk recovery metrics and exits cleanly.

---

## Complete Keybindings

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

## Pro Tips

- **Safe One-Key Cleanup**: Press `l` immediately after scan completes to select only low-risk caches across all 13 ecosystems, then hit `Enter`.
- **Target Heavy Hogs**: Press `m` to select only artifacts above the configured `mass_select_threshold_mb` (default: 1 GB).
- **Fast Jump**: Use `Tab` to leap across huge lists without holding down the arrow keys.

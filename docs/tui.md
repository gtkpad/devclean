# TUI (Terminal User Interface)

DevClean's default mode launches an interactive terminal interface built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Launch

```bash
devclean
```

## State Flow

```
scanning → selecting → confirming → deleting → done
```

## Screens

### Scanning

```
⠙ Scanning ecosystems...
```

Spinner displayed while all cleaners run in parallel.

### Selecting

```
DevClean                                        Selected: 8 targets (3.2 GB)
──────────────────────────────────────────────────────────────────────────────

── docker (5 targets, 2.3 GB) ─────────────────────────────────────────────
> [x] dangling-image  abc123f       2.1 GB   LOW
  [ ] dangling-image  def456a       200 MB   MED

── node (12 targets, 4.1 GB) ──────────────────────────────────────────────
  [x] node_modules    ~/Work/app1   850 MB   MED
  [x] node_modules    ~/Work/app2   1.2 GB   MED

── go (2 targets, 1.8 GB) ─────────────────────────────────────────────────
  [x] go-mod-cache    ~/go/pkg/mod  1.5 GB   LOW
  [x] go-build-cache  ~/.cache/go   300 MB   LOW

[8/32]

↑↓/jk: navigate | space: toggle | a: all | n: none | l: low-risk | enter: confirm | q: quit
```

Features:

- Targets grouped by ecosystem with section headers
- Per-section totals (target count + size)
- Global selection counter with total size (updates in real time)
- Scroll position indicator
- Risk level displayed per target

### Confirming

```
Confirm deletion?

Targets: 8 selected
Space to free: 3.2 GB

Paths:
  ~/.../image-abc123
  ~/Work/app1/node_modules
  ~/Work/app2/node_modules
  ~/go/pkg/mod
  + 4 more

[y] confirm  [n] cancel
```

### Deleting

```
⠙ Deleting targets...
```

Spinner while deletion is in progress.

### Done

```
✓ Freed: 3.2 GB from 8 targets
```

Exits automatically after displaying the result.

## Keybindings

| Key            | Action                           |
| -------------- | -------------------------------- |
| `j` / `↓`      | Move cursor down                 |
| `k` / `↑`      | Move cursor up                   |
| `Tab`          | Jump to next ecosystem group     |
| `Space`        | Toggle selection on current item |
| `a`            | Select all targets               |
| `n`            | Deselect all targets             |
| `l`            | Select only Low-risk targets     |
| `Enter`        | Proceed to confirmation          |
| `q` / `Ctrl-C` | Quit without changes             |

## Tips

- Use `l` (low-risk) for safe bulk cleanup — selects only targets that are trivially rebuilt (caches)
- `Tab` lets you quickly jump between ecosystems when the list is long
- The size counter updates instantly as you toggle, helping you gauge impact before confirming
- Press `n` then manually select specific items for targeted cleanup

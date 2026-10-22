# Configuration

DevClean uses a YAML configuration file to define scan roots, safety boundaries, per-ecosystem TTL policies, trash retention, and automation settings.

---

## File Location & Precedence

Default configuration path:
```
~/.config/devclean/config.yaml
```

Override with `--config /path/to/custom.yaml` on any command.

### Configuration Precedence

Settings are merged in the following order (highest precedence wins):
1. **CLI flags** (e.g., `--dry-run`, `--format json`, `--force`)
2. **Profile overrides** (`--profile <name>` loading `~/.config/devclean/profiles/<name>.yaml`)
3. **User configuration file** (`~/.config/devclean/config.yaml`)
4. **Built-in defaults**

---

## Full Reference Configuration

```yaml
# Scan roots: directories searched recursively for project artifacts
scan_paths:
  - ~/Code
  - ~/Work
  - ~/Projects

# Excluded paths: specific canonical paths excluded from scanning and deletion
exclude_paths:
  - ~/Work/production-service
  - ~/Projects/active-client-demo

# Protected glob patterns: matched against both directory basename and full path
protected_patterns:
  - "*/proprietary/*"
  - "*/sensitive/*"
  - "*/do-not-delete/*"
  - "*/critical/*"

# Ecosystem policies (all 22 ecosystems)
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

  rust:
    enabled: true
    max_age_days: 30
    fail_mode: continue
    exclude_paths: []

  java:
    enabled: true
    max_age_days: 30
    fail_mode: continue
    exclude_paths: []

  ai:
    enabled: true
    max_age_days: 14
    fail_mode: continue
    exclude_paths: []

  pnpm:
    enabled: true
    max_age_days: 30
    fail_mode: continue
    exclude_paths: []

  bun:
    enabled: true
    max_age_days: 30
    fail_mode: continue
    exclude_paths: []

  nextjs:
    enabled: true
    max_age_days: 7
    fail_mode: continue
    exclude_paths: []

  xcode:
    enabled: true
    max_age_days: 14
    fail_mode: continue
    exclude_paths: []

  homebrew:
    enabled: true
    max_age_days: 30
    fail_mode: continue
    exclude_paths: []

  cpp:
    enabled: true
    max_age_days: 30
    fail_mode: continue
    exclude_paths: []

  flutter:
    enabled: true
    max_age_days: 30
    fail_mode: continue
    exclude_paths: []

  terraform:
    enabled: true
    max_age_days: 30
    fail_mode: continue
    exclude_paths: []

  cocoapods:
    enabled: true
    max_age_days: 14
    fail_mode: continue
    exclude_paths: []

  elixir:
    enabled: true
    max_age_days: 14
    fail_mode: continue
    exclude_paths: []

  # Ephemeral caches and system logs default to 0d for immediate cache eligibility parity
  shell:
    enabled: true
    max_age_days: 0
    fail_mode: continue
    exclude_paths: []

  macos:
    enabled: true
    max_age_days: 0
    fail_mode: continue
    exclude_paths: []

  browsers:
    enabled: true
    max_age_days: 0
    fail_mode: continue
    exclude_paths: []

  leftovers:
    enabled: true
    max_age_days: 0
    fail_mode: continue
    exclude_paths: []

# Safety & Trash configuration
safety:
  trash_enabled: true       # Stage deleted items into ~/.local/share/devclean/trash
  retention_days: 7         # Auto-prune trash manifests older than 7 days
  max_size_mb: 10240        # Maximum trash quota in MB (10 GB)
  trash_dir: ""             # Optional custom trash location (empty = default OS path)

# Terminal User Interface (TUI) options
tui:
  mass_select_threshold_mb: 1024 # Items >= 1 GB selected when pressing 'm' in TUI

# Heuristic scoring engine
intelligence:
  threshold: 0.65           # Default cutoff score for --smart filter
  max_age_days: 90          # Horizon for age score normalization
  ref_size_mb: 500          # Reference target size for size weight normalization
  weights:
    age: 0.35               # Artifact staleness weight
    size: 0.30              # Space impact weight
    git_activity: 0.25      # Git commit/branch recency weight
    risk: 0.10              # Low vs Medium/High risk penalty

# Watch daemon options (devclean watch)
watch:
  interval: 6h              # Scan interval
  threshold_mb: 5120        # Notify when cleanable size exceeds 5 GB

# Historical audit log
history:
  retention_days: 30        # Days to keep cleanup execution records

# Lifecycle execution hooks
hooks:
  node:
    pre_delete: ""
    post_delete: ""
    timeout: 30s
    platform: [darwin, linux]
  docker:
    pre_delete: ""
    post_delete: ""
    timeout: 60s
    platform: [darwin, linux]
```

---

## Profiles Support

DevClean supports contextual configuration profiles stored under `~/.config/devclean/profiles/<name>.yaml`.

### Example: `~/.config/devclean/profiles/ci.yaml`

```yaml
safety:
  trash_enabled: false

ecosystems:
  node:
    max_age_days: 0
  rust:
    max_age_days: 0
  nextjs:
    max_age_days: 0
```

Apply a profile during execution:
```bash
devclean clean --auto --profile ci
```

---

## Managing Exclusions via CLI

Instead of editing `config.yaml` manually, use the `devclean whitelist` command:

- Add path to `exclude_paths`:
  ```bash
  devclean whitelist add ~/Work/client-portal
  ```
- Add glob pattern to `protected_patterns`:
  ```bash
  devclean whitelist add --pattern "*/vendor/*"
  ```
- Remove exclusions:
  ```bash
  devclean whitelist remove ~/Work/client-portal
  devclean whitelist remove --pattern "*/vendor/*"
  ```
- Inspect active exclusions:
  ```bash
  devclean whitelist list
  ```

> [!NOTE]
> `devclean whitelist` uses YAML AST parsing to modify `config.yaml` while preserving existing comments, indentation, and structure.

---

## Cleanup Presets

DevClean includes three pre-defined operational presets accessible via `--preset <name>` on `devclean clean`:

| Preset | Target Scope | TTL Behavior | Use Case |
| :--- | :--- | :--- | :--- |
| **`safe`** | Global caches and logs only (`RiskLow`). Strictly skips project-level builds and dependencies (`node_modules`, `target/`, `build/`, `.next/cache`, `vcpkg_installed`). | Standard TTLs applied. | Zero-risk developer maintenance without triggering any local project rebuilds. |
| **`standard`** | All enabled ecosystems and configured target directories. | Uses per-ecosystem `max_age_days` from `config.yaml`. | Default routine developer cleanup. |
| **`aggressive`** | All enabled ecosystems and configured target directories. | Authoritatively zeroes `max_age_days` (`0d`) across all 22 ecosystems. | Maximum immediate disk reclamation before large migrations or low-disk emergencies. |

Example:
```bash
# Run safe cleanup preview
devclean clean --auto --preset safe --dry-run

# Run aggressive cleanup staging into trash
devclean clean --auto --preset aggressive
```

---

## Interactive TUI Configuration (`devclean config`)

To configure DevClean without editing YAML directly, launch the interactive configuration editor:

```bash
devclean config
```

The full-screen Bubble Tea manager offers 5 configuration tabs:
1. **Ecosystems**: Toggle individual ecosystem cleaners on/off and adjust `max_age_days` TTL thresholds.
2. **Scan Paths**: Add, edit, or remove project scan roots (`scan_paths`).
3. **Safety & Trash**: Configure trash staging, retention days, maximum quota (`max_size_mb`), and view the immutable blacklist.
4. **Watcher**: Adjust disk threshold limits (`threshold_mb`) and monitoring intervals (`interval`).
5. **Scheduler**: Configure automated scheduling frequency, specific times, and synchronize with Systemd User Timers (Linux) or Launchd (macOS).

> [!TIP]
> The configuration editor uses `internal/config/ast.go` to parse and mutate the YAML AST directly, preserving all hand-crafted comments and document formatting.

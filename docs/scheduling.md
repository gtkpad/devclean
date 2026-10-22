# Scheduling & Automation

DevClean can register recurring automated cleanup jobs or background threshold monitoring using native operating system schedulers.

---

## Commands

### `devclean schedule`

Register a recurring cleanup schedule or threshold watch daemon.

```bash
# Register daily cleanup at noon
devclean schedule --daily

# Register weekly cleanup (Sunday at midnight)
devclean schedule --weekly

# Register hourly cleanup
devclean schedule --hourly

# Register cleanup at a specific time of day (e.g. 03:30 AM)
devclean schedule --at 03:30

# Custom cron expression
devclean schedule --cron "0 3 * * 1"   # Every Monday at 3 AM

# Register background watch daemon instead of cleanup job
devclean schedule --watch

# Check current scheduler status and last execution details
devclean schedule --status
```

| Flag | Description |
| :--- | :--- |
| `--daily` | Register cleanup at noon (cron: `0 12 * * *`) |
| `--weekly` | Register cleanup weekly on Sunday at midnight (cron: `0 0 * * 0`) |
| `--hourly` | Register cleanup at the top of every hour (cron: `0 * * * *`) |
| `--at <HH:MM>` | Register cleanup at a specified 24-hour time of day |
| `--cron <expr>` | Custom 5-field cron expression |
| `--watch` | Register continuous threshold watch daemon via systemd/launchd |
| `--status` | Display current schedule, timer status, and last run stats |

---

### `devclean schedule run`

Immediately execute the scheduled cleanup command outside the normal timer interval:

```bash
devclean schedule run
```

Runs unattended `devclean clean --auto` with configured TTLs, records output to execution logs, and updates `lastrun.json`.

---

### `devclean schedule logs`

Inspect, tail, or reset scheduled execution logs (`~/.config/devclean/schedule.log`):

```bash
# View last 50 log lines (default)
devclean schedule logs

# Tail last 100 log lines
devclean schedule logs --lines 100

# Stream logs in real time
devclean schedule logs --follow

# Output structured log entries as JSON
devclean schedule logs --json

# Clear and truncate log file
devclean schedule logs --clear --yes
```

| Flag | Description |
| :--- | :--- |
| `--lines <n>` | Number of log lines to display (default: `50`) |
| `-f, --follow` | Follow log output in real time |
| `--json` | Format log entries as structured JSON objects |
| `--clear` | Truncate the schedule log file (requires `--yes` if non-interactive) |
| `-y, --yes` | Confirm log clearing without interactive confirmation prompt |

> [!NOTE]
> Scheduled execution logs are automatically rotated with a circular 10 MB limit (`internal/scheduler/rotator.go`) to prevent uncontrolled log file growth.

---

### `devclean unschedule`

Remove registered cleanup schedules or watch daemons:

```bash
# Remove recurring cleanup schedule
devclean unschedule

# Remove registered watch daemon
devclean unschedule --watch
```

Exits 0 even if no schedule was found (idempotent).

---

## Platform Behavior

### Linux (Systemd User Timers & Crontab)

1. **Systemd User Timers (Primary)**:
   - When systemd is available, DevClean installs user-level units under `~/.config/systemd/user/`:
     - Service unit: `devclean.service`
     - Timer unit: `devclean.timer`
   - Managed automatically via `systemctl --user daemon-reload`, `enable`, and `start`.
2. **Crontab Fallback**:
   - On Linux systems without systemd user sessions, DevClean falls back to user `crontab`.
   - Entry is tagged with `# devclean auto` or `# devclean watch`.

### macOS (Launchd)

- Generates user LaunchAgent property list files in `~/Library/LaunchAgents/`:
  - Cleanup schedule: `com.devclean.auto.plist`
  - Watch daemon: `com.devclean.watch.plist`
- Registered and unloaded cleanly using `launchctl load` and `launchctl unload`.

---

## Safety: Dry-Run Validation

Before registering any schedule, DevClean performs a dry-run validation:

```bash
devclean clean --auto --dry-run
```

This verifies that the configuration file is valid and won't fail silently when running unattended. If dry-run encounters configuration or permission errors, schedule registration is aborted.

---

## Conflict Detection & Arguments Persistence

- If a schedule already exists, DevClean prompts for confirmation before overwriting.
- Configured schedule arguments (e.g., `--at 03:00`, `--preset safe`) are persisted in `config.yaml` under `scheduler.args` using AST comment-preserving serialization.

## Execution Mode

Scheduled runs use `devclean clean --auto`, which:

- Applies TTL filtering (only deletes targets older than `max_age_days`)
- Respects all safety mechanisms (blacklist, symlinks, mounts)
- Logs results for later review

## Last Run History

Each execution is recorded in:

```
~/.config/devclean/lastrun.json
```

The `--status` flag reads this file to display:

- Last execution timestamp
- Bytes freed in last run
- Next scheduled execution time

## Cron Expression Format

DevClean uses standard 5-field cron expressions:

```
┌───────────── minute (0-59)
│ ┌───────────── hour (0-23)
│ │ ┌───────────── day of month (1-31)
│ │ │ ┌───────────── month (1-12)
│ │ │ │ ┌───────────── day of week (0-6, Sun=0)
│ │ │ │ │
* * * * *
```

Examples:

- `0 12 * * *` — Daily at noon
- `0 3 * * 1` — Every Monday at 3 AM
- `0 0 1 * *` — First of each month at midnight
- `0 */6 * * *` — Every 6 hours

Invalid expressions are rejected with an error message before any system changes are made.

# Scheduling

DevClean can register recurring automated cleanup using the OS-native scheduler.

## Commands

### `devclean schedule`

Register a recurring cleanup job.

```bash
# Daily at noon
devclean schedule --daily

# Custom cron expression
devclean schedule --cron "0 3 * * 1"   # Every Monday at 3 AM

# Check current status
devclean schedule --status
```

| Flag       | Description                                   |
| ---------- | --------------------------------------------- |
| `--daily`  | Register cleanup at noon (cron: `0 12 * * *`) |
| `--cron`   | Custom 5-field cron expression                |
| `--status` | Display current schedule and last run info    |

At least one flag is required. Without flags, the command shows usage help.

### `devclean unschedule`

Remove the registered cleanup job.

```bash
devclean unschedule
```

Exits 0 even if no schedule was found (idempotent).

## Platform Behavior

### macOS (Launchd)

- Generates a `.plist` file in `~/Library/LaunchAgents/`
- Label: `com.devclean.auto`
- Registered via `launchctl load`
- Removed via `launchctl unload`

### Linux (Crontab)

- Injects an entry into the user's crontab (`crontab -e`)
- Entry is marked with a comment for identification: `# devclean auto`
- Removal strips the marked entry

## Safety: Dry-Run Validation

Before registering a schedule, DevClean performs a dry-run validation:

```
devclean clean --auto --dry-run
```

This ensures the current configuration is valid and won't fail silently when running unattended. If the dry-run fails, the schedule is not registered.

## Conflict Detection

If a schedule already exists, DevClean prompts for confirmation before overwriting. This prevents accidentally changing an established schedule.

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

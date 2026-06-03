# MCP Server (AI Agent Integration)

DevClean exposes its scanning and cleaning capabilities as an [MCP (Model Context Protocol)](https://modelcontextprotocol.io) server, allowing autonomous AI agents to manage disk space programmatically.

## Starting the Server

```bash
devclean mcp
```

The server communicates over **stdio** (JSON-RPC), compatible with any MCP client (Cursor, Claude Code, custom agents).

## Configuration Example

### Claude Code (`~/.claude/claude_desktop_config.json`)

```json
{
  "mcpServers": {
    "devclean": {
      "command": "devclean",
      "args": ["mcp"]
    }
  }
}
```

### Cursor (`.cursor/mcp.json`)

```json
{
  "mcpServers": {
    "devclean": {
      "command": "devclean",
      "args": ["mcp"],
      "transportType": "stdio"
    }
  }
}
```

## Tools

### `scan_environment`

Scans all ecosystems and returns cleanable targets.

**Parameters:** None

**Response:**

```json
[
  {
    "ecosystem": "docker",
    "targets": [
      {
        "id": "img-abc123",
        "name": "dangling image",
        "path": "docker://sha256:abc123",
        "size_bytes": 1073741824,
        "risk": "low",
        "last_accessed": "2026-04-15T10:00:00Z",
        "category": "dangling-image"
      }
    ]
  },
  {
    "ecosystem": "node",
    "targets": [...]
  }
]
```

If an ecosystem scan fails, the entry includes `"error"` instead of `"targets"`.

---

### `propose_deletion`

Validates target IDs against safety rules (blacklist, protected patterns) and returns a deletion preview. Always call this before `execute_approved_deletion`.

**Parameters:**

| Name         | Type   | Required | Description                          |
| ------------ | ------ | -------- | ------------------------------------ |
| `target_ids` | string | Yes      | JSON array of target IDs (from scan) |

**Response:**

```json
[
  { "id": "img-abc123", "path": "docker://sha256:abc123", "size_bytes": 1073741824, "status": "approved" },
  { "id": "env-file", "path": "/home/user/.env", "size_bytes": 512, "status": "blocked", "reason": "path is protected by safety blacklist" },
  { "id": "unknown-1", "status": "not_found", "reason": "target not found in current scan" }
]
```

**Statuses:**
- `approved` — safe to delete
- `blocked` — protected by blacklist or safety rules
- `not_found` — ID doesn't match any current scan target

---

### `execute_approved_deletion`

Executes deletion of previously proposed targets. Re-validates against the blacklist before deleting (defense in depth).

**Parameters:**

| Name         | Type   | Required | Description                               |
| ------------ | ------ | -------- | ----------------------------------------- |
| `target_ids` | string | Yes      | JSON array of approved target IDs to delete |

**Response:**

```json
{
  "deleted_count": 3,
  "freed_bytes": 2147483648
}
```

If partial failures occur:

```json
{
  "deleted_count": 2,
  "freed_bytes": 1073741824,
  "errors": ["docker: daemon not running"]
}
```

## Recommended Agent Workflow

1. Call `scan_environment` to discover targets
2. Filter targets by risk level, size, or age
3. Call `propose_deletion` with selected IDs to validate safety
4. Present approved targets to the user (or auto-approve based on policy)
5. Call `execute_approved_deletion` with approved IDs only

## Safety Guarantees

The MCP server inherits all DevClean safety mechanisms:

- **Immutable blacklist** — `.git`, `.env`, `.ssh`, `secrets.json` are never deletable
- **Protected patterns** — configurable globs from `config.yaml` are enforced
- **Double validation** — both `propose` and `execute` check the blacklist independently
- No target can be deleted without first appearing in a `scan_environment` result

---

## Additional Tools

### `get_stats`

Retrieve cleanup history and statistics with optional filters.

**Parameters:**

| Name              | Type    | Required | Description                                      |
| ----------------- | ------- | -------- | ------------------------------------------------ |
| `since`           | string  | No       | ISO date string to filter entries after          |
| `ecosystem`       | string  | No       | Filter by ecosystem name                         |
| `limit`           | number  | No       | Maximum number of history entries to return      |
| `include_history` | boolean | No       | Include individual history entries in response   |

**Response:**

```json
{
  "total_freed_bytes": 5368709120,
  "total_runs": 12,
  "per_ecosystem": [
    { "name": "node", "freed_bytes": 3221225472, "runs": 8, "targets_cleaned": 15 },
    { "name": "docker", "freed_bytes": 2147483648, "runs": 4, "targets_cleaned": 6 }
  ]
}
```

With `include_history: true`:

```json
{
  "total_freed_bytes": 5368709120,
  "total_runs": 12,
  "per_ecosystem": [...],
  "history": [
    { "timestamp": "2026-05-15T10:00:00Z", "targets": 3, "freed_bytes": 1073741824, "ecosystems": ["node"], "duration": 2000000000, "mode": "interactive" }
  ]
}
```

---

### `get_config`

Retrieve current configuration, optionally merged with a named profile.

**Parameters:**

| Name      | Type   | Required | Description                              |
| --------- | ------ | -------- | ---------------------------------------- |
| `profile` | string | No       | Profile name to merge with root config   |

**Response:** Full configuration object as JSON (scan_paths, ecosystems, safety, hooks, etc.).

---

### `explain_target`

Explain whether a path is a cleanup target, excluded, or protected.

**Parameters:**

| Name   | Type   | Required | Description              |
| ------ | ------ | -------- | ------------------------ |
| `path` | string | Yes      | Absolute path to analyze |

**Response:**

```json
{
  "path": "/home/user/project/node_modules",
  "status": "target",
  "ecosystem": "node",
  "reason": "node_modules/ with sibling manifest",
  "size_bytes": 524288000,
  "age_days": 45
}
```

**Statuses:**
- `target` — path is a recognized cleanup target
- `excluded` — matches an exclude pattern in config
- `protected` — protected by safety blacklist
- `not_target` — not recognized by any ecosystem cleaner

---

### `restore_target`

Restore a target from trash by entry ID.

**Parameters:**

| Name        | Type   | Required | Description                |
| ----------- | ------ | -------- | -------------------------- |
| `target_id` | string | Yes      | Trash entry ID to restore  |

**Response (success):**

```json
{
  "success": true,
  "restored_path": "/home/user/project/node_modules"
}
```

**Response (error):**

```json
{
  "success": false,
  "error_code": "path_conflict",
  "error_detail": "path conflict: file already exists"
}
```

**Error codes:**
- `path_conflict` — file already exists at restore path
- `symlink_escape` — restore path contains symlink outside trash
- `not_found` — ID not in trash
- `expired` — entry already purged

---

### `list_trash`

List all restorable items currently in trash.

**Parameters:** None

**Response:**

```json
[
  {
    "id": "abc-123",
    "batch_id": "batch-456",
    "original_path": "/home/user/project/node_modules",
    "size_bytes": 524288000,
    "ecosystem": "node",
    "moved_at": "2026-05-15T10:00:00Z"
  }
]
```

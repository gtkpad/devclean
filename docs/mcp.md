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

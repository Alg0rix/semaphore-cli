# MCP server mode

`semaphore-cli mcp` serves the Semaphore API as MCP tools over stdio. It
uses the same credential chain as the CLI (flags > `SEMAPHORE_CLI_*` >
saved context), so either pass env in the client config or log in once with
`auth login` first. The stdio transport owns stdout — diagnostics go to
stderr.

## Register with clients

Claude Code:

```bash
claude mcp add semaphore \
  --env SEMAPHORE_CLI_URL=https://semaphore.example.com \
  --env SEMAPHORE_CLI_TOKEN=semp_xxx -- semaphore-cli mcp
# or use a saved context:
claude mcp add semaphore --env SEMAPHORE_CLI_CONTEXT=prod -- semaphore-cli mcp
```

Generic MCP JSON (Claude Desktop, Cursor, Devin settings):

```json
{
  "mcpServers": {
    "semaphore": {
      "command": "semaphore-cli",
      "args": ["mcp"],
      "env": {
        "SEMAPHORE_CLI_URL": "https://semaphore.example.com",
        "SEMAPHORE_CLI_TOKEN": "semp_xxx"
      }
    }
  }
}
```

## Tools

| Tool | Args | Returns |
|---|---|---|
| `info` | — | `{version, ansible, web_host, …}` |
| `whoami` | — | user of the API token |
| `project_list` | — | `[{id, name, created, …}]` |
| `template_list` | `project` | `[{id, name, app, playbook, last_task_id, last_task_status, …}]` |
| `resource_list` | `project`, `resource` | raw JSON for `inventory` `environment` `schedule` `key` `repository` `view` `member` (project-scoped) or `user` `runner` (global; `project` ignored) |
| `task_run` | `project`, `template`, `limit?`, `branch?`, `message?`, `args?`, `dry_run?`, `wait?`, `timeout_ms?` | task object; with `wait` the final one |
| `task_wait` | `project`, `task_id`, `timeout_ms?` | final task object |
| `task_list` | `project` | recent tasks, newest first |
| `task_get` | `project`, `task_id` | one task |
| `task_logs` | `project`, `task_id` | `["line", …]` output lines |
| `task_stop` | `project`, `task_id`, `force?` | confirmation text |
| `task_confirm` | `project`, `task_id` | confirmation text |
| `task_reject` | `project`, `task_id` | confirmation text |

`project` accepts an id or a name everywhere. Read tools carry
`readOnlyHint`/`idempotentHint` annotations.

## Semantics for callers

- Errors arrive as results with `isError: true` and a text message — check
  `isError` before reading `structuredContent`.
- Task outputs are slim objects (`mcpTask`): scalar fields only, RFC3339
  timestamps, no nested task blobs. A failed `task_wait` still returns the
  terminal state in `structuredContent` on success; a timeout is an
  `isError` result saying the task is still running.
- `task_run` + `wait` covers the full run loop in one call; use
  `timeout_ms` to bound it. Follow with `task_logs` for output.

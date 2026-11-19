# MCP server mode

`semaphore-cli mcp` runs a Model Context Protocol server on stdin/stdout. It
exposes the Semaphore REST API as MCP tools, so MCP-compatible agents can
drive Semaphore directly instead of shelling out to the CLI.

The server uses the same credential resolution as the CLI: flags,
`SEMAPHORE_CLI_*` environment variables, then the active saved context.
Pass credentials through the client environment, or log in once with
`semaphore-cli auth login` before you register the server.

Do not write logs to stdout in MCP mode. The protocol owns stdout; the server
keeps diagnostics on stderr.

## Client configuration

### Claude Code

```bash
claude mcp add semaphore --env SEMAPHORE_CLI_URL=https://semaphore.example.com \
    --env SEMAPHORE_CLI_TOKEN=YOUR_TOKEN -- semaphore-cli mcp
```

Or register a saved context instead of a raw token:

```bash
claude mcp add semaphore --env SEMAPHORE_CLI_CONTEXT=main -- semaphore-cli mcp
```

### Claude Desktop / Cursor / generic MCP JSON

```json
{
  "mcpServers": {
    "semaphore": {
      "command": "semaphore-cli",
      "args": ["mcp"],
      "env": {
        "SEMAPHORE_CLI_URL": "https://semaphore.example.com",
        "SEMAPHORE_CLI_TOKEN": "YOUR_TOKEN"
      }
    }
  }
}
```

### Devin

Add an MCP server entry in your Devin settings with the same `command`,
`args`, and `env` values.

## Tools

| Tool | Description | Read-only |
|---|---|---|
| `info` | Semaphore server version and info | yes |
| `whoami` | The user the API token belongs to | yes |
| `project_list` | All projects | yes |
| `template_list` | Templates of a project | yes |
| `resource_list` | `inventory`, `environment`, `schedule`, `key`, `repository`, `view`, `member`, `user`, or `runner` | yes |
| `task_run` | Start a task by template id or name; `wait: true` returns the final task | no |
| `task_wait` | Block until a task finishes; returns its final state | no |
| `task_list` | Recent project tasks, newest first | yes |
| `task_get` | One task by id | yes |
| `task_logs` | Output lines of a task | yes |
| `task_stop` | Stop a running task (`force` to kill) | no |
| `task_confirm` | Approve a task waiting for confirmation | no |
| `task_reject` | Reject a task waiting for confirmation | no |

Most tools take a `project` argument that accepts a project id or name.
`task_*` tools also take `task_id`.

Errors return a result with `isError: true` and a text message, not a
protocol error. Agents should read `isError` before they use
`structuredContent`.

## Example

`task_run` with `wait` covers the whole run loop in one tool call:

```json
{
  "name": "task_run",
  "arguments": {
    "project": "myproject",
    "template": "deploy",
    "wait": true,
    "timeout_ms": 600000
  }
}
```

The result contains the final task object, including `status`, `start`,
`end`, and `commit_hash`. Follow up with `task_logs` for the output lines.

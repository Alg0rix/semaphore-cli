# semaphore-cli

A standalone command-line client for [Semaphore UI](https://semaphoreui.com),
the web interface for Ansible, Terraform, and other automation tools.

`semaphore-cli` talks to a Semaphore server over its REST API. It works from
any machine that can reach the server. It does not need the server config file
or database access, and it does not conflict with the `semaphore` server
binary.

The CLI is designed for humans and AI agents. Commands print structured
output to stdout and diagnostics to stderr. Task results use stable exit
codes, so scripts and agents can react to outcomes directly.

## Features

- Token and password authentication with named server contexts.
- Full project resource coverage: projects, templates, inventories,
  environments, schedules, keys, repositories, views, members, users, runners.
- Task lifecycle: run, wait, logs, stop, confirm, reject, rerun, delete.
- Table, JSON, and YAML output with field selection and quiet mode.
- Agent-ready semantics: the final task object prints to stdout when a task
  ends, and the exit code reports the outcome.
- Built-in MCP server mode for tool-calling agents.
- Embedded agent skill that the CLI installs into standard agent skill
  directories.

## Install

Build from source (Go 1.21 or later):

```bash
git clone https://github.com/Alg0rix/semaphore-cli
cd semaphore-cli
go build -o semaphore-cli .
```

Or install directly:

```bash
go install github.com/Alg0rix/semaphore-cli@latest
```

## Quickstart

```bash
# Log in and save a context.
semaphore-cli auth login --url https://semaphore.example.com --user alice

# Or pipe an API token (create one in Semaphore under User Settings).
echo "$TOKEN" | semaphore-cli auth login --url https://semaphore.example.com --with-token

# Save a default project in the context.
semaphore-cli context set main -p myproject

# Run a template and follow its output.
semaphore-cli template run deploy --follow
```

## Authentication

The CLI resolves credentials in this order:

1. Flags: `--url`, `--token`, `--context`, `--project`.
2. Environment: `SEMAPHORE_CLI_URL`, `SEMAPHORE_CLI_TOKEN`,
   `SEMAPHORE_CLI_CONTEXT`, `SEMAPHORE_CLI_PROJECT`,
   `SEMAPHORE_CLI_OUTPUT`, `SEMAPHORE_CLI_INSECURE`.
3. The active saved context in `~/.semaphore-cli/config.yaml` (mode `0600`).

`auth login --user/--password` creates a session, mints an API token, and
stores it in the context. `--with-token` stores a token you provide on stdin.
Use `auth status`, `auth token`, and `auth logout` to inspect or clear the
session. Manage multiple servers with `context list|use|set|delete`.

For CI, set `SEMAPHORE_CLI_URL` and `SEMAPHORE_CLI_TOKEN` in the environment.

## Commands

| Command | Description |
|---|---|
| `auth login\|status\|token\|logout` | Manage authentication |
| `context list\|use\|set\|delete` | Manage named server contexts |
| `info` | Show server version and info |
| `project list\|get` | List and inspect projects |
| `template list\|get\|run` | List, inspect, and run templates |
| `task list\|get\|run\|logs\|wait\|stop\|confirm\|reject\|rerun\|delete` | Manage tasks |
| `inventory\|environment\|schedule\|key\|repository\|view\|member\|user\|runner list\|get` | List and inspect resources |
| `mcp` | Serve the API as MCP tools over stdio |
| `skill install` | Install the embedded agent skill |

Most `get` and `run` commands accept an id or a name. Project-scoped commands
take `-p/--project` (id or name) or `SEMAPHORE_CLI_PROJECT`.

### Task run flags

`task run` and `template run` share the same flags:

```bash
semaphore-cli task run <id|name> -p <project> \
    [--follow] [--wait] [--timeout 5m] \
    [--limit HOSTS] [--branch BRANCH] [--message TEXT] [--args '["-x"]'] \
    [--dry-run] [--diff] [--debug-task]
```

- `task run` sends the name to the server, which resolves it. Use it when the
  template name may contain spaces.
- `template run` resolves the name locally first, so a typo reports
  `not found` before the API call.

## Output

`-o/--output` selects `table` (default), `json`, or `yaml`. Structured output
always goes to stdout; status messages and prompts go to stderr.

```bash
semaphore-cli task list -o json
semaphore-cli project list --fields id,name --no-headers
semaphore-cli template list -q          # ids only, one per line
```

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | General error |
| 2 | Usage error |
| 3 | Task failed |
| 4 | Authentication failed |
| 5 | Not found |
| 6 | Task stopped |
| 124 | Wait timeout |

## Use with agents and scripts

The task commands return machine-readable results, so an agent can complete a
full run loop in one call:

```bash
# Run and wait. Prints the final task object and exits with its outcome.
semaphore-cli task run deploy -p myproject --wait -o json
```

```bash
# Split the run: capture the id, wait later, then read logs.
TID=$(semaphore-cli task run deploy -p myproject -q)
semaphore-cli task wait "$TID" -p myproject -o json
semaphore-cli task logs "$TID" -p myproject -o json   # timestamped lines
```

A finished task prints its final object to stdout even when it failed.
Check the exit code first, then parse stdout.

## MCP server mode

`semaphore-cli mcp` starts an MCP server on stdio. It exposes the API as
tools (`info`, `whoami`, `project_list`, `template_list`, `resource_list`,
`task_run`, `task_wait`, `task_list`, `task_get`, `task_logs`, `task_stop`,
`task_confirm`, `task_reject`). See [docs/mcp.md](docs/mcp.md) for client
configuration.

## Agent skill

The binary embeds an agent skill that teaches AI coding agents how to drive
the CLI. Install it into the standard skill directories:

```bash
semaphore-cli skill install                 # ~/.agents, ~/.claude, ~/.config/devin
semaphore-cli skill install --path .agents/skills   # project-local
```

## Development

```bash
go build -o semaphore-cli .
go vet ./...
```

The layout separates the API client (`internal/apiclient`), configuration
(`internal/config`), and output formatting (`internal/output`) from the
Cobra commands (`cmd/`).

## License

[MIT](LICENSE)

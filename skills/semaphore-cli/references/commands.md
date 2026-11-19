# Command reference

Global flags on every command: `--url`, `--token`, `--context`,
`-p/--project` (id or name), `-o/--output` (`table` default, `json`,
`yaml`), `--fields a,b`, `-q` (ids only), `--no-headers`, `-y/--yes`,
`--insecure`, `--ca-cert FILE`, `--timeout DUR` (HTTP timeout), `--debug`
(HTTP trace to stderr).

## Commands

| Command | What it does |
|---|---|
| `auth login` | `--user/--password` or `--with-token` (stdin), `--name` for context, `--token-name` |
| `auth status` | URL, context, user, token expiry; `-o json` |
| `auth token` | Print the stored API token |
| `auth logout` | Revoke the token on the server, clear the context |
| `context list\|use NAME\|set NAME\|delete NAME` | Named server profiles |
| `info` | Server version, web host, features |
| `project list`, `project get <id\|name>` | Projects |
| `template list`, `template get <id\|name>` | Templates (`-p` required) |
| `template run <id\|name>` | Resolve template locally, then start a task |
| `task list` | Recent tasks, newest first (`-p`) |
| `task get <id>` | One task (`-p`) |
| `task run <id\|name>` | Start a task; name resolved by the server (`-p`) |
| `task logs <id>` | Task output; `-f` streams until done (`-p`) |
| `task wait <id>` | Block until terminal state; prints final task (`-p`) |
| `task stop <id>` | Request stop; `--force` kills the process (`-p`) |
| `task confirm <id>` | Approve a task paused for confirmation (`-p`) |
| `task reject <id>` | Reject a task paused for confirmation (`-p`) |
| `task rerun <id>` | Re-run a finished task; `-f` follows (`-p`) |
| `task delete <id>` | Delete task + output; prompts unless `-y` (`-p`) |
| `inventory\|environment\|schedule\|key\|repository\|view\|member list\|get` | Project resources (`-p`) |
| `user\|runner list\|get` | Global resources (no `-p`) |
| `mcp` | MCP server over stdio — see mcp.md |
| `skill install [--path DIR]` | Install this skill into agent skill dirs |

`get`/`run`/`logs`/`wait` accept numeric ids; `project get`,
`template get|run`, `task run` and the resource `get`s also accept names.

## Run flags (`task run`, `template run`)

```
--follow, -f     stream task output until it finishes
--wait, -w       wait silently; print final task; exit code = outcome
--timeout DUR    bound for --follow/--wait (default: no limit)
--limit HOSTS    ansible --limit
--branch BRANCH  override git branch
--message TEXT   task message
--args 'JSON'    extra CLI arguments as a JSON array string
--dry-run        ansible --check
--diff           ansible --diff
--debug-task     verbose ansible output
```

## Output contract

- stdout carries the result: table rows, or the object/array in `-o
  json|yaml`. `-q` emits one id per line. `--fields` narrows table columns.
- stderr carries progress (`task 14 started`), prompts, and errors.
- Exit codes: `0` ok · `1` error · `2` usage · `3` task failed · `4` auth ·
  `5` not found · `6` task stopped · `124` wait timeout · `130` interrupted.
- `run --wait`/`--follow` and `task wait` print the final task object even
  when the outcome is a failure exit code. Check the code first, then parse.

## Error shapes

API errors print as `semaphore-cli: <message>` on stderr with the mapped
exit code. Not-found name lookups (`project "x" not found`,
`template "x" not found`) are exit 5. Missing `-p` when a command needs a
project is exit 2.

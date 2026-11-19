---
name: semaphore-cli
description: Use the semaphore-cli binary to run and monitor Semaphore UI (Ansible Semaphore) tasks over its REST API. Use for triggering template runs, watching task status and logs, and listing projects, templates, inventories, environments, schedules, keys, repositories, views, members, users and runners with JSON output.
---

# semaphore-cli

Use `semaphore-cli` for Semaphore UI API work. Build it with
`go build -buildvcs=false -o semaphore-cli .` if the binary is missing.
Do not confuse it with `semaphore`, the server admin binary.

## Authentication

Resolution order: flag > `SEMAPHORE_CLI_*` env > saved context.

- Saved login: `semaphore-cli auth login --url https://host --user alice`
  (prompts for password, mints an API token) or pipe a token with
  `--with-token`. Contexts live in `~/.semaphore-cli/config.yaml`.
- Agent/CI: set `SEMAPHORE_CLI_URL` and `SEMAPHORE_CLI_TOKEN` (User
  Settings > API Tokens), or pass `--url`/`--token` per call.
- `semaphore-cli context set NAME -p PROJECT` saves a default project;
  `-p` accepts an id or a name. `context use NAME` switches.

## Agent workflow

Every command accepts `-o json|yaml` (stdout is always clean machine
readable output; messages go to stderr), `--fields a,b`, `-q` (ids only).

1. `semaphore-cli info` / `auth status` to check connectivity and identity.
2. `project list`, `template list -p <project>` to discover targets.
3. Trigger a run and wait for the result in one call:

   ```bash
   semaphore-cli task run <template id|name> -p <project> --wait -o json
   ```

   `task run` resolves names server-side; `template run` resolves them
   client-side first and gives a cleaner not-found error. Flags:
   `--follow`, `--limit`, `--branch`, `--message`, `--args`, `--dry-run`,
   `--diff`, `--debug-task`, `--timeout`.

4. Or split it: `TID=$(semaphore-cli task run T -p P -q)` then
   `semaphore-cli task wait $TID -p P -o json` — the final Task object
   (status, start/end, commit_hash) prints when the task ends.
5. `task logs $TID -p P` prints plain lines; `-o json` returns the
   timestamped output array. `logs -f` streams.
6. `task stop`, `task confirm`, `task reject`, `task rerun`,
   `task delete -y` manage lifecycle.

## Exit codes (check before parsing)

0 ok, 1 error, 2 usage, 3 task failed, 4 auth failed, 5 not found,
6 task stopped, 124 wait timeout. A failed wait still prints the final
task JSON to stdout — parse it before reporting.

## MCP mode

`semaphore-cli mcp` serves the same operations as MCP tools over stdio
(info, whoami, project_list, template_list, task_run, task_wait, task_list,
task_get, task_logs, task_stop, task_confirm, task_reject, resource_list).
Register it with an MCP client, passing `SEMAPHORE_CLI_URL` and
`SEMAPHORE_CLI_TOKEN` in the environment.

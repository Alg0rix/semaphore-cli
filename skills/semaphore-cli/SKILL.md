---
name: semaphore-cli
description: Use the semaphore-cli binary to run, watch, and inspect Semaphore UI (Ansible Semaphore) tasks over its REST API. Use for triggering template runs, waiting on task results, reading task logs, and listing projects, templates, inventories, environments, schedules, keys, repositories, views, members, users and runners — including through its built-in MCP server.
---

# semaphore-cli

Use `semaphore-cli` for Semaphore UI API work. It is a REST client; it does
not need the server config or database. Do not confuse it with the
`semaphore` server binary. Build it with `go build -o semaphore-cli .` from
the repo, or install a release binary if it is missing.

Three rules govern every call:

- Structured results go to stdout, diagnostics and prompts to stderr.
  `-o json|yaml` is always clean to parse; `-q` prints ids only.
- The exit code reports the outcome: 0 ok, 3 task failed, 4 auth, 5 not
  found, 6 stopped, 124 wait timeout. Check it before parsing.
- A finished task prints its final JSON object to stdout even on failure —
  check the code first, then parse stdout for the details.

## Load the relevant reference

| Task | Reference |
| --- | --- |
| Login, contexts, env vars, CI credentials, TLS | [auth](references/auth.md) |
| Full command list, flags, output formats, exit codes | [commands](references/commands.md) |
| Triggering runs, waiting, logs, agent run loops, recipes | [tasks](references/tasks.md) |
| MCP server mode, client config, tool reference | [mcp](references/mcp.md) |

## Core workflow

1. Authenticate once: `semaphore-cli auth login --url URL --user NAME`, or
   set `SEMAPHORE_CLI_URL`/`SEMAPHORE_CLI_TOKEN`. Check with `info` and
   `auth status`.
2. Pick the project: `-p` accepts an id or a name. Save it in the context
   with `context set NAME -p PROJECT` to stop passing `-p`.
3. Trigger and collect the result in one call:

   ```bash
   semaphore-cli task run TEMPLATE -p PROJECT --wait -o json
   ```

4. Or split it: `TID=$(semaphore-cli task run T -p P -q)`, then
   `task wait $TID -p P -o json`, then `task logs $TID -p P -o json`.

## Gotchas

- `task run` resolves names server-side; `template run` resolves them
  client-side first and reports a cleaner `not found` for typos.
- `task logs` plain output is line-oriented; `-o json` returns the
  timestamped `TaskOutput` array.
- Mutations that prompt (`task delete`) need `-y` in non-interactive runs.
- `task confirm`/`reject` only act on tasks paused for confirmation; on a
  finished task the server returns 400, which is expected.

# Running tasks and reading results

The task API is the agent's main surface. Every pattern below respects the
contract: the task object lands on stdout, the exit code carries the
outcome, progress stays on stderr.

## One-shot: run and collect

```bash
semaphore-cli task run deploy -p myproject --wait -o json
```

Blocks until the task ends, prints the final `Task` object
(`id`, `status`, `start`, `end`, `commit_hash`, `version`, `message`,
`limit`, `git_branch`, `user_name`, `created`), then exits:

| Exit | Status |
|---|---|
| 0 | `success` |
| 3 | `error` |
| 6 | `stopped` |
| 124 | still running at `--timeout` |

`--follow` does the same but also streams log lines to stderr live.
`--wait` is silent — prefer it when only the result matters.

## Split: capture id, decide later

```bash
TID=$(semaphore-cli task run deploy -p myproject -q)   # stdout = "42"
semaphore-cli task wait "$TID" -p myproject -o json
semaphore-cli task logs "$TID" -p myproject -o json
```

`task wait` prints the same final object and uses the same exit codes.
`task logs` prints plain lines by default; `-o json` returns
`[{"task_id":..,"time":..,"output":..}]`. `task logs -f` streams until the
task finishes.

## Task object fields

```json
{
  "id": 14, "template_id": 1, "status": "success",
  "message": "", "limit": "", "git_branch": "",
  "user_name": "Admin", "created": "...",
  "start": "...", "end": "...",
  "commit_hash": "1ec1c82b…", "version": "",
  "debug": false, "dry_run": false
}
```

`status` values seen in practice: `waiting`, `running`, `success`,
`error`, `stopped`, plus `waiting_confirmation` for templates that require
manual approval (drive those with `task confirm` / `task reject`).

## Recipes

Deploy and report:

```bash
semaphore-cli task run deploy -p prod --wait -o json > /tmp/task.json
code=$?
if [ $code -eq 0 ]; then jq -r '.commit_hash' /tmp/task.json
else jq -r '.status + " " + .message' /tmp/task.json; fi
```

Long job with a budget:

```bash
semaphore-cli task run backup -p prod --wait --timeout 30m -o json
# exit 124 → still running; poll again with task wait
```

Kill a stuck run:

```bash
semaphore-cli task list -p prod -o json | jq '.[] | select(.status=="running")'
semaphore-cli task stop 99 -p prod            # graceful
semaphore-cli task stop 99 -p prod --force    # kill immediately
semaphore-cli task wait 99 -p prod -o json    # exits 6 when stopped
```

Dry-check before a real run:

```bash
semaphore-cli task run deploy -p prod --dry-run --diff --wait -o json
```

Re-run the same template with the same parameters:

```bash
semaphore-cli task rerun 14 -p prod -f
```

## Which run command

- `task run <id|name>` — sends `template_name` to the server; works with
  names containing spaces; a bad name surfaces as a server 404 (exit 5).
- `template run <id|name>` — lists templates and resolves the name first;
  a typo reports `template "x" not found` (exit 5) before any API call.
- Same flags, same endpoint underneath. Pick `task run` for scripts,
  `template run` for interactive typo-checking.

## Failure reporting

When a wait exits non-zero, stdout still holds the final task object —
parse it for `status`, `end`, and `version`/`commit_hash`, then call
`task logs -o json` for the failure lines. Report the exit code to the
caller; it distinguishes failure (`3`), operator stop (`6`), and your own
timeout (`124`).

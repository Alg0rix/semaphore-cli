# Authentication and contexts

`semaphore-cli` authenticates to the Semaphore REST API with a bearer API
token. It never needs the server config file or database access.

## Credential resolution order

1. Flags: `--url`, `--token`, `--context`, `--project`, `--insecure`,
   `--ca-cert`.
2. Environment: `SEMAPHORE_CLI_URL`, `SEMAPHORE_CLI_TOKEN`,
   `SEMAPHORE_CLI_CONTEXT`, `SEMAPHORE_CLI_PROJECT`,
   `SEMAPHORE_CLI_OUTPUT`, `SEMAPHORE_CLI_INSECURE`.
3. The active saved context in `~/.semaphore-cli/config.yaml` (mode `0600`).

A flag always wins over the environment, and the environment always wins
over the saved context.

## Log in

Password login (creates a session, then mints a named API token):

```bash
semaphore-cli auth login --url https://semaphore.example.com --user alice
# prompts for the password; or pass --password (visible in shell history)
semaphore-cli auth login --url URL --user alice --name prod   # save as context "prod"
```

Token login (create the token in Semaphore under User Settings > API Tokens):

```bash
echo "$TOKEN" | semaphore-cli auth login --url URL --with-token
echo "$TOKEN" | semaphore-cli auth login --url URL --with-token --name prod
```

## Contexts

```bash
semaphore-cli context list                    # all saved contexts, active marked
semaphore-cli context use prod                # switch the active context
semaphore-cli context set prod --url URL      # create or update fields
semaphore-cli context set prod -p myproject   # save a default project
semaphore-cli context set prod --insecure     # save TLS-skip for self-signed
semaphore-cli context delete prod
```

`context set` only changes the flags you pass; other fields keep their
values. A saved `-p` accepts a project id or name, so every project-scoped
command can drop the flag.

## Check the session

```bash
semaphore-cli auth status           # url, context, user, token expiry
semaphore-cli auth status -o json   # machine-readable
semaphore-cli info                  # server version; proves connectivity
```

`auth status` exits 4 when the token is missing or rejected.

## CI / agent credentials

Prefer environment variables so no config file or login is needed:

```bash
export SEMAPHORE_CLI_URL=https://semaphore.example.com
export SEMAPHORE_CLI_TOKEN=semp_xxx
export SEMAPHORE_CLI_PROJECT=myproject   # optional default
```

Treat the token as a secret. `auth token` prints the stored token — use it
only when wiring credentials, not for inspection. `auth logout` revokes the
API token on the server and removes it from the context.

## TLS

- `--insecure` skips certificate verification (self-signed dev servers).
- `--ca-cert FILE` trusts a specific CA instead — prefer it over
  `--insecure` for anything shared.
- Both can be saved per-context with `context set NAME --insecure` or
  `--ca-cert`.

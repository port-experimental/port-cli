# Live Smoke Checks

Read-only / dry-run checks against real Port organizations. This is **manual and opt-in** — `make check` / CI unit tests remain the merge gate.

## What it runs

`make live-smoke` builds nothing itself beyond expecting `./bin/port` (the Make target runs `build` first) and then executes `scripts/live-smoke.sh`:

1. `api blueprints list` for base and target orgs
2. `compare` (JSON)
3. `export --skip-entities` (schema-only archive)
4. `migrate --skip-entities --dry-run` (no writes)

The script asserts JSON shape, `export`/`migrate` `success=true`, and a non-empty export archive. It never prints credential values.

## Required credentials

| Variable | Purpose |
| --- | --- |
| `PORT_CLIENT_ID` | Base org machine credentials |
| `PORT_CLIENT_SECRET` | Base org machine credentials |
| `PORT_TARGET_CLIENT_ID` | Target org machine credentials |
| `PORT_TARGET_CLIENT_SECRET` | Target org machine credentials |

Optional:

| Variable | Default |
| --- | --- |
| `PORT_API_URL` | `https://api.getport.io/v1` |
| `PORT_TARGET_API_URL` | same as `PORT_API_URL` |
| `PORT_BASE_ORG` | `base` |
| `PORT_TARGET_ORG` | `target` |
| `PORT_BIN` | `./bin/port` |

Missing credentials fail immediately with a clear message and usage text (no secret values are echoed).

## Local usage

```bash
export PORT_CLIENT_ID="..."
export PORT_CLIENT_SECRET="..."
export PORT_TARGET_CLIENT_ID="..."
export PORT_TARGET_CLIENT_SECRET="..."

make live-smoke
```

Credentials are written only to a temporary `chmod 600` config under `$TMPDIR`, and are unset from the child process environment before each CLI invocation.

## Identical vs different orgs

- When base and target mirror each other, compare may report `identical: true` — that is expected.
- Against divergent orgs, smoke still passes as long as list/compare/export/migrate dry-run succeed.

## Opt-in GitHub Actions

Workflow: `.github/workflows/live-smoke.yml`

- Trigger: **workflow_dispatch only** (never on push/PR by default)
- Requires repository secrets matching the env vars above
- Does not print secret values; fails clearly if secrets are unset

Unit tests in `.github/workflows/ci.yml` remain the required check.

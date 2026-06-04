# Local skills E2E smoke test

Manual end-to-end validation for `port skills` against a **live** Port stack. This script is **not** run in CI or `make test`.

## Prerequisites

1. Port stack running locally (`port-api`, `ai-service`, `admin-service`).
2. Authenticated Port CLI the same way you normally use it (`port auth login` and/or org credentials in `~/.port/config.yaml`).
3. Demo catalog seeded from the Port monorepo:

   ```sh
   cd /path/to/port
   yarn seed:demo-skills
   ```

   See `scripts/demo-skills/README.md` in the Port repo for group layout and active versions (`demo-onboarding` **2.0.0**, `demo-api-guide` **1.1.0**, etc.).

4. `ORG` defaults to `demo` (override if needed).

5. OAuth token in `~/.port/creds.json` for that org (the script symlinks it next to the temp config; same as normal CLI auth).

## Run

`make e2e-skills-local` runs `make build` then executes tests with **`./bin/port`**.

```sh
make e2e-skills-local
```

Or directly (runs `make build` unless `SKIP_BUILD=1`):

```sh
./scripts/e2e-skills-local.sh
```

| Variable | Default |
| -------- | ------- |
| `PORT_BIN` | `<repo>/bin/port` |
| `SKIP_BUILD` | `0` |
| `PORT_API_URL` | `http://localhost:3000/v1` |
| `PORT_AI_SERVICE_URL` | `http://localhost:3016/v1` |
| `ORG` | `demo` |

## Layout

| Path | Purpose |
| ---- | ------- |
| `scripts/e2e-skills-local.sh` | Orchestrator |
| `scripts/e2e-skills/lib/` | Shared helpers (config, disk assertions, seed constants) |
| `scripts/e2e-skills/scenarios/` | One file per scenario |
| `scripts/e2e-skills/fixtures/` | Local create/edit fixtures |

## Isolation

The script passes `--config` with a copy of your `~/.port/config.yaml` and symlinks `~/.port/creds.json` beside it (Port resolves creds relative to the config path). Only `skills.targets` is pointed at `$TMPDIR/port-cli-e2e-*/workdir/.cursor`.

## Scenarios

1. **Preflight** — API health, `port skills list`
2. **List & search** — demo skill ids and active version in list JSON
3. **Sync versions** — full group sync; disk content matches **active** semver only (not legacy trees)
4. **Selection** — explicit `--group` / `--select-all-ungrouped`; `team_group_defaults` + `include_groups` / `exclude_groups` and disk reconciliation
5. **CRUD** — create, batch, duplicate guard, edit
6. **Archive removed** — `archive` absent from `port skills --tree`

## Troubleshooting

- **Auth failures:** run `port auth login` (or set org credentials in `~/.port/config.yaml`) and retry `port skills list` manually.
- **Missing demo skills:** `yarn seed:demo-skills` in the Port repo; restart `ai-service` after catalog package changes.
- **False passes on disk checks:** skills are matched by `name:` in `SKILL.md` (directories use display titles, not identifiers).

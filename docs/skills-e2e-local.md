# Local skills E2E smoke test

Manual end-to-end validation for `port skills` against a **live** Port stack. This script is **not** run in CI or `make test`.

## Prerequisites

1. Port stack running locally (`port-api`, `ai-service`, `admin-service`).
2. Organization with machine app credentials (default `org_BneDtWovPqXaA2VZ`, override with `ORG`).
3. Demo catalog seeded from the Port monorepo after blueprint changes:

   ```sh
   cd /path/to/port
   yarn seed:demo-skills
   ```

4. Port CLI auth (same as normal `port skills` usage). The script sources **`~/.port/.env`**, merges **`~/.port/config.yaml`** (needs PyYAML) into a temp config, and symlinks **`~/.port/creds.json`** next to that config for OAuth. Machine credentials are passed as root `--client-id` / `--client-secret`; org selection uses `ORG` / `PORT_DEFAULT_ORG` via `port skills --org` (not a root flag). The `port` binary also loads `~/.port/.env` on its own via godotenv.

   ```sh
   # ~/.port/.env example
   PORT_CLIENT_ID="..."
   PORT_CLIENT_SECRET="..."
   PORT_API_URL="http://localhost:3000/v1"
   PORT_AI_SERVICE_URL="http://localhost:3016/v1"
   ```

   If you only use `~/.port/config.yaml` for org credentials, install PyYAML (`pip install pyyaml`) so the script can merge that file with isolated skills paths.

## Run

`make e2e-skills-local` runs `make build` then executes tests with **`./bin/port`** from this repo (not a global install).

```sh
make e2e-skills-local
```

Or run the script directly (it runs `make build` unless `SKIP_BUILD=1`):

```sh
./scripts/e2e-skills-local.sh
```

Optional overrides:

| Variable | Default |
| -------- | ------- |
| `PORT_BIN` | `<repo>/bin/port` (local `make build` output) |
| `SKIP_BUILD` | `0` — set to `1` to skip `make build` inside the script |
| `PORT_API_URL` | `http://localhost:3000/v1` |
| `PORT_AI_SERVICE_URL` | `http://localhost:3016/v1` |
| `ORG` | `org_BneDtWovPqXaA2VZ` |

## Isolation

The script uses `--config` with a temp file under `$TMPDIR/port-cli-e2e-*` and sets `CURSOR_CONFIG_DIR` to a temp `.cursor` directory so your real `~/.port/config.yaml` and `~/.cursor/skills` are not modified.

## Scenarios covered

- API preflight and authenticated `port skills list`
- Demo catalog list/search/sync (including grouped skills with `select_all_groups`)
- Single and batch `port skills create`
- Duplicate create (409 / non-zero exit)
- `port skills edit --publish`
- Archive command removed

## Troubleshooting

- **Auth failures:** ensure `~/.port/.env` uses `PORT_CLIENT_ID` / `PORT_CLIENT_SECRET` (not unprefixed names). Set `ORG` if your default org is not `org_BneDtWovPqXaA2VZ`. Install PyYAML to merge org blocks from `~/.port/config.yaml`. Preflight prints the CLI error line on failure.
- **`ai-service returned 500` on list/search/sync (create works):** restart **ai-service** after pulling blueprint/catalog changes so it loads the current `@port-labs/customer-skills-catalog` (old in-memory code may still scan `release_state`, which port-api rejects). From the Port repo: `yarn workspace @port-labs/customer-skills-catalog build`, then restart `ai-service` in process-compose (or your dev stack). Ensure `yarn seed:ai` ran so `_skill` / `_skill_version` blueprints match the active-version model.
- **Missing demo skills:** re-run `yarn seed:demo-skills` in the Port repo after updating skill blueprints.
- **Sync failures:** the script uses `select_all_groups` and `select_all_ungrouped` in the temp config; run `yarn seed:demo-skills` if grouped demo skills are missing after sync.

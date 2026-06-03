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

4. Credentials in the environment or `~/.port/.env`:

   ```sh
   export PORT_CLIENT_ID="..."
   export PORT_CLIENT_SECRET="..."
   export PORT_API_URL="http://localhost:3000/v1"
   export PORT_AI_SERVICE_URL="http://localhost:3016/v1"
   ```

5. Built CLI:

   ```sh
   make build
   ```

## Run

```sh
make e2e-skills-local
```

Or directly:

```sh
./scripts/e2e-skills-local.sh
```

Optional overrides:

| Variable | Default |
| -------- | ------- |
| `PORT_BIN` | `./bin/port` |
| `PORT_API_URL` | `http://localhost:3000/v1` |
| `PORT_AI_SERVICE_URL` | `http://localhost:3016/v1` |
| `ORG` | `org_BneDtWovPqXaA2VZ` |

## Isolation

The script uses `--config-file` under `$TMPDIR/port-cli-e2e-*` and sets `CURSOR_CONFIG_DIR` to a temp `.cursor` directory so your real `~/.port/config.yaml` and `~/.cursor/skills` are not modified.

## Scenarios covered

- API preflight and authenticated `port skills list`
- Demo catalog list/search/sync
- Optional skill selection
- Single and batch `port skills create`
- Duplicate create (409 / non-zero exit)
- `port skills edit --publish`
- Archive command removed

## Troubleshooting

- **Auth failures:** set `PORT_CLIENT_ID` / `PORT_CLIENT_SECRET` or run `port auth login` with the same `--config-file` pattern.
- **Missing demo skills:** re-run `yarn seed:demo-skills` in the Port repo after updating skill blueprints.
- **Sync/select failures:** run `port skills init` once against the temp config if you need a saved selection; the script uses `select --skill` when possible.

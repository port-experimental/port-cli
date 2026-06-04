# Local skills E2E smoke test

Manual end-to-end validation for `port skills` against a **live** Port stack. Not run in CI (`go test ./...` excludes the `e2e` build tag).

## Prerequisites

1. Local Port stack (`port-api`, `ai-service`, `admin-service` for team tests).
2. `port auth login --org demo` (or your org) — token in `~/.port/creds.json`.
3. `organizations.demo` (or your `ORG`) in `~/.port/config.yaml` with `api_url`.
4. `yarn seed:demo-skills` in the Port monorepo.

## Run

```sh
make e2e-skills-local
```

| Variable | Default |
| -------- | ------- |
| `ORG` | `demo` |
| `PORT_API_URL` | `http://localhost:3000/v1` |
| `PORT_AI_SERVICE_URL` | `http://localhost:3016/v1` |
| `E2E_ADMIN_URL` | `http://localhost:3002/v0.1` |
| `E2E_TEAM_NAME` | (auto: first owning team of `demo-engineering-required`) |
| `PORT_BIN` | `./bin/port` |

Direct:

```sh
PORT_E2E_SKILLS=1 go test -tags=e2e -count=1 -timeout=15m ./e2e/skills/ -v
```

## Isolation

Each subtest resets **`$TMPDIR/port-cli-e2e-*/workdir/.cursor/skills/port`** and writes a fresh temp `config.yaml` (org auth from `~/.port/config.yaml`, symlinked `creds.json`). Disk assertions use **identifier + active catalog from ai-service** (version, description, body snippet), not hardcoded semver strings.

## Scenarios

- Preflight (health, list version matches grouped catalog)
- Search
- Full catalog sync + active-version disk checks + legacy superseded guard
- `selected_groups` narrow/widen + prune
- **Team ownership** — assigns the test user to the team that owns `demo-engineering-required`, then syncs with `team_group_defaults` + `include_groups`
- CRUD (create, batch, duplicate guard, edit)
- `archive` subcommand removed from CLI tree

## Troubleshooting

- **Auth:** `port auth login --org demo`; confirm `organizations.demo` in `~/.port/config.yaml`.
- **Catalog empty:** run `yarn seed:demo-skills` in the Port monorepo.
- **Team test:** admin-service must be reachable; token needs permission to assign teams. Override team with `E2E_TEAM_NAME` if needed.

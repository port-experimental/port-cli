# Local skills E2E smoke test

Manual end-to-end validation for `port skills` against a **live** Port stack. Not run in CI.

## Prerequisites

1. Local Port stack (`port-api`, `ai-service`).
2. `port auth login --org demo` (or your org) — token in `~/.port/creds.json`.
3. `organizations.demo` (or your `ORG`) in `~/.port/config.yaml` with `api_url`.
4. `yarn seed:demo-skills` in the Port monorepo.
5. **python3** (stdlib only — no pip/PyYAML).

## Run

```sh
make e2e-skills-local
```

| Variable | Default |
| -------- | ------- |
| `ORG` | `demo` |
| `PORT_BIN` | `./bin/port` |

## Isolation (clean slate)

Each scenario:

1. Deletes **`$TMPDIR/port-cli-e2e-*/workdir/.cursor/skills/port`** only (not your real `~/.cursor`).
2. Writes a **fresh** temp `config.yaml` with org credentials copied from `~/.port/config.yaml` but **no** skills selection/hooks from your machine.
3. Symlinks `~/.port/creds.json` next to that temp config.

Disk checks only inspect the isolated `skills/port` tree. They do not read your home-directory skills. Extra demo skills elsewhere are ignored; unexpected skills **under the isolated tree** fail the test.

## Scenarios

- Preflight, list/search
- Full catalog sync + active version markers (`selected_groups` for all demo groups)
- Narrow/widen `selected_groups` + reconcile (prune optional skills from disk)
- CRUD smoke, archive removed

## Troubleshooting

- **AUTH_FAILED:** `port auth login --org demo`; confirm `organizations.demo` exists in `~/.port/config.yaml`.
- **Batch create:** see stderr tail printed on failure.

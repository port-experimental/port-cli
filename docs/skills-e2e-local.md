# Local skills E2E smoke test

Manual end-to-end validation for `port skills` against a **live** Port stack. Not run in CI (`go test ./...` excludes the `e2e` build tag).

## Prerequisites

1. Local Port stack (`port-api`, `ai-service`, `admin-service` for team tests).
2. `port auth login --org demo` (or your org) — token in `~/.port/creds.json`.
3. `organizations.demo` (or your `ORG`) in `~/.port/config.yaml` with `api_url`.
4. `yarn seed:general` in the Port monorepo (includes `scripts/demo-skills/`).

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
| `E2E_TEAM_NAME` | (auto: first owning team of `platform-engineering`) |
| `PORT_BIN` | `./bin/port` |

Direct:

```sh
PORT_E2E_SKILLS=1 go test -tags=e2e -count=1 -timeout=15m ./e2e/skills/ -v
```

## Isolation

Each subtest resets **`$TMPDIR/port-cli-e2e-*/workdir/.cursor/skills/port`** and writes a fresh temp `config.yaml` (org auth from `~/.port/config.yaml`, symlinked `creds.json`). Disk assertions use **identifier + active catalog from ai-service** (version, description, body snippet), not hardcoded semver strings.

**Where skills land on disk**

| Scenario | Paths |
| -------- | ----- |
| Most subtests (`beginScenario`) | Temp workdir only: `.cursor/skills/port` (configured target) |
| `SyncWithoutInit` / `CLISyncWithoutInit` | Fake `HOME` under the temp run dir: **`$HOME/.agents/skills/port`** and **`$HOME/.claude/skills/port`** (default sync without `port skills init`) |
| `CLISyncExcludeLegacyInternal` | CLI `sync --exclude-legacy --exclude-internal` vs full sync |
| Project-scoped demo skills | Under each configured `project_dir` (temp workdir), in the tool subdirs derived from targets (e.g. `.cursor/skills/port`) |

E2E does **not** write to your real `~/.agents` or `~/.claude`. Skills are **not** written under the port-cli repo (except read-only `e2e/skills/testdata/` fixtures).

### Finding skills after a run

Each run uses a new directory unless you pin it:

```text
$TMPDIR/port-cli-e2e-<runId>/
  workdir/.cursor/skills/port/          ← most subtests
  sync-no-init-home/.agents/skills/port/
  sync-no-init-home/.claude/skills/port/
  cli-sync-no-init-home/.agents|/.claude/skills/port/
```

With `-v`, the test logs the exact paths at startup (`E2E artifact root: ...`).

```sh
# Latest run (macOS/Linux)
ROOT="$(ls -td "${TMPDIR:-/tmp}"/port-cli-e2e-* 2>/dev/null | head -1)"
echo "$ROOT"
find "$ROOT" -path '*/skills/port/*' -name SKILL.md | head

# Pin the same directory across runs (easier to inspect)
E2E_RUN_ID=debug PORT_E2E_SKILLS=1 go test -tags=e2e -count=1 ./e2e/skills/ -v
# → $TMPDIR/port-cli-e2e-debug/workdir/.cursor/skills/port/...
```

## Scenarios

- Preflight (health, list version matches grouped catalog)
- Search
- Full catalog sync + active-version disk checks + legacy superseded guard
- `selected_groups` narrow/widen + prune
- **Team ownership** — assigns the test user to the team that owns `platform-engineering`, then syncs with `team_group_defaults` + `include_groups`
- **UploadLifecycle** — name/folder mismatch rejected (pack + CLI); create without publish; batch create+edit with list version checks; publish via upload; unpublish clears published catalog
- `archive` subcommand removed from CLI tree

## Troubleshooting

- **Auth:** `port auth login --org demo`; confirm `organizations.demo` in `~/.port/config.yaml`.
- **Catalog empty:** run `yarn seed:general` in the Port monorepo.
- **Team test:** admin-service must be reachable; token needs permission to assign teams. Override team with `E2E_TEAM_NAME` if needed.

# Port API CLI Coverage Plan

Date: 2026-07-17  
Branch: `experiment/cli-architecture-analysis`  
Docs baseline: [Port API reference](https://docs.port.io/api-reference/port-api)  
Beads: epic `port-cli-k0v`, phase 1 `port-cli-leq`

## Goal

Expose remaining Port API families through `port api <resource>` factory commands, while keeping `port api call` as the escape hatch for niche or rapidly changing routes.

Do **not** reintroduce full OpenAPI client generation (see `2026-07-17-api-client-generation-reassessment.md`). Prefer `doEnvelope` client wrappers + `APIResourceSpec` factory registration + contract tests.

## Current coverage (summary)

| Family | `port api` | Client methods | Notes |
|--------|------------|----------------|-------|
| Blueprints / entities / pages / scorecards | Strong | Strong | Core catalog |
| Actions / action-runs / webhooks / audit | Strong | Strong | |
| Teams | Strong | Strong | |
| Users | Strong | Strong | Phase 1: invite/update/delete/roles |
| Agents / AI invoke | Partial | Thin (`Request`) | Enough for invoke/get |
| Integrations | Strong | Strong | Phase 1 core; metrics/logs deferred |
| Migrations | Strong | Strong | Phase 1 (`/v1/migrations`) |
| Permissions | Strong | Strong | Nested under `port api permissions` |
| Skills | Separate `port skills` | Separate | Out of scope for `port api` |
| Workflows | Strong | Strong | Phase 3 complete (CRUD, runs, nodes/logs/triggers) |
| Organization / secrets | Strong | Strong | Phase 2 |
| LLM management | Strong | Strong | Phase 4 |
| Memory | Strong | Strong | Phase 4 |
| Catalog auto-discovery | Strong | Strong | Phase 4 |
| MCP connectors | Strong | Strong | Phase 5 |
| Apps / credentials | Strong | Strong | Phase 5 |
| Plugins | Strong | Strong | Phase 5 |

## Phased rollout

### Phase 1 — Integrations + migrations + users

**Status:** Complete (`port-cli-leq`)

**Users** (`port api users`):

- Keep: `list`, `get`
- Add: `invite`, `update`, `delete`, `change-account-role`, `change-company-role`

**Integrations** (`port api integrations`):

- Add: `list`, `get`, `update`, `update-config`, `delete`
- Defer: metrics, event logs, sync metadata, config schema, member-view metadata

**Migrations** (`port api migrations`):

- Add: `list`, `get`, `create`, `cancel`
- Note: these are Port blueprint property migrations (`/v1/migrations`), not `port migrate` org-to-org transfer

**Acceptance:**

- Client wrappers use `doEnvelope` / `doNoContent` where possible
- Specs registered in `RegisterAPI` and `allFactoryResourceSpecs`
- Wrapper + factory contract tests green
- `make check` passes

### Phase 2 — Organization (+ secrets)

**Status:** Complete (`port-cli-6sy`)

**Organization** (`port api organization`):

- `get`, `update` (PATCH), `replace` (PUT)

**Secrets** (`port api secrets`):

- `list`, `get`, `create`, `update`, `delete` against `/v1/organization/secrets`
- Defer: app credential rotation (`/v1/apps/:id/rotate-secret`) to Phase 5

**Acceptance:** same as Phase 1 (wrappers, factory registration, contract tests, `make check`).

### Phase 3 — Workflows

**Status:** Complete (`port-cli-ntk` + `port-cli-7pk`)

**Workflows** (`port api workflows`):

- `list`, `get`, `create`, `update`, `delete`
- `get-node`, `list-triggers`

**Workflow runs** (`port api workflow-runs`):

- `list`, `get`, `trigger`, `cancel`
- `logs`, `node-logs`, `update-node-run`, `write-node-logs`

### Phase 4 — AI platform adjacent

**Status:** Complete (`port-cli-jdv`)

**LLM providers** (`port api llm-providers`):

- `list`, `create`, `get-defaults`, `set-defaults`

**Memory** (`port api memory`):

- `list`, `delete`, `get-settings`, `update-settings`

**Auto-discovery** (`port api auto-discovery`):

- `create`, `active`, `latest`, `suggestions`, `review`, `update-suggestion`

### Phase 5 — Extensibility

**Status:** Complete (`port-cli-24n`)

**MCP** (`port api mcp`):

- `list-servers`, `get-server`, `disconnect`, `list-templates`, `list-port-tools`, `list-tools`, `call-tool`, `session-token`

**Apps** (`port api apps`):

- `list`, `update`, `delete`, `rotate-secret`, `rotate-user-credentials`

**Plugins** (`port api plugins`):

- `list`, `get`, `update`, `delete`, `upload-url`, `update-upload-url`, `finalize-upload`, `install`

## Working rules

1. Prefer factory specs over bespoke `register*` functions.
2. Add positive + negative client tests for new wrappers.
3. Update CHANGELOG from the user perspective.
4. Leave uncovered routes reachable via `port api call METHOD /path`.
5. Re-check docs when adding a family — paths and envelope keys drift.

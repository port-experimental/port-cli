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
| Users | Partial (list/get) | Partial | Phase 1 completes invite/update/delete/roles |
| Agents / AI invoke | Partial | Thin (`Request`) | Enough for invoke/get |
| Integrations | Missing | Partial (list/config/delete) | Phase 1 |
| Migrations | Missing | Partial (create/get) | Phase 1 |
| Permissions | Strong | Strong | Nested under `port api permissions` |
| Skills | Separate `port skills` | Separate | Out of scope for `port api` |
| Workflows | Missing | Missing | Large; later phase |
| Organization / secrets | Missing | Missing | Later |
| LLM management | Missing | Missing | Later |
| Memory | Missing | Missing | Later |
| Catalog auto-discovery | Missing | Missing | Later |
| MCP connectors | Missing | Missing | Later |
| Apps / credentials | Missing | Missing | Later |
| Plugins | Missing | Missing | Later |

## Phased rollout

### Phase 1 — Integrations + migrations + users (this PR)

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

### Phase 2 — Organization

- Org details, secrets, and related admin routes used in day-to-day ops

### Phase 3 — Workflows

- Large surface; ship list/get/create/update/delete first, then runs/executions

### Phase 4 — AI platform adjacent

- LLM management, memory records, auto-discovery invocations

### Phase 5 — Extensibility

- MCP connectors, apps/credentials, plugins

## Working rules

1. Prefer factory specs over bespoke `register*` functions.
2. Add positive + negative client tests for new wrappers.
3. Update CHANGELOG from the user perspective.
4. Leave uncovered routes reachable via `port api call METHOD /path`.
5. Re-check docs when adding a family — paths and envelope keys drift.

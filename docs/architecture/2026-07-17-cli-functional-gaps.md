# Port CLI Functional Gaps

Date: 2026-07-17  
Branch: `experiment/cli-architecture-analysis`  
Related: [API coverage plan](./2026-07-17-api-coverage-plan.md), [architecture refactor summary](./2026-07-07-cli-architecture-refactor-summary.md)

## Context

Architecture work (command runtime, shared apply/diff/snapshot, API factory, DoJSON helpers) and **`port api` coverage phases 1–5** are complete. This document captures **product / functionality** gaps—not further structural refactors.

The intentional split:

| Surface | Purpose |
|---------|---------|
| `export` / `import` / `migrate` / `compare` / `clear` | Org **catalog config** transfer and drift |
| `port api <resource>` (+ `port api call`) | Direct API ops, including admin/operational families |
| `port skills` | Separate AI skills product surface |

## What the data plane covers today

`--include` / resource flags for export, import, migrate, and compare:

- blueprints, entities, scorecards
- actions, automations
- pages (plus sidebar folders as supporting payload)
- integrations, teams, users
- blueprint-permissions, action-permissions, page-permissions

`port clear` supports a subset: entities, actions, scorecards, automations, pages, blueprints (with documented limits below).

## Gap matrix

| Capability | Data plane | `port api` | Gap |
|------------|------------|------------|-----|
| Blueprints / entities / scorecards | Yes | Yes | — |
| Actions / automations | Yes | Yes (actions) | Automations via actions filter / include |
| Pages + folders | Yes | Pages yes | Folders mostly via import path |
| Integrations / teams / users | Yes | Yes | User **invite** vs import **STAGED** `_user` (see below) |
| Permissions | Yes | Yes | — |
| Webhooks | No | Yes | Highest-value migrate-loop gap |
| Workflows / workflow runs | No | Yes | Large; definitions-only migrate is a product decision |
| Organization / secrets | No | Yes | Admin; usually not org-to-org copy |
| LLM / memory / auto-discovery | No | Yes | Operational; keep API-only unless product asks |
| MCP / apps / plugins | No | Yes | Admin/extensibility; keep API-only unless product asks |
| Action runs / audit / agents / AI invoke | No | Yes | Operational; API-only is correct |
| Blueprint property migrations (`/v1/migrations`) | Via import `--on-error` recreate path | Yes (`port api migrations`) | Naming clash with `port migrate` — document clearly |

## Prioritized functional opportunities

### P0 — Correctness / trust for new `port api` families

New Phase 4–5 wrappers guess response envelope keys (`providers`, `memory`, `triggers`, `uploadUrl`, etc.). `decodeEnvelope` currently returns `(zero, nil)` when a key is missing, so a successful HTTP call can print empty/null and look like a healthy empty result.

**Actions:**

1. Fail closed (or return raw envelope) when the expected key is absent.
2. Live spot-check list/get for organization, integrations, workflows, llm-providers, secrets, mcp, apps, plugins.
3. Extend `make live-smoke` with read-only list calls for those families when credentials exist.

### P1 — Webhooks in the catalog migrate loop

Webhooks have full CRUD under `port api webhooks` but are **not** in export / import / migrate / compare / `--include`.

This is the most common “I can manage it in API but not migrate it” gap after the core catalog.

**Actions:** collector + diff + apply + `--include webhooks` + clear policy (delete via clear vs api-only) + tests and CHANGELOG from the user perspective.

### P1 — `port clear` sandbox rebuild completeness

Documented limitations:

- Does not delete teams, users, integrations, or permissions
- `--pages` deletes root sidebar pages/folders only (not nested children)

So `clear → import → compare` cannot fully reset an org without manual `port api` deletes.

**Actions (pick one product path):**

- A: Extend clear for teams/users/integrations (permissions optional / dangerous).
- B: Keep clear limited; document a “full sandbox reset” recipe that pairs clear with explicit `port api` deletes.

### P1 — Docs: catalog migrate vs admin API

README and `docs/api/CLI_API_COMMANDS.md` still under-describe the expanded `port api` surface and do not explain what **does not** transfer via migrate.

**Actions:** refresh command lists (from factory/`--tree`); add a one-page matrix (this doc can be linked from README).

### P2 — Workflows as optional migrate resource

Workflow definitions are a large surface. Runs/logs/triggers should stay API-only.

**Actions:** if product wants parity, ship definitions-only export/import/migrate first; defer runs.

### P2 — Factory UX polish

- Several write ops require `--data` even for empty bodies (e.g. workflow-runs trigger).
- Client methods accept query params (migrations, memory) but CLI always passes `nil`.

**Actions:** optional `--data`; ExtraFlags for common filters where docs justify them.

### P2 — Users: invite vs STAGED import

- `import` / `migrate` create/update `_user` entities as **STAGED** (no invite email); `--users-as-disabled` for non-admins.
- `port api users invite` sends the invitation path.

Easy to misuse. Document in README “User Import” and migrate help; consider a warning when both patterns appear in the same runbook.

## Explicit non-goals (for now)

- Full OpenAPI client generation (see [reassessment](./2026-07-17-api-client-generation-reassessment.md)).
- Putting LLM, memory, MCP, plugins, or action-runs into export/migrate by default.
- Big ApplyPlan rewrite or skills architecture pass unless a concrete functional bug forces it.
- Treating `port api migrations` (blueprint property jobs) as the same feature as `port migrate` (org transfer).

## Suggested sequencing

1. Envelope-key harden + live smoke for new API families  
2. README / CLI API docs + link this matrix  
3. Webhooks in export/import/migrate/compare  
4. Clear completeness **or** documented reset recipe  
5. Optional: workflows definitions-only; factory UX polish  

## Acceptance for closing a gap

For any resource added to the data plane:

- Collector, diff, apply, and `--include` (all four of export/import/migrate/compare where applicable)
- Clear policy documented
- Positive + negative tests (Eric’s style)
- CHANGELOG from the user’s perspective
- migrate/import parity per `.cursor/rules/port-cli-pr-quality.mdc`

# Port CLI Architecture Refactor Summary

Branch: `experiment/cli-architecture-analysis`

The architecture refactor stack through PR 22 is complete. The main result is a thinner command layer, shared resource semantics, plan-driven import/migrate summaries, and a faster shared entity apply pipeline.

## Status

- PRs 1-22 complete.
- `make check` passed after PR 22.
- `internal/commands/api.go` is down to 163 lines and only keeps generic `api call`.

## Architecture Improvements

### Command Runtime

Config loading, org resolution, authentication, and API client creation are centralized through `Runtime` and `clientForAPICommand`.

This reduces duplicated credential setup and lets command handlers focus on CLI UX and delegation.

### Output Rendering

Export, import, migrate, and compare rendering moved into renderer packages. Import and migrate also share apply-count rendering through `internal/render/apply_counts.go`.

This keeps text and JSON output contracts aligned and reduces resource-by-resource summary drift.

### API Command Factory

Most `port api` resource commands now use `APIResourceSpec` descriptors through `registerAPIResource`.

The command layer is now data-driven for standard resource operations:

- teams
- users
- webhooks
- blueprints
- entities
- pages
- scorecards
- actions
- action-runs
- audit
- permissions
- agents
- ai

Only generic `api call` remains hand-written because it is structurally different from resource commands.

### API Request Helpers

Most standard API wrapper methods moved to shared helpers such as `DoJSON`, `doEnvelope`, and `doNoContent`.

This cuts request/path/decode boilerplate while keeping wrapper methods and tests as the public contract.

### Resource Semantics

Resource identity and normalization now live in `internal/resources`, and compare/import/migrate use a shared diff engine.

This gives compare, import dry-run, and migrate dry-run the same basis for deciding creates, updates, skips, and permission updates.

### Snapshot Collection

Read-side collection moved behind snapshot collect plans and `snapshot.Collector`.

Compare, migrate source collection, and export metadata collection now reuse the same include/filter semantics instead of each reimplementing collection behavior.

### Plan-Driven Import and Migrate

Migrate now operates around execution plans and plan summaries. Import dry-run also routes predicted counters through plan-derived `ApplyCounters`.

Migrate apply delegates to import apply instead of maintaining a parallel implementation.

### Entity Bulk Pipeline

Entity create/update logic moved into a shared bulk pipeline with chunking, partial failure accounting, and migrate delegation.

For bulk-safe paths, this can reduce entity upsert API calls by up to roughly 20x.

## Measured Shape Change

| Area | Before | After | Net effect |
| --- | ---: | ---: | --- |
| `internal/commands/api.go` | ~2,971 lines | 163 lines | Only generic `api call` remains hand-written. |
| `internal/modules/migrate/migrate.go` | ~2,043 lines | 548 lines | Apply path delegates to import and plan summaries. |
| `internal/api/requests.go` | ~1,295 lines | 572 lines | Standard wrappers use shared endpoint helpers. |
| Shared packages | Limited | `resources`, `diff`, `snapshot`, `render`, `plan`, `entities` | Core semantics moved out of command handlers and migrate-specific code. |

## Possible Gaps

### High: `import_module/import.go` Is Still Very Large

`internal/modules/import_module/import.go` remains around 3,070 lines and still owns many phases.

Suggested follow-up: split phase orchestration, importer operations, retry/topological ordering, and result assembly into cohesive files or subpackages.

### Medium: Execution Plan Is Not Yet the Only Apply Interface

Migrate delegates through import, but import apply still consumes `DiffResult`-shaped inputs in several paths.

Suggested follow-up: introduce an `ApplyPlan` boundary so dry-run and apply parity becomes stronger and easier to test.

### Medium: API Factory Specs Are Growing

API factory specs are split by domain under `internal/commands/api_factory_specs_*.go`, with `allFactoryResourceSpecs()` remaining the registry in `api_factory_specs.go`.

Suggested follow-up: only revisit further splits if a single domain file becomes a review bottleneck again.

### Medium: Generated Command Coverage Is Representative, Not Exhaustive

Factory tests cover representative flags and contracts, but not every generated command path exhaustively.

Suggested follow-up: add a generated command-tree or golden sweep across all API resource specs.

### Medium: Live Smoke Validation Is Manual

Live smoke is documented in `docs/live-smoke.md` and runnable via `make live-smoke`.

An opt-in GitHub Actions workflow (`.github/workflows/live-smoke.yml`) runs on `workflow_dispatch` only and never prints secrets. Unit tests in `ci.yml` remain the merge gate.

### Low: Generic `api call` Stays Bespoke

This is intentional because generic `api call` does not fit the resource-command factory shape.

Suggested follow-up: leave it bespoke unless a separate generic request UX abstraction emerges.

### Low: OpenAPI or Client Generation Remains Deferred

**Decision (PR 30):** Defer full OpenAPI client generation. Keep helpers + factory.

See `docs/architecture/2026-07-17-api-client-generation-reassessment.md` for the audit and revisit triggers. `docs/api/OPENAPI_STATUS.md` retains tooling notes only.

## Recommended Next Move

The Post-PR 22 roadmap through PR 30 is complete on this branch.

Follow-on work outside that numbered plan: API coverage phases in `docs/architecture/2026-07-17-api-coverage-plan.md` (phases 1–5 complete).

Optional follow-ups outside that plan: further ApplyPlan payload migration, or a typed adapter for a single stable domain (e.g. skills) if churn warrants it.

Live smoke: `make live-smoke` / `docs/live-smoke.md` (opt-in workflow_dispatch CI available).

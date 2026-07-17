# CLI Architecture Refactor Implementation Plan

Date: 2026-07-03
Branch: `experiment/cli-architecture-analysis`
Source analysis: `docs/architecture/2026-07-03-cli-architecture-analysis.md`

## Goal

Turn the architecture analysis into a sequence of small, reviewable implementation PRs that improve the Port CLI architecture without pausing feature delivery.

Primary outcomes:

- Reduce command-layer duplication.
- Centralize config/auth/client setup.
- Make compare/import/migrate use shared resource semantics.
- Improve large-org efficiency, especially entity import/migration.
- Preserve current CLI behavior with contract tests before deeper refactors.

## Guiding Principles

- Keep every implementation PR behavior-preserving unless explicitly marked as a feature or performance change.
- Add tests before moving behavior across package boundaries.
- Prefer seams and adapters over big-bang rewrites.
- Keep command UX stable: flags, output, exit codes, and JSON fields must remain backward-compatible.
- Use live orgs only for read-only or dry-run smoke checks.
- Do not expose credentials in docs, commits, logs, or PR descriptions.

## Current Architecture Hotspots

From the analysis:

- `internal/commands/api.go`: ~2,971 lines, ~44 command registration functions.
- `internal/modules/import_module/import.go`: ~3,044 lines.
- `internal/modules/migrate/migrate.go`: ~2,043 lines.
- `internal/api/requests.go`: ~1,295 lines, ~79 API wrapper methods across API files.
- Total coverage: ~46%.
- `internal/commands`: ~31.8% coverage.
- `internal/api`: ~35.4% coverage.

Live smoke timings from the analysis:

- Blueprint list: <1s per org.
- Compare schema/resources: ~11s.
- Export schema-only: ~5.5s.
- Migrate dry-run schema-only: ~9.3s.

## Proposed PR Sequence

### PR 1: Architecture Safety Net and Contract Tests

**Purpose:** Protect behavior before refactors.

**Scope:** tests and docs only, plus test helpers where needed.

**Files likely touched:**

- `internal/commands/*_test.go`
- `internal/api/*_test.go`
- `internal/modules/compare/*_test.go`
- `internal/modules/import_module/*_test.go`
- `internal/modules/migrate/*_test.go`
- `docs/architecture/*`

**Tasks:**

- [x] Add command-tree snapshot test for `port --tree` with a stable sanitizer for version/build data.
- [x] Add golden help tests for representative commands:
  - `port --help`
  - `port api --help`
  - `port export --help`
  - `port import --help`
  - `port migrate --help`
  - `port compare --help`
  - `port skills --help`
- [x] Add JSON schema/shape tests for:
  - export JSON result
  - import dry-run JSON result
  - migrate dry-run JSON result
  - compare JSON result
  - `--json-errors`
- [x] Add fixture-based equivalence tests:
  - compare identical snapshots returns no diffs
  - import dry-run identical snapshot returns no creates/updates
  - migrate dry-run identical snapshot returns no creates/updates
- [x] Expand API endpoint wrapper tests for high-value wrappers beyond blueprints:
  - entities
  - pages
  - actions
  - scorecards
  - permissions
  - webhooks
- [x] Add a local smoke script or Make target for read-only/dry-run live checks.

**Acceptance criteria:**

- `go test ./...` passes.
- No production behavior changes.
- Tests fail if command/help/output contracts drift unexpectedly.

**Risk:** low.

**Why first:** Refactors below need a stronger harness so agents can move code safely.

---

### PR 2: Command Runtime for Config/Auth/API Client Setup

**Purpose:** Remove repeated config/auth/client boilerplate from command handlers.

**Scope:** introduce a runtime seam and migrate a few commands first.

**Files likely touched:**

- new `internal/commands/runtime.go` or `internal/cli/runtime.go`
- `internal/commands/api.go`
- `internal/commands/export.go`
- `internal/commands/import.go`
- `internal/commands/migrate.go`
- `internal/commands/compare.go`
- command tests

**Design sketch:**

```go
type Runtime struct {
    Flags GlobalFlags
    Config *config.ConfigManager
}

func NewRuntime(ctx context.Context) *Runtime
func (r *Runtime) LoadOrg(ctx context.Context, org string) (*config.Config, *config.OrganizationConfig, error)
func (r *Runtime) ClientForOrg(ctx context.Context, org string) (*api.Client, string, error)
func (r *Runtime) SourceTargetClients(ctx context.Context, source, target string) (*api.Client, *api.Client, error)
```

**Tasks:**

- [x] Create runtime wrapper around `GlobalFlags`, `ConfigManager`, token refresh, and `api.NewClient`.
- [x] Add unit tests for org resolution and token/client setup using temp config files.
- [x] Migrate low-risk commands first:
  - `api blueprints list`
  - `api users list`
  - `config sources` if useful
- [ ] Migrate export/import/migrate/compare only after runtime API stabilizes.
- [x] Keep existing helper functions as compatibility wrappers until all callers migrate.

**Acceptance criteria:**

- No CLI flag/output behavior changes.
- At least three command paths use the runtime.
- `go test ./...` passes.

**Risk:** low/medium.

**Rollback:** command handlers can be moved back to direct config/client setup.

---

### PR 3: Output Renderer Layer

**Purpose:** Move text/JSON rendering out of command handlers.

**Scope:** export/import/migrate/compare output only; do not change data semantics.

**Files likely touched:**

- new `internal/render/` package or `internal/output/renderers/`
- `internal/commands/export.go`
- `internal/commands/import.go`
- `internal/commands/migrate.go`
- `internal/commands/compare.go`
- output tests/goldens

**Design sketch:**

```go
type Format string

const (
    FormatText Format = "text"
    FormatJSON Format = "json"
)

type Renderer[T any] interface {
    Text(w io.Writer, result T, opts TextOptions) error
    JSON(w io.Writer, result T) error
}
```

**Tasks:**

- [x] Create renderer interfaces and common options.
- [x] Extract export JSON/text rendering into `ExportRenderer`.
- [x] Extract migrate JSON/text rendering into `MigrateRenderer`.
- [x] Extract import JSON/text rendering into `ImportRenderer`.
- [x] Extract compare JSON/text/html dispatch into a renderer facade.
- [x] Replace command handler rendering with renderer calls.
- [x] Add golden tests for each renderer.

**Acceptance criteria:**

- Existing JSON fields remain backward-compatible.
- Existing text output remains equivalent except benign whitespace if explicitly accepted by golden updates.
- `go test ./...` passes.

**Risk:** medium because output contracts are user-facing.

**Mitigation:** golden tests from PR 1.

---

### PR 4: API Command Descriptor Factory, First Slice

**Purpose:** Reduce `internal/commands/api.go` duplication by generating simple CRUD commands from descriptors.

**Scope:** migrate one or two resource groups first.

**Files likely touched:**

- new `internal/commands/api_factory.go`
- `internal/commands/api.go`
- `internal/commands/api_test.go`

**Initial target resources:**

- teams
- users
- webhooks or blueprints if tests are strong enough

**Design sketch:**

```go
type APIResourceSpec struct {
    Name string
    Short string
    IDArgs []ArgSpec
    Operations []APIOperationSpec
}

type APIOperationSpec struct {
    Name string
    Use string
    Short string
    Format bool
    DataFile bool
    Force bool
    Run func(ctx context.Context, c *api.Client, args []string, data map[string]interface{}) (any, error)
}
```

**Tasks:**

- [x] Create command factory for list/get/create/update/delete shapes.
- [x] Use runtime from PR 2 for client setup.
- [x] Migrate `teams` commands to descriptor factory.
- [x] Migrate `users` commands to descriptor factory.
- [x] Add tests ensuring generated commands preserve flags, args, help, and output format behavior.
- [x] Measure line count reduction.

**Acceptance criteria:**

- No CLI behavior changes for migrated resources.
- Generated commands pass existing tests.
- `internal/commands/api.go` starts shrinking.

**Risk:** medium.

**Rollback:** keep old registration functions until generated replacements are proven.

---

### PR 5: API Endpoint Abstraction, First Slice

**Purpose:** Reduce decode/path boilerplate in `internal/api/requests.go` without jumping directly to full OpenAPI generation.

**Files likely touched:**

- new `internal/api/endpoint.go`
- `internal/api/requests.go`
- `internal/api/requests_test.go`

**Design sketch:**

```go
type Envelope[T any] struct {
    Key string
}

func (c *Client) DoJSON(ctx context.Context, method, path string, body any, params map[string]string, out any) error
func DecodeEnvelope[T any](key string) func(*http.Response) (T, error)
```

**Tasks:**

- [x] Add `DoJSON` helper around `request` and JSON decode.
- [x] Add generic envelope decode helper.
- [x] Migrate blueprint wrappers to `DoJSON`.
- [x] Migrate page wrappers to `DoJSON`.
- [x] Add table-driven tests for path/method/body/query/response.

**Acceptance criteria:**

- No behavior changes.
- Wrapper code gets shorter.
- Tests cover migrated endpoints.

**Risk:** low/medium.

---

### PR 6: Resource Identity and Normalization Registry

**Purpose:** Create one source of truth for resource identity and normalization across compare/import/migrate.

**Files likely touched:**

- new `internal/resources/registry.go`
- new `internal/resources/normalize.go`
- `internal/modules/compare/differ.go`
- `internal/modules/import_module/diff.go`
- tests

**Design sketch:**

```go
type ResourceKind string

type Descriptor struct {
    Kind ResourceKind
    Identity func(map[string]interface{}) (string, bool)
    Normalize func(map[string]interface{}) map[string]interface{}
    ServerManagedFields []string
}
```

Initial descriptors:

- blueprints
- entities
- scorecards
- actions
- pages
- integrations
- teams
- users
- permissions

**Tasks:**

- [x] Implement descriptor registry and identity functions.
- [x] Move compare identities to registry.
- [x] Move import diff identities to registry.
- [x] Move normalization field lists to registry.
- [x] Add equivalence tests proving compare/import diff agree on identical and changed fixtures.

**Acceptance criteria:**

- Compare and import diff use the same identity functions for migrated resource kinds.
- Existing compare/import/migrate tests pass.

**Status:** Complete — compare uses `internal/diff` for all resource kinds; import uses `DiffForImport` / `DiffPermissions` for all kinds (blueprint system-patch, page nav equality, and integration `IgnoreMissing` handled via `ImportConfig` hooks).

**Risk:** medium/high due to semantic centrality.

**Mitigation:** do one resource kind at a time.

---

### PR 7: Shared Diff Engine

**Purpose:** Make compare, import dry-run, and migrate dry-run consume the same normalized diff engine.

**Prerequisite:** PR 6.

**Files likely touched:**

- new `internal/diff/`
- `internal/modules/compare/`
- `internal/modules/import_module/diff.go`
- `internal/modules/migrate/`

**Tasks:**

- [x] Introduce generic `Diff[T]` / `ResourceDiff` model independent of compare output types.
- [x] Port compare differ to shared engine.
- [x] Port import diff to shared engine for low-risk resources first.
- [x] Preserve import-specific create/update/skip result shape with adapters.
- [x] Add fixture test: same source/target yields consistent compare/import/migrate summaries.

**Acceptance criteria:**

- Compare/import/migrate diff summaries agree for shared fixture snapshots.
- No live behavior regression in dry-run smoke tests.

**Status:** Complete — all import resource kinds use `DiffForImport` / `DiffPermissions`; migrate inherits via `DiffComparer`.

**Risk:** high.

**Mitigation:** staged resource-by-resource migration.

---

### PR 8: Collection Snapshot Model, Read-Only First

**Purpose:** Represent collected org state as reusable snapshots and reduce duplicate collection semantics.

**Files likely touched:**

- new `internal/snapshot/`
- `internal/modules/export/collector.go`
- `internal/modules/compare/fetcher.go`
- possibly migrate source collection adapter

**Design sketch:**

```go
type CollectPlan struct {
    Resources []ResourceKind
    Filters Filters
    IncludePermissions bool
    IncludeEntities bool
}

type Snapshot struct {
    OrgName string
    Data *export.Data
    Metadata SnapshotMetadata
}
```

**Tasks:**

- [x] Define `CollectPlan` and `Snapshot` around existing `export.Data`.
- [x] Wrap existing collector behind `snapshot.Collector`.
- [x] Migrate compare fetcher to use snapshots.
- [x] Add snapshot save/load path only if needed; otherwise keep in-memory first.
- [x] Add tests for include filters and permission/entity toggles.

**Acceptance criteria:**

- Compare uses snapshots internally.
- Export collector behavior remains unchanged.
- Live compare smoke remains healthy.

**Status:** Complete — compare fetcher uses `snapshot.Collector` and `snapshot.LoadFromFile`.

**Risk:** medium.

---

### PR 9: Migrate as Plan Orchestrator

**Purpose:** Convert migrate from resource-specific logic into orchestration over collect/diff/plan/apply.

**Prerequisites:** PR 6–8.

**Files likely touched:**

- new `internal/plan/`
- `internal/modules/migrate/migrate.go`
- `internal/modules/import_module/`

**Design sketch:**

```go
type ExecutionPlan struct {
    Steps []Step
}

type Step struct {
    Kind ResourceKind
    Operation Operation // create, update, skip, delete, permission-update
    Identifier string
    Payload any
}
```

**Tasks:**

- [x] Introduce execution plan type.
- [x] Generate plan from shared diff for blueprints/actions/scorecards/pages/integrations first.
- [x] Render migrate dry-run from plan.
- [x] Apply migrate from plan by delegating to import/apply functions.
- [x] Preserve existing result count fields as plan summaries.

**Acceptance criteria:**

- Dry-run and apply use the same plan.
- Existing migrate tests pass.
- Live schema-only migrate dry-run remains zero-change for identical orgs.

**Status:** Complete — `internal/plan` builds execution steps from `DiffResult`; migrate dry-run and apply lookups (blueprints, scorecards, actions, pages, integrations) use the plan; skip counts derived from plan summary.

**Risk:** high.

---

### PR 10: Shared Entity Bulk Pipeline

**Purpose:** Implement the highest-impact performance improvement for large orgs.

**Prerequisite:** existing `docs/plans/2026-06-18-parallelize-entity-upserts.md`.

**Files likely touched:**

- new `internal/modules/entities/`
- `internal/api/requests.go`
- `internal/modules/import_module/`
- `internal/modules/migrate/`

**Tasks:**

- [x] Implement or verify `BulkUpsertEntities` API client method.
- [x] Add entity batch/chunk helpers with a hard limit of 20.
- [x] Move create/update two-phase entity apply into shared package.
- [x] Make import use shared bulk pipeline.
- [x] Make migrate delegate entity apply to import/shared pipeline.
- [x] Add partial failure accounting and tests.
- [x] Add performance smoke test with fake server and thousands of entities.

**Acceptance criteria:**

- Entity import/migrate produces same counts as before.
- API calls are reduced by batching in tests.
- Existing entity relation two-phase behavior is preserved.

**Status:** Complete — `internal/modules/entities` owns `ProcessChunk` bulk logic; import delegates to it; migrate uses `Importer.ImportEntities` / streaming import; API `CreateUserEntitiesBulk` delegates to `BulkUpsertEntities`.

**Risk:** high.

**Performance target:** reduce entity upsert calls by up to ~20x for bulk-safe paths.

---

### PR 11: Collapse Migrate Apply Into Import Apply

**Purpose:** Remove migrate's duplicated apply implementation.

**Status:** Complete — `Importer.ApplyFiltered` is the shared apply path; migrate `importToTarget` delegates to it with `migrateResultFromImport` for counter mapping.

**Follow-up:** `internal/modules/migrate/apply_equivalence_test.go` asserts migrate apply matches `ApplyFiltered` on shared fixtures.

---

### PR 12: Move Migrate Source Collection Onto Snapshot

**Purpose:** Unify live org collection behind `snapshot.Collector`.

**Status:** Complete — migrate uses `snapshot.MigrateCollectPlan`; export metadata collection uses `snapshot.ExportMetadataCollectPlan` (PR 12 follow-up).

---

### PR 13: Shared CLI Resource Flag Builder

**Purpose:** Centralize `--include` validation and per-resource flag expansion.

**Status:** Complete — `internal/commands/resourceflags` used by export, import, migrate, and compare.

---

### PR 14: Finish Runtime Adoption

**Purpose:** Remove config/auth/client boilerplate from data commands.

**Status:** Complete — export/import use `CredentialsForBaseOrg` / `CredentialsForTargetOrg`; compare fetcher uses `Runtime.OrgClientFactory` with global credential fallback when per-side overrides are empty.

**Deferred:** `internal/commands/api.go` still uses direct `config.NewConfigManager` for permissions, agents, action-runs, audit, and generic call (PR 16+).

---

### PR 15: Expand API Command Factory

**Purpose:** Migrate standard CRUD `port api` resource groups to descriptor factory.

**Status:** Complete — webhooks, blueprints, entities, pages, scorecards, and actions now use `registerAPIResource`; factory supports extra flags (`--blueprint`, `--compact`) and custom delete prompts.

**Follow-up:** permissions, agents, action-runs, audit, and `api call` remain hand-written.

---

### PR 16: Finish DoJSON / doEnvelope Migration

**Purpose:** Complete migration of standard envelope wrappers in `internal/api/requests.go` to `doEnvelope` / `doNoContent`.

**Status:** Complete — entities, scorecards, actions, teams, users, automations, integrations, permissions, action-runs, webhooks, and audit logs now use shared helpers.

**Kept explicit:** `ForEachEntityPage` (pagination), `GetFolders` (sidebar tree walk), `BulkDeleteEntities` (full map response), generic `Call`.

**Tests:** Extended `architecture_endpoint_wrappers_test.go` for entities, teams/users, integrations/action-runs, and audit.

---

### PR 17: Plan-Driven Results and Render Summary

**Purpose:** Centralize import/migrate counter mapping so text and JSON summaries cannot drift resource-by-resource.

**Status:** Complete — `internal/render/apply_counts.go` defines shared `ApplyCounts`, adapters from import/migrate results, and `PopulateApplyCountsJSON` / `PrintApplyCountsText` used by both renderers.

**Follow-up:** `migrate.generateDryRunResult` still maps `plan.Summary` into `migrate.Result` field-by-field (module layer; render path is unified).

---

### PR 18: Finish API Command Runtime Adoption

**Purpose:** Remove hand-rolled config/auth/client setup from remaining `port api` subcommands.

**Status:** Complete — permissions, agents, AI, action-runs, audit, and generic `api call` now use `clientForAPICommand` → `Runtime.ClientForOrg`.

---

### PR 19: Plan-Driven Migrate Counter Mapping

**Purpose:** Centralize `plan.Summary` → `migrate.Result` counter mapping in the module layer.

**Status:** Complete — `plan.ApplyCounters` with `ApplyCountersFromSummary` / `ApplyCountersFromImport`; migrate uses `populateMigrateCounters` for dry-run and apply paths.

---

### PR 20: Import Dry-Run via plan.ApplyCounters

**Purpose:** Route import dry-run counter prediction through the same execution-plan summary path as migrate.

**Status:** Complete — `populateImportResultCounters` maps `plan.ApplyCountersFromSummary` into `import_module.Result` in `generateDryRunResult`.

---

### PR 21: Factory Action-Runs, Audit, and Permissions

**Purpose:** Migrate remaining standard-shaped `port api` groups to `registerAPIResource`.

**Status:** Complete — `actionRunsResourceSpec`, `auditResourceSpec`, and `permissionsChildSpec` in `api_factory_specs.go`; `api.go` retains only agents, AI, and generic `api call`.

---

### PR 22: Factory Agents and AI Commands

**Purpose:** Migrate `port api agents` and `port api ai` to `registerAPIResource` using `client.Request` for custom endpoints.

**Status:** Complete — `agentsResourceSpec` and `aiResourceSpec` added; `api.go` now only contains `api call` plus shared helpers.

**Also fixed:** PR 20 introduced a `plan` ↔ `import_module` import cycle; moved `BuildFromDiffResult` and `ApplyCountersFromImport` into `import_module`.

---

## Post-PR 22 Validation (2026-07-16)

Validated the earlier PR 23–28 next-step plan against the current branch and code shape.

### What still holds

- PRs 1–22 delivered the intended architecture outcomes: thinner commands, shared resource/diff/snapshot/plan layers, migrate apply collapse into import, and shared entity bulk apply.
- `internal/modules/import_module/import.go` (~3,070 lines) is still the dominant hotspot.
- Plan-driven counters exist, but apply still mostly consumes filtered `export.Data` plus `DiffResult`.
- Descriptor-driven `port api` commands and shared render/counters remain the right long-term shape.

### Findings that change the next-step plan

| Earlier proposal | Verdict | Why |
| --- | --- | --- |
| PR 23: one big import.go decomposition | **Rewrite / slice** | Too large as one PR. The file already has extractions (`blueprint_deps`, `entity_stream_import`, `plan_build`, `counters`, `errors`, `worker_pool`). Remaining work is separable: blueprints phases, pages/sidebar, permissions/users, helpers. |
| PR 24: full ApplyPlan apply boundary | **Defer / narrow** | Premature. `migrate.importToTarget` already builds an `ExecutionPlan`, but apply still calls `ApplyFiltered(data, diff)`. The plan is used for dry-run details and skip-aware counters, not for driving apply. A full plan-as-apply rewrite before import decomposition would be high risk and hard to review. |
| PR 25: split API factory specs | **Defer as polish** | `api_factory_specs.go` (~891 lines) is large but stable and low-risk. Not the highest-value next move. |
| PR 26: exhaustive API contract sweep | **Keep, move earlier** | Low-risk safety net before larger import splits. |
| PR 27: create live smoke workflow | **Rewrite** | Already exists as `scripts/live-smoke.sh` and `make live-smoke`. Remaining work is documentation, hardening, and optional CI wiring. |
| PR 28: OpenAPI reassessment | **Keep, keep last** | Docs-only decision; do not block structural work. |

### Current hotspots (approx.)

| Area | Lines | Notes |
| --- | ---: | --- |
| `import_module/import.go` | ~3,070 | Blueprints multi-phase, pages/sidebar, permissions, users, sanitize helpers |
| `commands/api_factory_specs.go` | ~891 | Descriptor data; factory core already extracted |
| `export/collector.go` | ~599 | Secondary; snapshot wrapping already exists |
| `migrate/migrate.go` | ~548 | Mostly orchestration; healthy after PR 11–12 |
| `api/requests.go` | ~572 | Mostly migrated to helpers |
| `commands/api.go` | ~163 | Only generic `api call` remains |

### Already extracted (do not re-plan as new work)

- `blueprint_deps.go` — topological ordering helpers used by blueprint apply
- `entity_stream_import.go` — streaming entity import
- `plan_build.go` / `counters.go` — plan construction and counter mapping
- `errors.go`, `worker_pool.go`, `loader.go`, `diff.go`
- `scripts/live-smoke.sh` + `make live-smoke`

### Stale notes in earlier PR sections

These are historical and should be treated as superseded:

- PR 14 “Deferred”: remaining `api.go` hand-rolled client setup — completed in PR 18.
- PR 15 “Follow-up”: permissions/agents/action-runs/audit hand-written — completed in PRs 21–22.
- PR 17 “Follow-up”: migrate dry-run field-by-field mapping — addressed by PRs 19–20 via `ApplyCounters`.

---

## Updated Next Steps (Post-PR 22)

The next phase should reduce the remaining hotspot (`import.go`) in small behavior-preserving slices, strengthen cheap safety nets first, and only then narrow the plan/apply boundary where `DiffResult` is still required.

### PR 23: Exhaustive API Factory Contract Sweep

**Purpose:** Make every descriptor-generated `port api` command fail tests if args/flags/prompts/endpoints drift.

**Why first:** Low risk, high leverage before touching import apply. Complements existing representative factory tests.

**Status:** Complete — `allFactoryResourceSpecs()` plus `api_factory_contract_test.go` sweep every factory spec for command tree, args arity, `--org`/`--format`/`--data`/`--force`/extra flags; agents/AI `client.Request` method/path/body contracts covered. Also fixed `permissionsChildSpec` function signatures to match Go method expressions (receiver first), which was breaking `RegisterAPI` compilation.

**Scope:**

- Table-drive over every `APIResourceSpec` registered by `RegisterAPI`.
- Assert subcommands, positional args, `--format`, `--data`, `--force`, and resource-specific flags.
- Add focused method/path/body assertions for custom `client.Request` endpoints (agents, AI).
- Keep this PR tests-only unless a real contract bug is found.

**Acceptance criteria:**

- Every registered API resource spec is covered.
- Custom endpoints retain method/path/body contracts.
- `make check` passes.

**Risk:** low.

---

### PR 24: Extract Blueprint Apply Phases From `import.go`

**Purpose:** Move the multi-phase blueprint import implementation out of `import.go` without changing behavior.

**Why next:** `importBlueprints` and related helpers are a large, self-contained, high-risk block (phases, retries, topological ordering). Extracting them first unlocks safer later work and preserves migrate/import parity rules from `.cursor/rules/port-cli-pr-quality.mdc`.

**Status:** Complete — moved `importBlueprints`, `createOrUpdateBlueprint`, `updateBlueprintFields`, and `updateBlueprintFieldsDirect` into `import_blueprints.go` (~547 lines). `import.go` dropped from ~3,070 to ~2,523 lines. No algorithm changes. Also fixed blocking test compile issues (`architecture_dryrun_test` permissions shape; unused `plan` imports in migrate tests) and corrected apply-equivalence test options so teams are not skipped under `SkipEntities`.

**Scope:**

- Extract blueprint create/update phases into focused file(s), for example `import_blueprints.go`.
- Keep using `TopologicalSortAggProps` / `TopologicalSortOwnership` and existing retry semantics.
- Leave `Importer.Import` / `ApplyFiltered` signatures unchanged.
- Do not change counters, warnings, or CLI output.

**Acceptance criteria:**

- Existing blueprint import/migrate tests pass unchanged.
- Positive and negative retry coverage remains (or is added if a phase boundary was previously untested).
- `make check` passes.

**Risk:** medium/high.

**Mitigation:** move code first; no algorithm changes in the same PR.

---

### Main merge compatibility (post-PR 24)

**Status:** Complete — merged `origin/main` into `experiment/cli-architecture-analysis`.

Brought main’s product behavior onto the architecture apply path:

- Org-wide `/actions` endpoints (`GetAllActions`, create/update via `/actions`, `DeleteActionByID`)
- `--on-error` / `BlueprintUpdater` / `ErrorHandlingOptions` wired through import + migrate `ApplyFiltered`
- Relation-target validation before Phase 2a (scoped migrate must not false-flag targets that already exist in the target org)
- Kept shared `diff.DiffForImport` / thin migrate — did **not** restore main’s duplicated migrate apply path

Next feature work resumes at **PR 25**.

---

### PR 25: Extract Pages/Sidebar Apply From `import.go`

**Purpose:** Move page/folder/sidebar pipeline and related sanitize/retry helpers out of `import.go`.

**Why:** Pages/sidebar logic is another large, cohesive chunk (`PlanSidebarPipeline`, ordering, narrow fallbacks, widget/agent merges) and is independent enough from blueprint phases to land separately.

**Status:** Complete — moved sidebar/page types, `importSidebarPipeline`, page/folder clean/sort/plan helpers, create/update narrow fallbacks, widget/agent merges, and sidebar parent / after-item error helpers into `import_pages.go` (~1,072 lines). `import.go` dropped from ~2,524 to ~1,462 lines. No algorithm changes.

**Scope:**

- Extract page/folder pipeline + ordering + create/update fallbacks into focused file(s).
- Extract pure sanitize/clean helpers that pages/actions already share if that keeps the move small.
- Preserve sidebar parent / after-item retry behavior.

**Acceptance criteria:**

- Existing page/sidebar import tests pass.
- No output or counter drift.
- `make check` passes.

**Risk:** medium/high.

---

### PR 26: Extract Remaining Resource Apply + Thin Orchestration

**Purpose:** Finish reducing `import.go` to orchestration: `Import`, `ApplyFiltered`, and wiring.

**Status:** Complete — extracted remaining apply implementations into focused files:
- `import_entities.go` (~299 lines): two-phase bulk entity upsert + ownership filters
- `import_resources.go` (~424 lines): scorecards, actions, teams, users, integrations
- `import_permissions.go` (~183 lines): sanitize/retry + permission updates
- `import.go` dropped from ~1,462 to ~595 lines (orchestrator + shared helpers)

No algorithm changes; public apply APIs unchanged.

**Scope:**

- Extract permissions apply/retry (`importPermissions` and sanitization helpers that are permissions-specific).
- Extract teams/users/scorecards/actions/integrations apply helpers if still inline.
- Keep entity apply delegated to the shared bulk/stream packages already introduced.
- Target: `import.go` becomes a short orchestrator rather than the implementation dump.

**Acceptance criteria:**

- Public apply APIs unchanged.
- Import/migrate apply-equivalence tests pass.
- `import.go` line count drops substantially (target: well under ~1,000 lines; stretch: orchestration-only).
- `make check` passes.

**Risk:** medium.

---

### PR 27: Narrow Plan/Apply Boundary for Diff-Only Inputs

**Purpose:** Stop using `DiffResult` in apply for the few fields that still require it, without rewriting the whole apply engine onto plan steps.

**Why not a full ApplyPlan rewrite yet:** Apply currently needs `DiffResult` mainly for:

- permission updates (`importPermissions`)
- user update email set (`userUpdateEmailsFromDiff`)

Resource payloads still come from filtered `export.Data`. Replacing that entire path with plan payloads is a separate, larger project and should wait until `import.go` is decomposed.

**Status:** Complete — introduced `ApplyContext` / `ApplyContextFromPlan` so apply takes permission updates and user-update emails from the `ExecutionPlan`. `ApplyFiltered` and migrate `importToTarget` no longer accept `DiffResult`. Import `Execute` also derives apply metadata from the plan. Fixture tests cover plan→apply-context extraction and dry-run counter alignment.

**Scope:**

- Derive permission updates and user-update emails from `ExecutionPlan` (or a small apply-context struct built from the plan).
- Update `ApplyFiltered` / migrate call sites so apply no longer needs the full `DiffResult` for those cases.
- Keep `DiffResult` for diffing and filtering.
- Add fixture tests proving dry-run counters and apply permission/user counts stay aligned.

**Acceptance criteria:**

- Apply no longer requires `DiffResult` solely for permissions/user-update metadata.
- Import/migrate dry-run and apply summaries remain equivalent on shared fixtures.
- `make check` passes.

**Risk:** medium.

**Explicit non-goals for this PR:**

- Do not make every create/update payload come from plan steps yet.
- Do not delete `DiffResult`.

---

### PR 28: Harden and Document Live Smoke

**Purpose:** Make the existing live smoke path an intentional part of the architecture workflow.

**Current state:** `scripts/live-smoke.sh` and `make live-smoke` already cover blueprint list, compare, export schema-only, and migrate dry-run schema-only.

**Status:** Complete — hardened `scripts/live-smoke.sh` (clear missing-cred / missing-binary errors, step failure labels, JSON assertions, no credential echo), documented in `docs/live-smoke.md` + README + architecture summary, and added opt-in `.github/workflows/live-smoke.yml` (`workflow_dispatch` only). Unit tests remain the CI gate.

**Scope:**

- Document how to run `make live-smoke` and required env vars in architecture docs / README section.
- Harden output assertions if needed (for example clearer identical-org expectations).
- Optionally add a manual/opt-in CI workflow that never runs by default and never prints secrets.

**Acceptance criteria:**

- Docs point to the existing script/Make target.
- Missing credentials fail clearly.
- No credential leakage in logs.

**Risk:** low.

---

### PR 29: Split API Factory Specs by Domain (Optional Polish)

**Purpose:** Keep descriptor maintenance manageable once import decomposition is underway or complete.

**Status:** Complete — split `api_factory_specs.go` into domain files while keeping `allFactoryResourceSpecs()` as the registry and `api_factory.go` as the single factory:

- `api_factory_specs.go` — registry only
- `api_factory_specs_catalog.go` — blueprints, entities, pages, scorecards
- `api_factory_specs_identity.go` — teams, users
- `api_factory_specs_automation.go` — actions, action-runs, webhooks
- `api_factory_specs_permissions.go` — permissions child helper
- `api_factory_specs_platform.go` — audit, agents, ai

No behavior changes; factory contract tests remain green.

**Scope:**

- Split `api_factory_specs.go` into domain files while keeping `api_factory.go` as the single factory.
- No behavior changes.

**Acceptance criteria:**

- Factory contract tests from PR 23 remain green.
- `make check` passes.

**Risk:** low.

**Priority:** optional; do only if the specs file becomes a recurring review bottleneck.

---

### PR 30: Reassess API Client Generation

**Purpose:** Decide whether OpenAPI/client generation is worth pursuing after the helper/factory refactor.

**Scope:**

- Audit remaining hand-written wrapper/spec maintenance cost.
- Compare current `doEnvelope`/factory approach against generated client ergonomics.
- Document a recommendation: keep helpers, generate selected clients, or defer.

**Acceptance criteria:**

- Written recommendation with trade-offs and migration cost.
- No production code changes unless the recommendation is explicitly accepted.

**Risk:** low.

---

### Explicitly Deferred

- Full “every apply payload comes from `ExecutionPlan` steps” rewrite.
- Large rewrites of `export/collector.go` beyond the existing snapshot wrapper.
- Skills command architecture (`commands/skills*.go`) — separate product surface, not part of this refactor stack.
- Merging OpenAPI generation into the next implementation PRs.

---

## Cross-Cutting Test Strategy

Every implementation PR should run:

```bash
go test ./...
make build
```

When touching CLI UX/output:

```bash
./bin/port --help >/dev/null
./bin/port --tree >/dev/null
./bin/port --json-errors compare --source a --target b --output invalid
```

When touching compare/import/migrate semantics:

- fixture-based compare/import/migrate dry-run equivalence tests
- live read-only/dry-run smoke with temporary config:
  - blueprint list for base/target
  - compare base/target
  - export base schema-only
  - migrate dry-run schema-only
- Prefer `make live-smoke` when credentials are available.

When touching entity performance:

- fake-server bulk batching tests
- large fixture or generated entities
- partial failure and retry tests

## Recommended Branching Strategy

Use one PR per phase where possible:

- `test/api-factory-contract-sweep`
- `refactor/import-blueprints-extract`
- `refactor/import-pages-sidebar-extract`
- `refactor/import-orchestrator-thin`
- `refactor/plan-apply-diff-narrowing`
- `docs/live-smoke-hardening`
- `refactor/api-factory-spec-domains` (optional)
- `docs/api-client-generation-reassessment`

Keep `experiment/cli-architecture-analysis` as the design branch until the completed PR 1-22 stack is reviewed. Follow-up implementation branches should branch from the reviewed baseline.

## Issue Breakdown to Create After Review

Suggested bd issue set:

1. `Add exhaustive generated API command contract sweep`
2. `Extract blueprint apply phases from import.go`
3. `Extract pages/sidebar apply from import.go`
4. `Thin import.go to apply orchestration`
5. `Narrow plan/apply boundary for permissions and user updates`
6. `Harden and document existing live-smoke workflow`
7. `Optional: split API factory specs by domain`
8. `Reassess API client generation`

## Current Recommendation

1. Land/review the completed PR 1–22 architecture stack on `experiment/cli-architecture-analysis`.
2. Start implementation with **PR 23: Exhaustive API Factory Contract Sweep** (cheap safety net).
3. Then do **PR 24: Extract Blueprint Apply Phases** as the first real structural cut into `import.go`.
4. Continue with pages/sidebar extraction and thinning orchestration before any broader plan-as-apply rewrite.
5. Only after those extractions, do the **narrow** plan/apply boundary for permissions and user-update metadata.

Do not start with a full ApplyPlan rewrite or OpenAPI generation. Those remain high-cost relative to the validated remaining hotspot.

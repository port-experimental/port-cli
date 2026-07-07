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

When touching entity performance:

- fake-server bulk batching tests
- large fixture or generated entities
- partial failure and retry tests

## Recommended Branching Strategy

Use one PR per phase where possible:

- `refactor/cli-runtime`
- `refactor/output-renderers`
- `refactor/api-command-factory-teams-users`
- `refactor/api-endpoint-helper-blueprints-pages`
- `refactor/resource-registry-identities`
- `refactor/shared-diff-engine`
- `refactor/snapshot-collector`
- `refactor/migrate-execution-plan`
- `perf/entity-bulk-pipeline`

Keep `experiment/cli-architecture-analysis` as the design branch. Implementation branches should branch from `main` after this plan is reviewed.

## Issue Breakdown to Create After Review

Suggested bd issue set:

1. `Architecture safety net and contract tests`
2. `Introduce command runtime for config/auth/client setup`
3. `Extract output renderers for export/import/migrate/compare`
4. `Create API command descriptor factory for teams/users`
5. `Create API endpoint helper and migrate blueprint/page wrappers`
6. `Create resource identity and normalization registry`
7. `Introduce shared diff engine`
8. `Introduce snapshot collection model`
9. `Refactor migrate around execution plans`
10. `Implement shared entity bulk pipeline`

## Initial Recommendation

Start with PR 1 and PR 2. They are low-risk and will make all later work safer:

1. **Architecture Safety Net and Contract Tests**
2. **Command Runtime for Config/Auth/API Client Setup**

Do not start with the API command factory or shared diff engine until those guardrails are in place.

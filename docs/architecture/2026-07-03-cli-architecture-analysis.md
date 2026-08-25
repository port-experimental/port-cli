# Port CLI Architecture Analysis and Experimental Refactor Plan

Date: 2026-07-03
Branch: `experiment/cli-architecture-analysis`
Scope: docs-only architecture analysis; no code changes proposed in this branch.

## Executive Summary

Port CLI is now functionally broad and substantially safer after the recent P0/P1/P2/P3 remediation work. The next improvement frontier is architectural: reducing duplicated command/API patterns, consolidating import/export/migrate/compare semantics, and improving live-org performance by sharing collection, diff, and batching infrastructure.

The main architectural opportunity is to move from feature-specific orchestration toward a small set of reusable platform primitives:

- **Resource registry**: one descriptor per Port resource defining identity, API routes, dependencies, permissions, import/export behavior, and compare normalization.
- **Command factory**: generate repetitive `port api <resource>` CRUD commands from descriptors instead of hand-writing dozens of Cobra handlers.
- **Collection graph**: replace ad hoc export/migrate collection with a dependency-aware resource graph that fetches only requested resources and shares results across compare/migrate/import.
- **Diff engine**: one normalized diff implementation for compare, import dry-run, and migrate dry-run.
- **Execution planner**: transform diffs into ordered plans for import/migrate, with dry-run and apply sharing the same plan.
- **Bulk entity pipeline**: make bulk upsert, streaming, batching, and retry/backoff the default path for high-volume entity migration/import.
- **Output contract layer**: move text/JSON rendering out of command handlers into reusable result formatters.

If implemented in phases, these changes should reduce duplicated code, improve agent success rates, lower regression risk, and make large-org operations faster and easier to reason about.

## Evidence Collected

### Static Repository Metrics

Largest Go files by line count:

- `internal/modules/import_module/import.go`: ~3,044 lines
- `internal/commands/api.go`: ~2,971 lines
- `internal/modules/migrate/migrate.go`: ~2,043 lines
- `internal/api/requests.go`: ~1,295 lines
- `internal/commands/skills.go`: ~827 lines
- `internal/modules/skills/module.go`: ~815 lines
- `internal/commands/skills_prompts.go`: ~765 lines
- `internal/commands/clear.go`: ~658 lines
- `internal/commands/migrate.go`: ~622 lines
- `internal/modules/export/collector.go`: ~599 lines
- `internal/modules/import_module/diff.go`: ~536 lines
- `internal/config/loader.go`: ~504 lines
- `internal/commands/import.go`: ~464 lines
- `internal/commands/export.go`: ~411 lines

API surface size:

- API client wrapper methods in `internal/api/requests.go` and `internal/api/skills.go`: ~79 methods.
- Dedicated `port api` Cobra registration functions in `internal/commands/api.go`: ~44 functions.

Coverage snapshot from this run:

- Total statements: **46.0%**
- `internal/modules`: **68.3%** average by rough package grouping
- `internal/config`: **49.1%**
- `internal/auth`: **48.9%**
- `internal/api`: **35.4%**
- `internal/commands`: **31.8%**
- `internal/output`: **15.2%**

### Live E2E Smoke Observations

Using temporary credentials/config and read-only/dry-run commands:

- `api blueprints list --org base --format json`: 91 blueprints, ~0.64s
- `api blueprints list --org target --format json`: 91 blueprints, ~0.74s
- `compare --source base --target target --output json`: identical, ~10.90s
- `export --base-org base --skip-entities --output-format json`: success, ~5.46s, ~139 KB archive
- `migrate --source-org base --target-org target --skip-entities --dry-run --output-format json`: success, ~9.31s

Interpretation:

- Live read-only commands are healthy after remediation.
- Compare and migrate dry-run now agree for the tested schema-only path.
- Compare and migrate dry-run still do similar remote collection work independently; a shared collection/diff pipeline could reduce duplicate API calls and make semantics easier to keep aligned.

### Existing Plan Artifacts

Relevant prior plans already exist and should be incorporated rather than duplicated:

- `docs/plans/2026-06-17-api-command-coverage.md`: API command coverage expansion plan.
- `docs/plans/2026-06-18-parallelize-entity-upserts.md`: bulk entity upsert and batching plan.
- `docs/plans/2026-03-10-compare-entities.md`: opt-in entity compare plan.
- `docs/reviews/cli-review-findings-2026-07-02.md`: prior review and P0/P1/P2/P3 remediation status.

## Current Architecture Strengths

- **Clear user-facing command families**: export, import, migrate, compare, clear, api, auth, config, skills.
- **Reasonable module split**: command layer delegates most heavy operations to modules.
- **Streaming support exists**: entity streaming and archive writers are already present for large data paths.
- **Dry-run semantics exist**: import/migrate diff before applying changes.
- **Post-remediation CLI contracts are better**: structured errors, format validation, safer prompts, no-color, config source diagnostics, and dry-run identifiers are now in place.
- **Tests are broad enough to enable refactors**: import/export/migrate/skills have meaningful coverage and can be strengthened incrementally.

## Architectural Pain Points

### 1. Command Layer Duplication

`internal/commands/api.go` is nearly 3,000 lines and contains many near-identical patterns:

- resolve global flags
- load config
- resolve org
- get/refresh token
- create API client
- parse `--data`
- call an API method
- format JSON/YAML output
- sometimes confirm destructive operations

This makes adding resources expensive and increases inconsistency risk.

**Recommendation:** introduce a descriptor-driven command factory.

Conceptual shape:

```go
type ResourceCommandSpec struct {
    Name        string
    Short       string
    IDArgs      []ArgSpec
    Operations  []OperationSpec
}

type OperationSpec struct {
    Name       string
    Use        string
    Method     string
    Path       PathTemplate
    Body       BodySpec
    OutputKey  string
    Destructive bool
}
```

Initial target: API command family only.

Expected payoff:

- Remove hundreds to thousands of repetitive lines.
- Make supported flags uniform.
- Make endpoint tests table-driven.
- Make `port api` command coverage easier to complete.

### 2. API Wrapper Duplication and Weak Typing

`internal/api/requests.go` is large and manually maps many endpoints. It uses `map[string]interface{}` types broadly for Port resources.

This is flexible but creates issues:

- hard to validate request/response shape
- repeated decode boilerplate
- low direct coverage for wrappers
- duplicated path construction
- weak distinction between user-supplied fields and server-managed fields

**Recommendation:** introduce a small endpoint abstraction before considering full OpenAPI generation.

Conceptual shape:

```go
type Endpoint[T any] struct {
    Method string
    Path   func(args ...string) string
    Decode func(*http.Response) (T, error)
}
```

Then migrate wrapper methods gradually. This can coexist with current `api.Client.request`.

Expected payoff:

- Fewer hand-written decode blocks.
- Easier wrapper tests.
- A stepping stone toward generated OpenAPI clients.

### 3. Export, Compare, Import, and Migrate Have Separate Semantics

There are currently overlapping but separate concepts across modules:

- export collection logic in `internal/modules/export/collector.go`
- import diffing in `internal/modules/import_module/diff.go`
- compare diffing in `internal/modules/compare/differ.go`
- migrate source export logic in `internal/modules/migrate/migrate.go`
- system blueprint handling in `internal/modules/system_blueprints`
- permission comparison/import handling spread across import/migrate/export/compare

Recent P0 work fixed one compare-vs-migrate mismatch, but the architecture still allows drift.

**Recommendation:** centralize resource semantics in a resource registry and shared diff engine.

Conceptual shape:

```go
type ResourceDescriptor struct {
    Name          string
    Identity      IdentityFunc
    Dependencies  DependencyFunc
    Normalize     NormalizeFunc
    Collect       CollectFunc
    DiffPolicy    DiffPolicy
    ApplyPlan     ApplyFunc
    PermissionOf  string
}
```

Compare, import dry-run, and migrate dry-run should call the same diff normalization code.

Expected payoff:

- One source of truth for resource identity and normalization.
- Fewer mismatches between compare/import/migrate.
- Easier support for new resources and permissions.

### 4. Migrate Duplicates Export and Import Concepts

`internal/modules/migrate/migrate.go` has its own source collection, dependency resolution, dry-run generation, and apply logic. It overlaps heavily with export and import modules.

**Recommendation:** redefine migrate as orchestration over existing primitives:

1. collect source data using the shared collection graph
2. collect target data using the same graph
3. diff using shared diff engine
4. create an execution plan
5. dry-run renders plan, apply executes plan

Migrate should own source/target credential resolution and high-level flow, not resource-specific fetch/apply details.

Expected payoff:

- Less duplication.
- Faster fixes when import/export semantics change.
- Easier to reason about dry-run vs apply consistency.

### 5. Entity Workflows Need a First-Class Bulk Pipeline

Entities are the performance hotspot. The repo already has streaming and previous plans for bulk upserts, but entity handling is spread across import, migrate, entity stream, and export.

**Recommendation:** implement a dedicated entity pipeline package.

Conceptual package:

```text
internal/modules/entities/
  stream.go       # paginated fetch and archive stream readers
  batch.go        # batching/chunking
  plan.go         # create/update/phase split
  apply.go        # bulk upsert, retry, result accounting
  filters.go      # blueprint/entity filtering
```

Expected payoff:

- Reusable import/migrate entity behavior.
- Easier throughput tuning.
- One place for bulk limits, retry, partial failures, and count accounting.

### 6. Output Is Still Partly Coupled to Commands

Many commands still print directly with `fmt` or `lipgloss.Printf`. The output package has improved, but output responsibility remains mixed across command handlers and modules.

**Recommendation:** introduce result renderers.

Conceptual shape:

```go
type Renderer[T any] interface {
    Text(w io.Writer, result T, opts TextOptions) error
    JSON(w io.Writer, result T) error
}
```

Targets:

- export result renderer
- import result renderer
- migrate result renderer
- compare result renderer
- skills output renderer

Expected payoff:

- Cleaner command handlers.
- Easier golden tests.
- Better quiet/verbose/json consistency.

### 7. Config/Auth Resolution Is Repeated

Many command handlers independently do:

- `GetGlobalFlags`
- `NewConfigManager`
- `LoadWithOverrides` / `LoadWithDualOverrides`
- `GetOrRefreshToken`
- `api.NewClient`

**Recommendation:** introduce a command runtime/context service.

Conceptual shape:

```go
type Runtime struct {
    Flags GlobalFlags
    Config *config.ConfigManager
}

func (r *Runtime) ClientForOrg(ctx context.Context, org string) (*api.Client, error)
func (r *Runtime) SourceTargetClients(ctx context.Context, source, target string) (*api.Client, *api.Client, error)
```

Expected payoff:

- Less boilerplate in commands.
- Fewer auth/config precedence bugs.
- Easier tests with fake runtime/client factories.

### 8. Concurrency and Rate Limiting Are Localized but Not Centralized

There are several concurrency knobs:

- export collector `maxConcurrentBlueprints`
- migrate collector `maxConcurrentBlueprints`
- entity streaming/import worker pools
- API client retry/backoff

These are implemented locally. They work, but tuning is difficult and behavior can drift.

**Recommendation:** centralize operation scheduling.

Potential abstraction:

```go
type Scheduler struct {
    BlueprintConcurrency int
    EntityConcurrency    int
    RetryPolicy          RetryPolicy
    RateLimitBudget      Budget
}
```

Expected payoff:

- Easier tuning for large orgs.
- Better testability around rate-limit behavior.
- Consistent retry/backoff behavior across modules.

## Efficiency Opportunities

### A. Avoid Duplicate Collection Between Compare and Migrate Dry-Run

Current live timings:

- compare schema/resources: ~10.90s
- migrate dry-run schema: ~9.31s

Both collect overlapping source/target metadata. A shared collection graph could cache or reuse collected source/target data when workflows compose operations.

Potential design:

- `CollectPlan` declares resources, filters, and dependencies.
- `Collector.Execute(plan)` returns `Snapshot`.
- `Snapshot` can be saved, compared, imported, or migrated.

Expected effect:

- Less repeated API traffic.
- Easier offline workflows: export once, compare/migrate many times.

### B. Use Bulk Entity Upsert by Default

Existing plan `docs/plans/2026-06-18-parallelize-entity-upserts.md` estimates ~20x fewer API calls with batches of 20.

Recommended next step:

- Implement bulk upsert for import first.
- Make migrate delegate entity apply to import pipeline.
- Add telemetry counts: batches, partial failures, retried batches, conflicts.

### C. Reduce Permission Fetch Fan-Out

Blueprint/action/page permission fetching contributes many per-resource requests.

Potential improvements:

- Fetch permissions only when selected or required.
- Add include/exclude policy to collection graph.
- Cache permission fetches in a snapshot.
- Consider a future bulk permissions endpoint if Port API supports it.

### D. Normalize Once, Diff Many

Normalization currently occurs in multiple places. Normalize resources once into canonical forms in a snapshot, then diff/import/migrate consume canonical values.

Expected effect:

- Less CPU duplication.
- Fewer semantic mismatches.
- Easier deterministic golden tests.

## Proposed Refactor Roadmap

### Phase 1: Architecture Harness and Safety Nets

Deliverables:

- Add golden CLI tests for command tree/help for high-value commands.
- Add table-driven API endpoint tests for all wrappers.
- Add snapshot fixture tests that run compare/import dry-run/migrate dry-run against the same data.
- Add package-level architecture docs defining desired boundaries.

Why first:

- Protects behavior while refactoring.
- Improves agent success rate on future changes.

### Phase 2: Runtime and Output Abstractions

Deliverables:

- `commands.Runtime` for config/auth/client resolution.
- Shared JSON/text renderers for export/import/migrate/compare.
- Remove direct `fmt`/`lipgloss` printing from most command handlers.

Why second:

- Shrinks command functions without changing domain behavior.
- Makes tests easier before deeper refactors.

### Phase 3: API Command Descriptor Factory

Deliverables:

- Resource operation descriptors for simple CRUD resources.
- Generate `port api blueprints/entities/pages/teams/users/webhooks/...` commands from descriptors.
- Keep custom handlers for complex resources.

Why third:

- Targets the largest command-layer file.
- Reduces repetitive patterns and future drift.

### Phase 4: Resource Registry and Shared Diff Engine

Deliverables:

- `ResourceDescriptor` registry for identity, dependencies, normalization, permissions.
- Compare and import diff use the same diff engine.
- Migrate dry-run uses the same engine and plan output.

Why fourth:

- Addresses core semantic drift.
- Needs stronger test coverage from earlier phases.

### Phase 5: Collection Graph and Snapshot Model

Deliverables:

- `Snapshot` type representing collected Port state.
- Dependency-aware collection plans.
- Export, compare, import dry-run, and migrate dry-run use snapshots.

Why fifth:

- Enables performance gains and offline workflows.
- Provides a durable model for future features.

### Phase 6: Entity Bulk Pipeline

Deliverables:

- Shared entity batching package.
- Bulk upsert apply path.
- Migrate delegates entity apply to import pipeline.
- Detailed throughput metrics and partial failure accounting.

Why sixth:

- Highest performance payoff for large orgs.
- Best done after plan/diff semantics stabilize.

## Suggested Package Boundaries

Target structure:

```text
internal/
  cli/                  # root command assembly, runtime, help policy
  commands/             # thin command declarations only
  portapi/              # API client, endpoints, retry policy
  resources/            # resource descriptors, identities, normalization
  snapshot/             # collection graph and collected state model
  diff/                 # shared diff engine
  plan/                 # execution plan and dry-run/apply outputs
  entities/             # streaming, batching, bulk upsert pipeline
  render/               # text/json output renderers
  config/               # config/env/credential source resolution
  auth/                 # OAuth/token handling
```

This does not need to be a big-bang move. New packages can be introduced as seams and existing code can migrate gradually.

## Recommended Issues to File Next

1. **Introduce command runtime for config/auth/client resolution**
   - Type: refactor
   - Risk: low/medium
   - Payoff: reduces command boilerplate and auth/config bugs.

2. **Add API command descriptor factory for CRUD resources**
   - Type: refactor
   - Risk: medium
   - Payoff: shrinks `internal/commands/api.go` and standardizes command behavior.

3. **Create shared resource identity and normalization registry**
   - Type: architecture
   - Risk: medium/high
   - Payoff: prevents compare/import/migrate drift.

4. **Move export/import/migrate/compare result rendering to dedicated renderers**
   - Type: refactor
   - Risk: low/medium
   - Payoff: output consistency and easier golden tests.

5. **Implement collection snapshot model**
   - Type: architecture
   - Risk: high
   - Payoff: performance and semantic consistency.

6. **Implement shared entity bulk pipeline**
   - Type: performance
   - Risk: high
   - Payoff: major large-org import/migrate speedup.

7. **Expand API endpoint wrapper coverage**
   - Type: test
   - Risk: low
   - Payoff: safer endpoint refactors and generated-client migration.

## Suggested Automated Guardrails

Add a CI workflow or Make target for architecture-sensitive CLI contracts:

- command tree snapshot for `port --tree`
- golden help tests for representative commands
- JSON schema checks for export/import/migrate/compare outputs
- fixture-based compare/import/migrate dry-run equivalence tests
- API endpoint wrapper table tests
- large-entity fake-server performance tests

This is the self-healing loop that would catch future drift between compare, import, and migrate before a user finds it.

## Branch Deliverable Status

This branch intentionally changes no production code. It contains:

- this architecture analysis document
- issue tracking updates for the analysis work

Recommended next action is to review this document, pick the first 2–3 refactor issues, and create implementation PRs with small seams rather than a broad rewrite.

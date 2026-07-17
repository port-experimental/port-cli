# API Client Generation Reassessment (PR 30)

**Date:** 2026-07-17  
**Branch context:** `experiment/cli-architecture-analysis` (post PRs 1–29)  
**Decision:** Defer full OpenAPI client generation. Keep `DoJSON` / `doEnvelope` / `doNoContent` wrappers and the API command factory.

This supersedes the “just waiting on oapi-codegen 3.1” framing in `docs/api/OPENAPI_STATUS.md` for architecture planning. That file remains useful as historical tooling notes and for `make generate-api` mechanics.

---

## Current maintenance surface (audit)

| Area | Approx. size | Role after refactor |
| --- | ---: | --- |
| `internal/api/requests.go` | ~706 lines | Thin resource wrappers; most CRUD is one-line `doEnvelope` / `doNoContent` |
| `internal/api/endpoint.go` | ~53 lines | Shared `DoJSON`, `doEnvelope`, `doNoContent` |
| `internal/api/client.go` | ~303 lines | Auth, retry, transport (not OpenAPI-shaped) |
| `internal/api/skills.go` | ~493 lines | Typed request/response structs + multipart (already “hand-structured”) |
| `internal/commands/api_factory.go` | ~210 lines | Single factory for `port api` resource commands |
| `internal/commands/api_factory_specs_*.go` | ~964 lines total | Domain-split descriptors; registry in `api_factory_specs.go` |
| Factory contract tests | ~389 lines | Exhaustive command-tree / flag / custom-endpoint sweep (PR 23) |

Roughly **70+** `Client` methods in `requests.go` plus **11** skills helpers. Resource domain types (`Blueprint`, `Entity`, `Action`, …) remain `map[string]interface{}` by design.

What the architecture refactor already removed:

- Repeated request/decode boilerplate (replaced by helpers)
- Hand-rolled `port api` command trees for standard resources (replaced by factory)
- Parallel migrate apply paths that duplicated API usage

Remaining hand-written cost is mostly: path strings, envelope keys, a few non-CRUD flows (entity pagination/search, org-wide actions filtering, migrations), and CLI descriptor wiring.

---

## Options compared

### A. Keep helpers + factory (status quo)

**Pros**

- Matches Port’s dynamic schemas (custom blueprint properties, flexible permissions payloads).
- Low review risk; already covered by wrapper tests and factory contract sweeps.
- Auth/retry/`APIError` behavior stays centralized in `Client`.
- Skills already use typed structs where the schema is stable enough.

**Cons**

- New endpoints still need a small wrapper + optional factory spec.
- No compile-time request/response schema from Port’s OpenAPI document.

**Migration cost:** none.

### B. Generate selected clients / types only

Generate types (or client methods) for stable, non-map domains — e.g. skills, audit, action-runs — behind an adapter; leave blueprints/entities/pages/permissions as maps.

**Pros**

- Incremental type safety where it pays off.
- Avoids forcing `map` resources into generated structs.

**Cons**

- Two client styles forever unless carefully adapted.
- Still blocked or noisy if the shared OpenAPI document cannot be generated cleanly.
- Spec tagging/`include-paths` discipline required so generation does not pull map-heavy schemas.

**Migration cost:** medium (adapter, CI generate step, dual test strategy). Only worth it for a domain that churns often and has a clean schema slice.

### C. Full OpenAPI-generated client replacing wrappers

**Pros**

- Broad endpoint coverage from the published spec.
- Potential auto-update when Port publishes schema changes.

**Cons**

- High migration cost: rewrite wrappers, factory data binding, import/export/migrate which assume map payloads, and large test suites.
- Fights product reality: blueprints/entities are intentionally schemaless maps in this CLI.
- Historical tooling friction (OpenAPI 3.1 / nullability / `additionalProperties`) documented in `docs/api/OPENAPI_STATUS.md`; even if tooling improved, product fit remains the larger issue.
- Generated clients typically own HTTP differently than our retry/token/`doEnvelope` layer — needs a durable adapter anyway.

**Migration cost:** high (multi-PR, high regression risk). Not justified by current maintenance pain after the helper/factory refactor.

---

## Recommendation

**Defer full OpenAPI client generation (option C).**  
**Keep option A as the default architecture.**  
**Revisit option B only when a specific typed domain shows clear churn pain** (candidate today: skills; not blueprints/entities).

### Why now

1. The original motivation for generation was boilerplate and command duplication. PRs through 29 addressed that with `doEnvelope` and the API factory.
2. Map-typed core resources are a product constraint for this CLI, not a temporary shortcut. Generated structs would either be wrong or wrap maps again.
3. Safety nets already exist without schema codegen: endpoint wrapper tests, factory contract sweep, live smoke (opt-in).
4. Existing `make generate-api` / `scripts/generate-api.sh` can stay as experimental tooling; they are not on the critical path and should not gate releases.

### Explicit non-goals (unless revisited)

- Do not replace `internal/api.Client` with a generated HTTP client in the near term.
- Do not regenerate factory specs from OpenAPI.
- Do not convert blueprint/entity/page/permission types to generated structs.

### Revisit triggers

Reopen this decision if any of the following become true:

- Port publishes a stable, CLI-consumable OpenAPI subset with clean 3.x generation and tagged domains.
- A typed domain (e.g. skills) requires frequent hand updates that a generated adapter would clearly reduce.
- Envelope/`DoJSON` helpers start accumulating per-endpoint special cases faster than factory/spec growth.

Until then, prefer adding a one-line `doEnvelope` wrapper and a factory spec over introducing a generated client stack.

---

## Relation to prior docs

| Doc | Status |
| --- | --- |
| `docs/api/OPENAPI_STATUS.md` | Tooling history + `make generate-api` notes; recommendation section outdated relative to this reassessment |
| `docs/architecture/2026-07-03-cli-architecture-analysis.md` | Correctly advised endpoint helpers before OpenAPI; that step is done |
| This document | Authoritative architecture decision for PR 30 |

No production code changes accompany this recommendation.

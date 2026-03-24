# Design: Skip System Blueprints Flag + Skip-Entities Bug Fix

**Date:** 2026-03-24
**Status:** Approved

---

## Overview

Two related changes to the `export`, `import`, and `migrate` commands:

1. **New flag `--skip-system-blueprints`**: Skip blueprints whose identifiers start with `_` (e.g. `_user`, `_team`, `_rule`) along with their entities.
2. **Bug fix for `--skip-entities`**: Users and teams are currently collected and imported even when `--skip-entities` is set. They should be skipped too, since they are conceptually the entities of the `_user` and `_team` system blueprints.

Both changes apply to all three commands: `export`, `import`, and `migrate`.

---

## Background

Port has a concept of "system blueprints" — built-in blueprints prefixed with `_` that exist in every organization. The most common are `_user` and `_team`. These blueprints:

- Are pre-existing in every Port organization (they don't need to be created).
- Have their "entities" represented as top-level API resources (`/users`, `/teams`) rather than the generic `/blueprints/{id}/entities` endpoint.

The existing codebase already has `IsSystemBlueprint(identifier string) bool` in `internal/modules/import_module/blueprint_deps.go`, which returns `true` for any identifier starting with `_`.

---

## Feature 1: `--skip-system-blueprints` Flag

### Behavior

When `--skip-system-blueprints` is set:
- Skip exporting/importing any blueprint whose identifier starts with `_`.
- Skip exporting/importing entities belonging to those blueprints (deep skip).
- Scorecards, actions, and permissions for system blueprints are also not processed (they are blueprint-scoped and will not be reached if the blueprint is excluded from iteration).

### Detection

System blueprints are identified by `strings.HasPrefix(identifier, "_")`. This logic is inlined at filter points rather than importing `IsSystemBlueprint()` across package boundaries, avoiding a cross-package dependency between `export` and `import_module`.

### Flag Definition

Added to all three commands:

```
--skip-system-blueprints    Skip system blueprints (identifiers starting with _) and their entities
```

### Options Structs

`SkipSystemBlueprints bool` is added to:

| Struct | File |
|--------|------|
| `export.Options` | `internal/modules/export/export.go` |
| `collector.Options` | `internal/modules/export/collector.go` |
| `import_module.Options` | `internal/modules/import_module/import.go` |
| `migrate.Options` | `internal/modules/migrate/migrate.go` |

### Export Changes (`collector.go`)

After fetching all blueprints and applying the `--blueprints` filter, add a system blueprint filter step:

```go
if opts.SkipSystemBlueprints {
    filtered := allBlueprints[:0]
    for _, bp := range allBlueprints {
        id, _ := bp["identifier"].(string)
        if !strings.HasPrefix(id, "_") {
            filtered = append(filtered, bp)
        }
    }
    allBlueprints = filtered
}
```

Because entity collection iterates over `allBlueprints`, filtering here automatically prevents entity collection for system blueprints — no additional changes needed in the entity loop.

### Import Changes (`import.go`)

Extend `applyDataExclusion()` to handle system blueprint filtering. When `opts.SkipSystemBlueprints`:

1. Remove any blueprint from the blueprints list whose identifier starts with `_`.
2. Remove any entity whose blueprint identifier starts with `_`.

This fits naturally alongside the existing deep/schema-only exclusion logic in `applyDataExclusion()`.

### Migrate Changes (`migrate.go`)

`SkipSystemBlueprints` is added to `migrate.Options` and passed through to both the internal export options and import options. No additional logic is needed since migrate fully delegates to the export and import modules.

---

## Feature 2: `--skip-entities` Bug Fix

### Problem

Users and teams are collected (in export) and imported (in import) as top-level organization resources, independent of the blueprint entity collection loop. The `--skip-entities` flag currently only gates the blueprint entity loop, leaving users and teams unaffected.

Since users and teams are the entity representation of `_user` and `_team` blueprints, they should be skipped when `--skip-entities` is set.

### Export Fix (`collector.go`)

The users and teams collection blocks currently look like:

```go
if shouldCollect("teams", opts.IncludeResources) { ... }
if shouldCollect("users", opts.IncludeResources) { ... }
```

Add `!opts.SkipEntities` guard:

```go
if !opts.SkipEntities && shouldCollect("teams", opts.IncludeResources) { ... }
if !opts.SkipEntities && shouldCollect("users", opts.IncludeResources) { ... }
```

### Import Fix (`import.go`)

The users and teams import sections are gated analogously. Add `!opts.SkipEntities` guard to both.

### Migrate

Migrate delegates to export and import, so the fix propagates automatically once the two modules are updated.

### Interaction with `--include`

If a user explicitly passes `--include users` or `--include teams` alongside `--skip-entities`, `--skip-entities` takes precedence (consistent with the existing behavior for `--skip-entities` + `--include entities`).

---

## Flag Interaction Summary

| Flag combination | Behavior |
|---|---|
| `--skip-system-blueprints` | Skip `_*` blueprint schemas and their entities |
| `--skip-entities` | Skip all entities including users and teams (bug fix) |
| `--skip-system-blueprints` + `--skip-entities` | Both apply independently; system blueprint schemas skipped, all entities (including users/teams) skipped |
| `--skip-system-blueprints` + `--exclude-blueprints _user` | Compose naturally; `_user` excluded by either flag |
| `--skip-system-blueprints` + `--blueprints _user` | `--skip-system-blueprints` takes precedence; `_user` is excluded |

---

## Files Changed

| File | Change |
|------|--------|
| `internal/commands/export.go` | Add `--skip-system-blueprints` flag |
| `internal/commands/import.go` | Add `--skip-system-blueprints` flag |
| `internal/commands/migrate.go` | Add `--skip-system-blueprints` flag |
| `internal/modules/export/export.go` | Add `SkipSystemBlueprints` to `Options` |
| `internal/modules/export/collector.go` | Filter system blueprints; guard users/teams with `!SkipEntities` |
| `internal/modules/import_module/import.go` | Add `SkipSystemBlueprints` to `Options`; extend `applyDataExclusion()`; guard users/teams with `!SkipEntities` |
| `internal/modules/migrate/migrate.go` | Add `SkipSystemBlueprints` to `Options`; pass through to export/import |

---

## Testing

- Unit test: `--skip-system-blueprints` filters `_user`, `_team` blueprints and their entities from export output.
- Unit test: `--skip-system-blueprints` does not filter non-system blueprints.
- Unit test: `--skip-entities` no longer collects/imports users or teams.
- Unit test: `--skip-entities` + `--include users` still skips users (`--skip-entities` wins).
- Integration: migrate with `--skip-system-blueprints` propagates flag to both export and import phases.

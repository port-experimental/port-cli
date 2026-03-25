# Design: Skip System Blueprints Flag + Skip-Entities Bug Fix

**Date:** 2026-03-24
**Status:** Approved

---

## Overview

Two related changes to the `export`, `import`, and `migrate` commands:

1. **New flag `--skip-system-blueprints`**: Shallow skip of blueprints whose identifiers start with `_` (e.g. `_user`, `_team`, `_rule`) — drops their schemas and entities only. Scorecards, actions, permissions, and action permissions for system blueprints still pass through.
2. **Bug fix for `--skip-entities`**: Users and teams currently bypass `--skip-entities`. Fix: add `!opts.SkipEntities` guard everywhere users/teams are collected or imported.

Both changes apply to all three commands: `export`, `import`, and `migrate`.

---

## Background

Port has "system blueprints" — built-in blueprints prefixed with `_` that exist in every organization. The most common are `_user` and `_team`. These blueprints:

- Are pre-existing in every Port organization (they don't need to be created).
- Have their "entities" represented as top-level API resources (`/users`, `/teams`) rather than the generic `/blueprints/{id}/entities` endpoint.

System blueprint detection is inlined as `strings.HasPrefix(id, "_")` at filter points (rather than importing `IsSystemBlueprint()` from `import_module` into `export`, which would create a cross-package dependency).

---

## Options Structs

`SkipSystemBlueprints bool` is added to three structs (there is no separate `collector.Options` — `collector.go` uses `export.Options` from the same package):

| Struct | File |
|--------|------|
| `export.Options` | `internal/modules/export/collector.go` (defined here, used by both export.go and collector.go) |
| `import_module.Options` | `internal/modules/import_module/import.go` |
| `migrate.Options` | `internal/modules/migrate/migrate.go` |

---

## Feature 1: `--skip-system-blueprints`

### What gets skipped (shallow skip)

For every blueprint whose identifier starts with `_`:

| Resource | Behavior |
|---|---|
| Blueprint schema | **Skipped** |
| Entities | **Skipped** |
| Scorecards | Included (pass through) |
| Actions | Included (pass through) |
| Permissions | Included (pass through) |
| Action permissions | Included (pass through) |

System blueprints must remain in the blueprint iteration so their scorecards, actions, and permissions are still collected/imported. Only the schema output and entity fetch are suppressed.

### Flag registration

Added to all three commands in `internal/commands/` with the same pattern as `--skip-entities`:

```
--skip-system-blueprints    Skip system blueprint schemas (identifiers starting with _) and their entities
```

### Export (`collector.go`)

Because system blueprints must remain in `iterBlueprints` (for scorecards/actions/permissions) but not in `dataBlueprints` (schema output), the existing `ApplyBlueprintExclusions` two-list pattern is leveraged. System blueprints are treated as schema-only exclusions for output purposes, with an additional per-blueprint entity skip in the entity loop.

Two changes in `collector.go`:

**1. Add system blueprints to the schema-only exclusion list before `ApplyBlueprintExclusions`:**

```go
if opts.SkipSystemBlueprints {
    for _, bp := range allBlueprints {
        id, _ := bp["identifier"].(string)
        if strings.HasPrefix(id, "_") {
            excludeSchema = append(excludeSchema, id)
        }
    }
}
```

This keeps system blueprints in `iterBlueprints` (scorecards/actions/permissions still collected) but removes them from `dataBlueprints` (schema not in output).

**2. Skip entity fetch for system blueprints in the entity collection loop:**

```go
skipEntitiesForBP := opts.SkipEntities || (opts.SkipSystemBlueprints && strings.HasPrefix(blueprintID, "_"))
if !skipEntitiesForBP && shouldCollect("entities", opts.IncludeResources) {
    // fetch entities
}
```

### Import (`import.go`)

#### `applyDataExclusion` signature

Add a `skipSystemBlueprints bool` parameter to `applyDataExclusion`. When true, perform a pre-pass that removes from `data`:

1. Blueprints with `_`-prefixed identifiers from `data.Blueprints`
2. Entities whose `blueprint` field starts with `_`

Scorecards, actions, permissions (`data.BlueprintPermissions`), and action permissions are **not** removed — they pass through.

This runs before `diff.Compare()`, so the diff never sees system blueprint schemas or their entities, and generates no operations for them.

The existing "Phase 3: Update system blueprints" code path processes system blueprints from `data.Blueprints`. Since `applyDataExclusion` removes them first, Phase 3 is a no-op when the flag is set — no additional changes needed there.

### Migrate (`migrate.go` — `exportFromSource()` and `Execute()`)

`migrate.go` has its own `exportFromSource()` function with inline blueprint filtering and users/teams collection (independent of `collector.go`). Changes:

1. Pass `SkipSystemBlueprints` from `migrate.Options` to the export options used inside `exportFromSource()`.
2. In `exportFromSource()`, apply the same two-part logic as `collector.go`: add system blueprints to `excludeSchema` before `ApplyBlueprintExclusions`, and skip entity fetch for system blueprints in the entity loop.
3. Apply the `!opts.SkipEntities` users/teams guard in `exportFromSource()` (see Feature 2).
4. Pass `SkipSystemBlueprints` to the import options used in the import phase.
5. In `Execute()`, the `diffOpts` struct (of type `import_module.Options`) is constructed from `opts` and passed to `comparer.Compare()`. `SkipSystemBlueprints` must be included in this mapping so the import phase of migrate also applies the system blueprint filter.

---

## Feature 2: `--skip-entities` Bug Fix

### Problem

Users and teams are collected/imported as top-level resources independently of the blueprint entity loop. `--skip-entities` currently only gates the blueprint entity loop.

### Export fix (`collector.go`)

```go
if !opts.SkipEntities && shouldCollect("teams", opts.IncludeResources) { ... }
if !opts.SkipEntities && shouldCollect("users", opts.IncludeResources) { ... }
```

### Import fix (`import.go`)

The guard is added in the `Importer.Import()` method (not `Execute()`), at the `importTeams` and `importUsers` call sites:

```go
if !opts.SkipEntities {
    // import teams
}
if !opts.SkipEntities {
    // import users
}
```

### Migrate fix (`migrate.go` — `exportFromSource()`)

`exportFromSource()` has its own inline `shouldCollect("teams")` and `shouldCollect("users")` blocks. Add `!opts.SkipEntities` guard directly in `exportFromSource()`.

### Conflict resolution: `--skip-entities` + `--include users/teams`

Follows the existing pattern for `--skip-entities` + `--include entities`: if a user passes both, a warning is emitted and `--skip-entities` is disabled for that resource (`--include` wins). This conflict check is added at the **command layer** (in `commands/export.go`, `commands/import.go`, `commands/migrate.go`) alongside the existing `entities` conflict check.

---

## Flag Interaction Summary

| Flag combination | Behavior |
|---|---|
| `--skip-system-blueprints` | Skip `_*` blueprint schemas + entities only; scorecards/actions/permissions still included |
| `--skip-entities` | Skip all entities including users and teams (bug fix) |
| `--skip-system-blueprints` + `--skip-entities` | Compose independently; entities doubly skipped for system blueprints |
| `--skip-system-blueprints` + `--exclude-blueprints _user` | Compose; `_user` schema excluded by either flag |
| `--skip-system-blueprints` + `--exclude-blueprint-schema _user` | Both add `_user` to schema exclusion; harmless duplicate |
| `--skip-system-blueprints` + `--blueprints _user` | `--skip-system-blueprints` takes precedence; `_user` schema excluded |
| `--skip-entities` + `--include users` | Warning emitted at command layer; `--include` wins |
| `--skip-entities` + `--include teams` | Warning emitted at command layer; `--include` wins |

---

## Files Changed

| File | Change |
|------|--------|
| `internal/commands/export.go` | Add `--skip-system-blueprints` flag; add `users`/`teams` conflict check for `--skip-entities` |
| `internal/commands/import.go` | Add `--skip-system-blueprints` flag; add `users`/`teams` conflict check for `--skip-entities` |
| `internal/commands/migrate.go` | Add `--skip-system-blueprints` flag; add `users`/`teams` conflict check for `--skip-entities` |
| `internal/modules/export/collector.go` | Add `SkipSystemBlueprints` to `Options`; add to `excludeSchema` before `ApplyBlueprintExclusions`; skip entities in loop; guard users/teams with `!SkipEntities` |
| `internal/modules/import_module/import.go` | Add `SkipSystemBlueprints` to `Options`; extend `applyDataExclusion` to remove `_*` schemas and entities only; guard users/teams in `Importer.Import()` |
| `internal/modules/migrate/migrate.go` | Add `SkipSystemBlueprints` to `Options`; update `exportFromSource()` and `Execute()` diffOpts; apply users/teams guard |

---

## Testing

- `collector.go`: `--skip-system-blueprints` excludes `_*` blueprint schemas from output but still collects their scorecards/actions/permissions.
- `collector.go`: `--skip-system-blueprints` skips entity fetch for `_*` blueprints.
- `collector.go`: `--skip-system-blueprints` preserves non-system blueprints fully.
- `collector.go`: `--skip-entities` skips users and teams collection.
- `collector.go`: `--skip-entities` + `--include users` emits warning; users are collected (`--include` wins).
- `import.go`: `applyDataExclusion` with `skipSystemBlueprints=true` removes only schemas and entities for `_*` blueprints; scorecards/actions/permissions remain.
- `import.go`: `Importer.Import()` respects `SkipEntities` for users and teams.
- `migrate.go`: `exportFromSource()` respects `SkipEntities` for users and teams.
- `migrate.go`: `SkipSystemBlueprints` propagates to both export and import phases via `diffOpts`.
- Command layer: conflict warnings emitted for `--skip-entities` + `--include users/teams`.

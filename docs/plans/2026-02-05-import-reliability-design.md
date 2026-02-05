# Import Reliability Improvements

## Problem Statement

The current import/migrate functionality fails when targeting a fresh Port organization. Testing revealed 9,296 out of 9,317 operations failing with "context canceled" errors, caused by cascading failures from unresolved blueprint dependencies.

### Root Causes Identified

1. **Incomplete dependency stripping**: Only `relations` are stripped from blueprints, but `mirrorProperties`, `calculationProperties`, and `aggregationProperties` also reference other blueprints
2. **No topological sorting**: Blueprints are created concurrently without respecting dependency order
3. **No concurrency limiting**: 9000+ goroutines spawned simultaneously overwhelms the system
4. **System blueprints not handled**: `_user`, `_team`, `_rule` etc. aren't in the validation set
5. **Error cascade**: `errgroup.WithContext` cancels all operations on first failure

## Design

### Blueprint Import: Three-Phase Approach

**Phase 1: Create non-system blueprints (stripped)**
- Skip all `_`-prefixed blueprints (system blueprints already exist)
- Strip dependent fields: `relations`, `mirrorProperties`, `calculationProperties`, `aggregationProperties`
- Create in topological order (dependencies first)
- Concurrency limit: 5 simultaneous

**Phase 2: Update non-system blueprints with stripped fields**
- Add back all stripped fields
- All targets now exist
- Concurrency limit: 5 simultaneous

**Phase 3: Update system blueprints**
- Update `_user`, `_team`, `_rule`, etc. with changes from export
- All non-system blueprints exist, so relations resolve

### Topological Sorting

Extract dependencies from:
- `relations[*].target` - direct relation targets
- `mirrorProperties[*].path` - format: `relationName.property`
- `calculationProperties[*].calculation` - may contain blueprint references
- `aggregationProperties[*].target` - aggregation targets

Algorithm:
1. Build directed graph: blueprint → blueprints it depends on
2. Apply Kahn's algorithm for topological sort
3. Handle cycles: break cycle, log warning, rely on retry mechanism
4. Group by dependency level for parallel execution within levels

Example ordering:
```
Level 0: snykVulnerability, action_run, response (no dependencies)
Level 1: group, snykTarget, question_template (depend on level 0)
Level 2: service, project (depend on level 1)
```

### Entity Import: Two-Phase Approach

**Phase 1: Create entities without relations**
- Strip relation values from entity properties
- Group entities by blueprint
- Order by blueprint dependency (leaf blueprints first)
- Concurrency limit: 20 simultaneous

**Phase 2: Update entities with relations**
- Add back relation values
- All target entities exist
- Concurrency limit: 20 simultaneous

### Concurrency Control

Replace unbounded goroutine spawning with worker pool pattern:

| Resource Type | Concurrency Limit |
|---------------|-------------------|
| Blueprints | 5 |
| Entities | 20 |
| Other (pages, actions, teams) | 10 |

Implementation:
- Buffered channel as semaphore
- Worker acquires before API call, releases after
- Use `sync.WaitGroup` instead of `errgroup.WithContext` to prevent cascade

### Error Handling

**Categories:**
- `DEPENDENCY_ERROR`: Missing blueprint/entity reference → queue for retry
- `AUTH_ERROR`: Authentication/permission issues → fail immediately
- `VALIDATION_ERROR`: Invalid data format → fail immediately
- `RATE_LIMIT`: Throttled by API → exponential backoff retry
- `NETWORK_ERROR`: Connection issues → exponential backoff retry

**Reporting:**
- Group errors by category and resource type
- Show count + first 5 examples (not all errors)
- Summary: "X blueprints failed due to Y"
- `--verbose` flag for full error list

**Progress streaming:**
- Real-time progress: "Importing blueprints: 45/89 (3 errors)"
- Don't wait until completion to show status

### System Blueprint Handling

All `_`-prefixed blueprints are system blueprints:
- `_user`, `_team`, `_rule`, etc.
- Assumed to exist in target org (don't create)
- Added to validation set for relation checking
- Updated in Phase 3 after all dependencies exist

## File Changes

| File | Changes |
|------|---------|
| `internal/modules/import_module/blueprint_deps.go` | Add topological sort, extract all dependency types, strip additional fields |
| `internal/modules/import_module/import.go` | Three-phase blueprint import, concurrency limits, improved error handling |
| `internal/modules/import_module/entity_import.go` | New: two-phase entity import with relation stripping |
| `internal/modules/import_module/errors.go` | New: error categorization and reporting |
| `internal/modules/import_module/worker_pool.go` | New: bounded concurrency worker pool |

## Testing Strategy

1. **Unit tests**
   - Topological sort with various dependency graphs (linear, diamond, cyclic)
   - Field stripping for all dependency types
   - Error categorization logic

2. **Integration tests**
   - Export from production org (89 blueprints, 9014 entities)
   - Import to fresh staging org
   - Verify all resources created successfully

3. **Edge cases**
   - Circular blueprint dependencies
   - Missing blueprint references (orphaned entities)
   - Rate limiting under load

## Rollout

1. Implement new logic as default
2. Add `--legacy-import` flag to use old behavior
3. Test with real data
4. Remove legacy flag after validation

## Success Criteria

- Import of 89 blueprints and 9014 entities completes without cascade failures
- All system blueprint updates succeed
- Clear error messages for any actual failures
- Progress visible during long imports

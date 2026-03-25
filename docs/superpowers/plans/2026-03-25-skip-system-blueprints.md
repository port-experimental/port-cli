# Skip System Blueprints + Skip-Entities Bug Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `--skip-system-blueprints` flag (shallow skip of `_*` blueprint schemas and entities) and fix `--skip-entities` to also gate users/teams, across all three commands: `export`, `import`, and `migrate`.

**Architecture:** Two independent changes composed across the same six files. The `--skip-entities` bug fix is additive guard clauses only. The `--skip-system-blueprints` feature uses the existing `ApplyBlueprintExclusions` two-list pattern: system blueprints land in `excludeSchema` so they stay in the iteration list (for scorecards/actions/permissions) but are removed from `dataBlueprints` (schema output), with an additional per-blueprint guard to skip entity fetching.

**Tech Stack:** Go, Cobra CLI, `errgroup`, `net/http/httptest` for tests

---

## File Map

| File | Change |
|------|--------|
| `internal/modules/export/collector.go` | Add `SkipSystemBlueprints` to `Options`; add `!opts.SkipEntities` guards for teams/users; add system-blueprint schema exclusion + entity skip |
| `internal/modules/export/collector_test.go` | New tests for skip-entities teams/users fix and skip-system-blueprints behavior |
| `internal/modules/import_module/import.go` | Add `SkipSystemBlueprints` to `Options`; add `!opts.SkipEntities` guards in `importOtherResources`; extend `applyDataExclusion` to handle system blueprints |
| `internal/modules/import_module/import_test.go` | New tests for skip-entities fix and applyDataExclusion system-blueprint behavior |
| `internal/modules/migrate/migrate.go` | Add `SkipSystemBlueprints` to `Options`; add `!opts.SkipEntities` guards in `exportFromSource()`; add system-blueprint entity skip in `exportFromSource()`; add `SkipSystemBlueprints` to `diffOpts` in `Execute()` |
| `internal/modules/migrate/migrate_test.go` | New tests for skip-entities fix and skip-system-blueprints in migrate |
| `internal/commands/export.go` | Add `--skip-system-blueprints` flag; add users/teams conflict check for `--skip-entities`; pass `SkipSystemBlueprints` to `export.Options` |
| `internal/commands/import.go` | Same flag + conflict check + pass-through to `import_module.Options` |
| `internal/commands/migrate.go` | Same flag + conflict check + pass-through to `migrate.Options` |

---

## Task 1: Fix `--skip-entities` bug in `collector.go` (teams and users)

**Files:**
- Modify: `internal/modules/export/collector.go:294-320`
- Test: `internal/modules/export/collector_test.go`

- [ ] **Step 1: Write the failing test**

Add to `collector_test.go`:

```go
func TestCollector_SkipEntities_SkipsTeamsAndUsers(t *testing.T) {
	teamsHit := false
	usersHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/access_token":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
		case "/blueprints":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "blueprints": []interface{}{}})
		case "/teams":
			teamsHit = true
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "teams": []interface{}{}})
		case "/users":
			usersHit = true
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "users": []interface{}{}})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}))
	defer server.Close()

	client := api.NewClient("id", "secret", server.URL, 0)
	collector := NewCollector(client)
	_, err := collector.Collect(context.Background(), Options{SkipEntities: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if teamsHit {
		t.Error("teams endpoint should not be called when SkipEntities=true")
	}
	if usersHit {
		t.Error("users endpoint should not be called when SkipEntities=true")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/modules/export/... -run TestCollector_SkipEntities_SkipsTeamsAndUsers -v
```

Expected: FAIL — teams and users endpoints ARE hit currently.

- [ ] **Step 3: Apply the fix in `collector.go`**

Change lines 294 and 309 from:

```go
if shouldCollect("teams", opts.IncludeResources) {
```
```go
if shouldCollect("users", opts.IncludeResources) {
```

To:

```go
if !opts.SkipEntities && shouldCollect("teams", opts.IncludeResources) {
```
```go
if !opts.SkipEntities && shouldCollect("users", opts.IncludeResources) {
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/modules/export/... -run TestCollector_SkipEntities_SkipsTeamsAndUsers -v
```

Expected: PASS

- [ ] **Step 5: Run full export package tests**

```bash
go test ./internal/modules/export/...
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/modules/export/collector.go internal/modules/export/collector_test.go
git commit -m "fix: --skip-entities now skips teams and users in collector"
```

---

## Task 2: Fix `--skip-entities` bug in `import.go` (teams and users)

**Files:**
- Modify: `internal/modules/import_module/import.go:862-869`
- Test: `internal/modules/import_module/import_test.go`

The `importOtherResources` method at line 839 has teams (condition at line 863) and users (condition at line 868) gated only by `shouldImport` — they need `!opts.SkipEntities` too.

Note: the test file is `package import_module` (not `_test`), so it has full access to unexported methods like `importOtherResources`.

- [ ] **Step 1: Write the failing test**

Add to `import_test.go`. The test uses `applyDataExclusion` directly (it's in the same package) and a mock Importer. Since `Importer.importOtherResources` is not exported, we test at a higher level via the tracking in a mock call counter. The simplest approach is to test `applyDataExclusion` indirectly by checking that teams/users on an `export.Data` don't get imported:

```go
func TestImportOtherResources_SkipEntities_SkipsTeamsAndUsers(t *testing.T) {
	teamsHit := false
	usersHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/auth/access_token":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
		case strings.HasPrefix(r.URL.Path, "/teams") && r.Method == http.MethodPost:
			teamsHit = true
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		case strings.HasPrefix(r.URL.Path, "/users") && r.Method == http.MethodPatch:
			usersHit = true
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}))
	defer server.Close()

	client := api.NewClient("id", "secret", server.URL, 0)
	importer := NewImporter(client)
	data := &export.Data{
		Teams: []api.Team{{"identifier": "t1", "name": "Team1"}},
		Users: []api.User{{"email": "u@example.com"}},
	}
	result := &Result{}
	opts := Options{SkipEntities: true}
	var mu sync.Mutex
	_ = mu
	if err := importer.importOtherResources(context.Background(), data, opts, result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if teamsHit {
		t.Error("teams import should not be called when SkipEntities=true")
	}
	if usersHit {
		t.Error("users import should not be called when SkipEntities=true")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/modules/import_module/... -run TestImportOtherResources_SkipEntities_SkipsTeamsAndUsers -v
```

Expected: FAIL — teams/users endpoints are called despite SkipEntities=true.

- [ ] **Step 3: Apply the fix in `import.go`**

Change lines 862 and 867 from:

```go
if shouldImport("teams", opts.IncludeResources) {
```
```go
if shouldImport("users", opts.IncludeResources) {
```

To:

```go
if !opts.SkipEntities && shouldImport("teams", opts.IncludeResources) {
```
```go
if !opts.SkipEntities && shouldImport("users", opts.IncludeResources) {
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/modules/import_module/... -run TestImportOtherResources_SkipEntities_SkipsTeamsAndUsers -v
```

Expected: PASS

- [ ] **Step 5: Run full import package tests**

```bash
go test ./internal/modules/import_module/...
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/modules/import_module/import.go internal/modules/import_module/import_test.go
git commit -m "fix: --skip-entities now skips teams and users in importer"
```

---

## Task 3: Fix `--skip-entities` bug in `migrate.go` (teams and users)

**Files:**
- Modify: `internal/modules/migrate/migrate.go:277-302`
- Test: `internal/modules/migrate/migrate_test.go`

`exportFromSource()` has teams (condition at line 277) and users (condition at line 291), gated only by `shouldCollect`.

- [ ] **Step 1: Write the failing test**

Add to `migrate_test.go`:

```go
func TestExportFromSource_SkipEntities_SkipsTeamsAndUsers(t *testing.T) {
	teamsHit := false
	usersHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/access_token":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
		case "/blueprints":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "blueprints": []interface{}{}})
		case "/teams":
			teamsHit = true
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "teams": []interface{}{}})
		case "/users":
			usersHit = true
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "users": []interface{}{}})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}))
	defer server.Close()

	m := &Module{
		sourceClient: api.NewClient("id", "secret", server.URL, 0),
		targetClient: api.NewClient("id", "secret", server.URL, 0),
	}
	opts := Options{SkipEntities: true}
	_, err := m.exportFromSource(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if teamsHit {
		t.Error("teams endpoint should not be called when SkipEntities=true")
	}
	if usersHit {
		t.Error("users endpoint should not be called when SkipEntities=true")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/modules/migrate/... -run TestExportFromSource_SkipEntities_SkipsTeamsAndUsers -v
```

Expected: FAIL

- [ ] **Step 3: Apply the fix in `migrate.go`**

Change lines 277 and 291 from:

```go
if shouldCollect("teams", opts.IncludeResources) {
```
```go
if shouldCollect("users", opts.IncludeResources) {
```

To:

```go
if !opts.SkipEntities && shouldCollect("teams", opts.IncludeResources) {
```
```go
if !opts.SkipEntities && shouldCollect("users", opts.IncludeResources) {
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/modules/migrate/... -run TestExportFromSource_SkipEntities_SkipsTeamsAndUsers -v
```

Expected: PASS

- [ ] **Step 5: Run full migrate package tests**

```bash
go test ./internal/modules/migrate/...
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/modules/migrate/migrate.go internal/modules/migrate/migrate_test.go
git commit -m "fix: --skip-entities now skips teams and users in migrate exportFromSource"
```

---

## Task 4: Add `SkipSystemBlueprints` field to all three `Options` structs

**Files:**
- Modify: `internal/modules/export/collector.go:14-22` (Options struct)
- Modify: `internal/modules/import_module/import.go:37-48` (Options struct)
- Modify: `internal/modules/migrate/migrate.go:31-38` (Options struct)

No tests needed yet — this is a pure struct field addition with no behavior.

- [ ] **Step 1: Add the field to `export.Options`**

In `collector.go`, change the `Options` struct:

```go
type Options struct {
	OutputPath             string
	Blueprints             []string
	Format                 string
	SkipEntities           bool
	SkipSystemBlueprints   bool // skip _* blueprint schemas and their entities
	IncludeResources       []string
	ExcludeBlueprints      []string // deep: exclude blueprint schema + all its resources
	ExcludeBlueprintSchema []string // shallow: exclude only the blueprint schema, keep resources
}
```

- [ ] **Step 2: Add the field to `import_module.Options`**

In `import.go`, change the `Options` struct:

```go
type Options struct {
	InputPath              string
	DryRun                 bool
	SkipEntities           bool
	SkipSystemBlueprints   bool // skip _* blueprint schemas and their entities
	IncludeResources       []string
	ExcludeBlueprints      []string // deep: exclude blueprint schema + all its resources
	ExcludeBlueprintSchema []string // shallow: exclude only the blueprint schema, keep resources
	Verbose                bool
	ShowPagesPipeline      bool
	ProgressCallback       ProgressCallback
	LogCallback            func(string)
}
```

- [ ] **Step 3: Add the field to `migrate.Options`**

In `migrate.go`, change the `Options` struct:

```go
type Options struct {
	Blueprints             []string
	DryRun                 bool
	SkipEntities           bool
	SkipSystemBlueprints   bool // skip _* blueprint schemas and their entities
	IncludeResources       []string
	ExcludeBlueprints      []string // deep: exclude blueprint schema + all its resources
	ExcludeBlueprintSchema []string // shallow: exclude only the blueprint schema, keep resources
}
```

- [ ] **Step 4: Verify it compiles**

```bash
go build ./...
```

Expected: no errors (zero-value for new bool field is false, backward compatible).

- [ ] **Step 5: Commit**

```bash
git add internal/modules/export/collector.go internal/modules/import_module/import.go internal/modules/migrate/migrate.go
git commit -m "feat: add SkipSystemBlueprints to Options structs (no behavior yet)"
```

---

## Task 5: Implement `--skip-system-blueprints` in `collector.go`

**Files:**
- Modify: `internal/modules/export/collector.go`
- Test: `internal/modules/export/collector_test.go`

Two changes in `Collect()`:
1. Before `ApplyBlueprintExclusions`, add `_*` blueprint IDs to `excludeSchema` so they stay in `iterBlueprints` (for scorecards/actions/permissions) but are removed from `dataBlueprints` (schema output).
2. In the entity-collection loop, skip entity fetch for `_*` blueprint IDs.

- [ ] **Step 1: Write failing tests**

Add to `collector_test.go`:

```go
func TestCollector_SkipSystemBlueprints_ExcludesSchemaAndEntities(t *testing.T) {
	entitiesHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/access_token":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
		case "/blueprints":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"blueprints": []map[string]interface{}{
					{"identifier": "_user", "title": "User"},
					{"identifier": "service", "title": "Service"},
				},
			})
		case "/blueprints/_user/entities":
			entitiesHit = true
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "entities": []interface{}{}})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}))
	defer server.Close()

	client := api.NewClient("id", "secret", server.URL, 0)
	collector := NewCollector(client)
	data, err := collector.Collect(context.Background(), Options{SkipSystemBlueprints: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// _user schema should NOT be in output blueprints
	for _, bp := range data.Blueprints {
		id, _ := bp["identifier"].(string)
		if id == "_user" {
			t.Error("_user blueprint schema should be excluded from output when SkipSystemBlueprints=true")
		}
	}

	// service blueprint should still be present
	found := false
	for _, bp := range data.Blueprints {
		if bp["identifier"] == "service" {
			found = true
		}
	}
	if !found {
		t.Error("non-system blueprint 'service' should remain in output")
	}

	// entities endpoint for _user should NOT be called
	if entitiesHit {
		t.Error("entities endpoint for _user should not be called when SkipSystemBlueprints=true")
	}
}

func TestCollector_SkipSystemBlueprints_StillCollectsScorecards(t *testing.T) {
	scorecardsHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/access_token":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
		case "/blueprints":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"blueprints": []map[string]interface{}{
					{"identifier": "_user", "title": "User"},
				},
			})
		case "/blueprints/_user/scorecards":
			scorecardsHit = true
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "scorecards": []interface{}{}})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}))
	defer server.Close()

	client := api.NewClient("id", "secret", server.URL, 0)
	collector := NewCollector(client)
	_, err := collector.Collect(context.Background(), Options{SkipSystemBlueprints: true, SkipEntities: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !scorecardsHit {
		t.Error("scorecards endpoint for _user should still be called when SkipSystemBlueprints=true (shallow skip)")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/modules/export/... -run "TestCollector_SkipSystemBlueprints" -v
```

Expected: FAIL — `_user` appears in output blueprints, entities endpoint IS called, scorecards not collected.

- [ ] **Step 3: Implement in `collector.go`**

There are **two** `ApplyBlueprintExclusions` calls in `Collect()`. Both need the pre-pass.

**First call (line 136 — when `shouldCollect("blueprints")` is true):**

Change:

```go
iterBlueprints, dataBlueprints := ApplyBlueprintExclusions(blueprints, opts.ExcludeBlueprints, opts.ExcludeBlueprintSchema)
data.Blueprints = dataBlueprints
blueprints = iterBlueprints
```

To:

```go
// Add system blueprints to schema-only exclusion so they stay in iterBlueprints
// (for scorecards/actions/permissions) but are removed from dataBlueprints (schema output).
excludeSchema := opts.ExcludeBlueprintSchema
if opts.SkipSystemBlueprints {
    for _, bp := range blueprints {
        id, _ := bp["identifier"].(string)
        if strings.HasPrefix(id, "_") {
            excludeSchema = append(excludeSchema, id)
        }
    }
}
iterBlueprints, dataBlueprints := ApplyBlueprintExclusions(blueprints, opts.ExcludeBlueprints, excludeSchema)
data.Blueprints = dataBlueprints
blueprints = iterBlueprints
```

**Second call (line 162 — the `else` branch when `!shouldCollect("blueprints")`):**

Change:

```go
// Discard dataList: blueprints are not written to output in this branch (shouldCollect("blueprints") is false)
iterBlueprints, _ := ApplyBlueprintExclusions(blueprints, opts.ExcludeBlueprints, opts.ExcludeBlueprintSchema)
blueprints = iterBlueprints
```

To:

```go
// Discard dataList: blueprints are not written to output in this branch (shouldCollect("blueprints") is false)
excludeSchema2 := opts.ExcludeBlueprintSchema
if opts.SkipSystemBlueprints {
    for _, bp := range blueprints {
        id, _ := bp["identifier"].(string)
        if strings.HasPrefix(id, "_") {
            excludeSchema2 = append(excludeSchema2, id)
        }
    }
}
iterBlueprints, _ := ApplyBlueprintExclusions(blueprints, opts.ExcludeBlueprints, excludeSchema2)
blueprints = iterBlueprints
```

Note: `strings` is already imported in `collector.go`.

Then in the entity-collection loop, change line 180 from:

```go
if !opts.SkipEntities && shouldCollect("entities", opts.IncludeResources) {
```

To:

```go
skipEntitiesForBP := opts.SkipEntities || (opts.SkipSystemBlueprints && strings.HasPrefix(bpID, "_"))
if !skipEntitiesForBP && shouldCollect("entities", opts.IncludeResources) {
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/modules/export/... -run "TestCollector_SkipSystemBlueprints" -v
```

Expected: all PASS

- [ ] **Step 5: Run full export package tests**

```bash
go test ./internal/modules/export/...
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/modules/export/collector.go internal/modules/export/collector_test.go
git commit -m "feat: --skip-system-blueprints skips _* schemas and entities in collector"
```

---

## Task 6: Implement `--skip-system-blueprints` in `import.go`

**Files:**
- Modify: `internal/modules/import_module/import.go` — `applyDataExclusion` function and its call site in `Execute()`
- Test: `internal/modules/import_module/import_test.go`

The approach: add a `skipSystemBlueprints bool` parameter to `applyDataExclusion`. When true, pre-filter `data.Blueprints` (remove `_*` schemas) and `data.Entities` (remove entities whose `blueprint` field starts with `_`). Scorecards, actions, and permissions are NOT removed.

- [ ] **Step 1: Write the failing test**

Add to `import_test.go`:

```go
func TestApplyDataExclusion_SkipSystemBlueprints(t *testing.T) {
	data := &export.Data{
		Blueprints: []api.Blueprint{
			{"identifier": "_user"},
			{"identifier": "_team"},
			{"identifier": "service"},
		},
		Entities: []api.Entity{
			{"identifier": "u1", "blueprint": "_user"},
			{"identifier": "s1", "blueprint": "service"},
		},
		Scorecards: []api.Scorecard{
			{"identifier": "sc1", "blueprintIdentifier": "_user"},
		},
		Actions: []api.Action{
			{"identifier": "a1", "blueprint": "_user"},
		},
		BlueprintPermissions: map[string]api.Permissions{
			"_user":   {"read": []string{"everyone"}},
			"service": {"read": []string{"everyone"}},
		},
	}

	applyDataExclusion(data, nil, nil, true)

	// _* blueprint schemas removed
	if len(data.Blueprints) != 1 {
		t.Errorf("expected 1 blueprint (service only), got %d", len(data.Blueprints))
	}
	if id, _ := data.Blueprints[0]["identifier"].(string); id != "service" {
		t.Errorf("expected remaining blueprint to be 'service', got %q", id)
	}

	// _* entities removed
	if len(data.Entities) != 1 {
		t.Errorf("expected 1 entity (s1 only), got %d", len(data.Entities))
	}

	// Scorecards for _user STILL present (shallow skip — scorecards pass through)
	if len(data.Scorecards) != 1 {
		t.Errorf("expected 1 scorecard (shallow skip keeps scorecards), got %d", len(data.Scorecards))
	}

	// Actions for _user STILL present
	if len(data.Actions) != 1 {
		t.Errorf("expected 1 action (shallow skip keeps actions), got %d", len(data.Actions))
	}

	// Blueprint permissions for _user STILL present
	if _, ok := data.BlueprintPermissions["_user"]; !ok {
		t.Error("blueprint permissions for _user should be kept (shallow skip)")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/modules/import_module/... -run TestApplyDataExclusion_SkipSystemBlueprints -v
```

Expected: compile error or FAIL — `applyDataExclusion` doesn't accept 4 args yet.

- [ ] **Step 3: Update `applyDataExclusion` signature and add system blueprint pre-pass**

Change the function signature at line 2320 from:

```go
func applyDataExclusion(data *export.Data, excludeDeep, excludeSchema []string) {
```

To:

```go
func applyDataExclusion(data *export.Data, excludeDeep, excludeSchema []string, skipSystemBlueprints bool) {
```

At the top of the function body, before the existing exclusion logic, add:

```go
// Pre-pass: remove system blueprint schemas and their entities (shallow skip).
// Scorecards, actions, and permissions are kept.
if skipSystemBlueprints {
    filteredBPs := data.Blueprints[:0:0]
    for _, bp := range data.Blueprints {
        id, _ := bp["identifier"].(string)
        if strings.HasPrefix(id, "_") {
            continue
        }
        filteredBPs = append(filteredBPs, bp)
    }
    data.Blueprints = filteredBPs

    filteredEnts := data.Entities[:0:0]
    for _, e := range data.Entities {
        bpID, _ := e["blueprint"].(string)
        if strings.HasPrefix(bpID, "_") {
            continue
        }
        filteredEnts = append(filteredEnts, e)
    }
    data.Entities = filteredEnts
}
```

Update the call site in `Execute()` at line 104 from:

```go
applyDataExclusion(data, opts.ExcludeBlueprints, opts.ExcludeBlueprintSchema)
```

To:

```go
applyDataExclusion(data, opts.ExcludeBlueprints, opts.ExcludeBlueprintSchema, opts.SkipSystemBlueprints)
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/modules/import_module/... -run TestApplyDataExclusion_SkipSystemBlueprints -v
```

Expected: PASS

- [ ] **Step 5: Run full import package tests**

```bash
go test ./internal/modules/import_module/...
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/modules/import_module/import.go internal/modules/import_module/import_test.go
git commit -m "feat: --skip-system-blueprints removes _* schemas and entities in importer"
```

---

## Task 7: Implement `--skip-system-blueprints` in `migrate.go`

**Files:**
- Modify: `internal/modules/migrate/migrate.go`
- Test: `internal/modules/migrate/migrate_test.go`

Two sub-changes:
1. In `exportFromSource()`: add the same `_*` → `excludeSchema` pre-pass before `ApplyBlueprintExclusions`, and per-blueprint entity skip.
2. In `Execute()`: add `SkipSystemBlueprints: opts.SkipSystemBlueprints` to `diffOpts`.

- [ ] **Step 1: Write failing tests**

Add to `migrate_test.go`:

```go
func TestExportFromSource_SkipSystemBlueprints_ExcludesSchemaAndEntities(t *testing.T) {
	entitiesHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/access_token":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
		case "/blueprints":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"blueprints": []map[string]interface{}{
					{"identifier": "_user"},
					{"identifier": "service"},
				},
			})
		case "/blueprints/_user/entities":
			entitiesHit = true
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "entities": []interface{}{}})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}))
	defer server.Close()

	m := &Module{
		sourceClient: api.NewClient("id", "secret", server.URL, 0),
		targetClient: api.NewClient("id", "secret", server.URL, 0),
	}
	opts := Options{SkipSystemBlueprints: true}
	data, err := m.exportFromSource(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, bp := range data.Blueprints {
		id, _ := bp["identifier"].(string)
		if id == "_user" {
			t.Error("_user blueprint schema should be excluded from migrate data")
		}
	}
	if entitiesHit {
		t.Error("entities endpoint for _user should not be called in migrate when SkipSystemBlueprints=true")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/modules/migrate/... -run TestExportFromSource_SkipSystemBlueprints -v
```

Expected: FAIL

- [ ] **Step 3: Implement in `exportFromSource()`**

In `exportFromSource()`, the current code at line 188 is:

```go
iterBlueprints, dataBlueprints := export.ApplyBlueprintExclusions(resolvedBlueprints, opts.ExcludeBlueprints, opts.ExcludeBlueprintSchema)
```

Change to:

```go
excludeSchema := opts.ExcludeBlueprintSchema
if opts.SkipSystemBlueprints {
    for _, bp := range resolvedBlueprints {
        id, _ := bp["identifier"].(string)
        if strings.HasPrefix(id, "_") {
            excludeSchema = append(excludeSchema, id)
        }
    }
}
iterBlueprints, dataBlueprints := export.ApplyBlueprintExclusions(resolvedBlueprints, opts.ExcludeBlueprints, excludeSchema)
```

In the entity-collection loop (line 215), change from:

```go
if !opts.SkipEntities && shouldCollect("entities", opts.IncludeResources) {
```

To:

```go
skipEntitiesForBP := opts.SkipEntities || (opts.SkipSystemBlueprints && strings.HasPrefix(bpID, "_"))
if !skipEntitiesForBP && shouldCollect("entities", opts.IncludeResources) {
```

- [ ] **Step 4: Add `SkipSystemBlueprints` to `diffOpts` in `Execute()`**

Change the `diffOpts` construction at lines 81-86 from:

```go
diffOpts := import_module.Options{
    SkipEntities:           opts.SkipEntities,
    IncludeResources:       opts.IncludeResources,
    ExcludeBlueprints:      opts.ExcludeBlueprints,
    ExcludeBlueprintSchema: opts.ExcludeBlueprintSchema,
}
```

To:

```go
diffOpts := import_module.Options{
    SkipEntities:           opts.SkipEntities,
    SkipSystemBlueprints:   opts.SkipSystemBlueprints,
    IncludeResources:       opts.IncludeResources,
    ExcludeBlueprints:      opts.ExcludeBlueprints,
    ExcludeBlueprintSchema: opts.ExcludeBlueprintSchema,
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/modules/migrate/... -run TestExportFromSource_SkipSystemBlueprints -v
```

Expected: PASS

- [ ] **Step 6: Run full migrate package tests**

```bash
go test ./internal/modules/migrate/...
```

Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add internal/modules/migrate/migrate.go internal/modules/migrate/migrate_test.go
git commit -m "feat: --skip-system-blueprints implemented in migrate exportFromSource and diffOpts"
```

---

## Task 8: Flag registration and conflict checks in command files

**Files:**
- Modify: `internal/commands/export.go`
- Modify: `internal/commands/import.go`
- Modify: `internal/commands/migrate.go`

Register `--skip-system-blueprints` flag in all three commands, pass it to the corresponding `Options` struct, and add users/teams conflict checks for `--skip-entities`.

- [ ] **Step 1: Update `internal/commands/export.go`**

**a) Declare the flag variable** alongside the other skip flags (near line 238 where `skipEntities` is declared):

```go
var skipSystemBlueprints bool
```

**b) Register the flag** in the flags section (after the `--skip-entities` flag at line 238):

```go
cmd.Flags().BoolVar(&skipSystemBlueprints, "skip-system-blueprints", false, "Skip system blueprint schemas (identifiers starting with _) and their entities")
```

**c) Add users/teams conflict check** in the `includeList` validation block (after the existing `entities` conflict check at lines 121-128):

```go
if skipEntities {
    for _, r := range includeList {
        if r == "users" {
            output.WarningPrintln("Warning: --skip-entities conflicts with --include users, ignoring --skip-entities for users")
            // Note: --include wins; users will be collected. SkipEntities remains true for entities/teams.
            // To fully honor --include users, pass skipEntities=false.
            // Simple approach: disable skipEntities entirely when any entity-type resource is explicitly included.
            skipEntities = false
            break
        }
        if r == "teams" {
            output.WarningPrintln("Warning: --skip-entities conflicts with --include teams, ignoring --skip-entities for teams")
            skipEntities = false
            break
        }
    }
}
```

**d) Pass `SkipSystemBlueprints` to `export.Options`** at lines 153-161:

```go
result, err := exportModule.Execute(cmd.Context(), export.Options{
    OutputPath:             outputPath,
    Blueprints:             blueprintList,
    ExcludeBlueprints:      excludeBlueprintList,
    ExcludeBlueprintSchema: excludeBlueprintSchemaList,
    Format:                 format,
    SkipEntities:           skipEntities,
    SkipSystemBlueprints:   skipSystemBlueprints,
    IncludeResources:       includeList,
})
```

- [ ] **Step 2: Update `internal/commands/import.go`**

Apply the same pattern: declare `skipSystemBlueprints bool`, register the flag, add users/teams conflict checks, pass `SkipSystemBlueprints: skipSystemBlueprints` to `import_module.Options`.

Look for where `SkipEntities: skipEntities` is set in the Options construction and add the new field alongside it.

- [ ] **Step 3: Update `internal/commands/migrate.go`**

Apply the same pattern: declare `skipSystemBlueprints bool`, register the flag, add users/teams conflict checks, pass `SkipSystemBlueprints: skipSystemBlueprints` to `migrate.Options`.

- [ ] **Step 4: Verify compilation**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 5: Smoke-test flag registration**

```bash
./bin/port export --help | grep skip-system
./bin/port import --help | grep skip-system
./bin/port migrate --help | grep skip-system
```

Expected: `--skip-system-blueprints` appears in help for all three.

- [ ] **Step 6: Run all tests**

```bash
make test
```

Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add internal/commands/export.go internal/commands/import.go internal/commands/migrate.go
git commit -m "feat: add --skip-system-blueprints flag and --skip-entities users/teams conflict check to commands"
```

---

## Final Verification

- [ ] **Run `make check`** (lint + test):

```bash
make check
```

Expected: no lint errors, all tests pass.

- [ ] **Build and verify flags**:

```bash
make build
./bin/port export --help | grep -E "skip-entities|skip-system"
./bin/port import --help | grep -E "skip-entities|skip-system"
./bin/port migrate --help | grep -E "skip-entities|skip-system"
```

Expected: both flags appear in all three commands.

---

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
|--------|---------|-----|------|--------|----------|
| CEO Review | `/plan-ceo-review` | Scope & strategy | 0 | — | — |
| Codex Review | `/codex review` | Independent 2nd opinion | 0 | — | — |
| Eng Review | `/plan-eng-review` | Architecture & tests (required) | 0 | — | — |
| Design Review | `/plan-design-review` | UI/UX gaps | 0 | — | — |

**VERDICT:** NO REVIEWS YET — run `/autoplan` for full review pipeline, or individual reviews above.

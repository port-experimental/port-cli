# Port Compare Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `port compare` command that compares two Port organizations and shows differences across all resource types.

**Architecture:** Fetcher loads data from live orgs or export files, Differ computes field-level differences with configurable exclusions, Formatters output results as text/JSON/HTML. Reuses existing `export.Data` structure and API client patterns.

**Tech Stack:** Go 1.21, Cobra CLI, html/template for HTML output

---

## Task 1: Create Compare Module Structure

**Files:**
- Create: `internal/modules/compare/compare.go`
- Create: `internal/modules/compare/types.go`

**Step 1: Create types.go with core data structures**

```go
// internal/modules/compare/types.go
package compare

import (
	"github.com/port-experimental/port-cli/internal/modules/export"
)

// Options represents compare options.
type Options struct {
	SourceOrg        string   // Source org name from config
	TargetOrg        string   // Target org name from config
	SourceFile       string   // Source export file path (alternative to org)
	TargetFile       string   // Target export file path (alternative to org)
	SourceClientID   string   // Override source client ID
	SourceSecret     string   // Override source client secret
	TargetClientID   string   // Override target client ID
	TargetSecret     string   // Override target client secret
	OutputFormat     string   // text, json, html
	HTMLFile         string   // Output path for HTML report
	HTMLSimple       bool     // Use simple HTML template
	Verbose          bool     // Show identifiers
	Full             bool     // Show full diff
	IncludeResources []string // Filter resource types
	FailOnDiff       bool     // Exit 1 if differences found
}

// DiffSummary represents the summary of differences for a resource type.
type DiffSummary struct {
	Added    int
	Modified int
	Removed  int
}

// ResourceDiff represents differences for a single resource type.
type ResourceDiff struct {
	Summary  DiffSummary
	Added    []ResourceChange
	Modified []ResourceChange
	Removed  []ResourceChange
}

// ResourceChange represents a single changed resource.
type ResourceChange struct {
	Identifier  string
	SourceData  map[string]interface{}
	TargetData  map[string]interface{}
	FieldDiffs  []FieldDiff
}

// FieldDiff represents a single field-level difference.
type FieldDiff struct {
	Path        string
	SourceValue interface{}
	TargetValue interface{}
}

// CompareResult represents the full comparison result.
type CompareResult struct {
	Source      string
	Target      string
	Timestamp   string
	Identical   bool
	Blueprints  ResourceDiff
	Actions     ResourceDiff
	Scorecards  ResourceDiff
	Pages       ResourceDiff
	Integrations ResourceDiff
	Teams       ResourceDiff
	Users       ResourceDiff
	Automations ResourceDiff
}

// OrgData wraps export.Data for comparison.
type OrgData struct {
	Name string
	Data *export.Data
}
```

**Step 2: Create compare.go with module shell**

```go
// internal/modules/compare/compare.go
package compare

import (
	"context"
	"fmt"
	"time"

	"github.com/port-experimental/port-cli/internal/config"
)

// Module handles organization comparison operations.
type Module struct {
	configManager *config.ConfigManager
}

// NewModule creates a new compare module.
func NewModule(configManager *config.ConfigManager) *Module {
	return &Module{
		configManager: configManager,
	}
}

// Execute runs the comparison and returns results.
func (m *Module) Execute(ctx context.Context, opts Options) (*CompareResult, error) {
	// TODO: Implement in subsequent tasks
	return &CompareResult{
		Source:    opts.SourceOrg,
		Target:    opts.TargetOrg,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Identical: true,
	}, nil
}
```

**Step 3: Verify compilation**

Run: `go build ./...`
Expected: Success, no errors

**Step 4: Commit**

```bash
git add internal/modules/compare/
git commit -m "feat(compare): add module structure and types"
```

---

## Task 2: Implement Fetcher for Live Organizations

**Files:**
- Create: `internal/modules/compare/fetcher.go`
- Create: `internal/modules/compare/fetcher_test.go`

**Step 1: Write failing test for fetcher**

```go
// internal/modules/compare/fetcher_test.go
package compare

import (
	"testing"
)

func TestDetectInputType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType string
	}{
		{"org name", "production", "org"},
		{"tar.gz file", "/path/to/export.tar.gz", "file"},
		{"json file", "./backup.json", "file"},
		{"relative tar", "backup.tar.gz", "file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectInputType(tt.input)
			if got != tt.wantType {
				t.Errorf("detectInputType(%q) = %q, want %q", tt.input, got, tt.wantType)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/modules/compare/... -run TestDetectInputType -v`
Expected: FAIL with "undefined: detectInputType"

**Step 3: Implement fetcher**

```go
// internal/modules/compare/fetcher.go
package compare

import (
	"context"
	"fmt"
	"strings"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/config"
	"github.com/port-experimental/port-cli/internal/modules/export"
	"github.com/port-experimental/port-cli/internal/modules/import_module"
)

// detectInputType determines if input is an org name or file path.
func detectInputType(input string) string {
	if strings.HasSuffix(input, ".tar.gz") ||
		strings.HasSuffix(input, ".json") ||
		strings.HasPrefix(input, "/") ||
		strings.HasPrefix(input, "./") ||
		strings.HasPrefix(input, "../") {
		return "file"
	}
	return "org"
}

// Fetcher loads organization data from live orgs or export files.
type Fetcher struct {
	configManager *config.ConfigManager
}

// NewFetcher creates a new fetcher.
func NewFetcher(configManager *config.ConfigManager) *Fetcher {
	return &Fetcher{
		configManager: configManager,
	}
}

// FetchOptions contains options for fetching org data.
type FetchOptions struct {
	OrgName      string
	FilePath     string
	ClientID     string
	ClientSecret string
	APIUrl       string
}

// Fetch loads organization data from either a live org or export file.
func (f *Fetcher) Fetch(ctx context.Context, opts FetchOptions) (*OrgData, error) {
	var input string
	if opts.FilePath != "" {
		input = opts.FilePath
	} else {
		input = opts.OrgName
	}

	inputType := detectInputType(input)

	if inputType == "file" {
		return f.fetchFromFile(ctx, input)
	}
	return f.fetchFromOrg(ctx, opts)
}

// fetchFromFile loads data from an export file.
func (f *Fetcher) fetchFromFile(ctx context.Context, filePath string) (*OrgData, error) {
	loader := import_module.NewLoader()
	data, err := loader.Load(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load export file %s: %w", filePath, err)
	}

	return &OrgData{
		Name: filePath,
		Data: data,
	}, nil
}

// fetchFromOrg loads data from a live Port organization.
func (f *Fetcher) fetchFromOrg(ctx context.Context, opts FetchOptions) (*OrgData, error) {
	// Load org config
	_, orgConfig, _, err := f.configManager.LoadWithDualOverrides(
		opts.ClientID,
		opts.ClientSecret,
		opts.APIUrl,
		opts.OrgName,
		"", "", "", "",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load config for org %s: %w", opts.OrgName, err)
	}

	if orgConfig == nil {
		return nil, fmt.Errorf("organization %s not found in config", opts.OrgName)
	}

	// Create API client
	client := api.NewClient(orgConfig.ClientID, orgConfig.ClientSecret, orgConfig.APIURL)
	defer client.Close()

	// Use export collector to fetch all data
	collector := export.NewCollector(client)
	data, err := collector.Collect(ctx, export.Options{
		SkipEntities:     true, // Don't compare entities by default
		IncludeResources: nil,  // Fetch all resource types
	})
	if err != nil {
		return nil, fmt.Errorf("failed to collect data from org %s: %w", opts.OrgName, err)
	}

	return &OrgData{
		Name: opts.OrgName,
		Data: data,
	}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/modules/compare/... -run TestDetectInputType -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/modules/compare/fetcher.go internal/modules/compare/fetcher_test.go
git commit -m "feat(compare): add fetcher for orgs and export files"
```

---

## Task 3: Implement Differ Core Logic

**Files:**
- Create: `internal/modules/compare/differ.go`
- Create: `internal/modules/compare/differ_test.go`

**Step 1: Write failing test for differ**

```go
// internal/modules/compare/differ_test.go
package compare

import (
	"testing"
)

func TestDiffResources_Added(t *testing.T) {
	source := []map[string]interface{}{}
	target := []map[string]interface{}{
		{"identifier": "new-bp", "title": "New Blueprint"},
	}

	diff := diffResources(source, target, "identifier")

	if diff.Summary.Added != 1 {
		t.Errorf("expected 1 added, got %d", diff.Summary.Added)
	}
	if len(diff.Added) != 1 {
		t.Errorf("expected 1 added item, got %d", len(diff.Added))
	}
	if diff.Added[0].Identifier != "new-bp" {
		t.Errorf("expected identifier 'new-bp', got %s", diff.Added[0].Identifier)
	}
}

func TestDiffResources_Removed(t *testing.T) {
	source := []map[string]interface{}{
		{"identifier": "old-bp", "title": "Old Blueprint"},
	}
	target := []map[string]interface{}{}

	diff := diffResources(source, target, "identifier")

	if diff.Summary.Removed != 1 {
		t.Errorf("expected 1 removed, got %d", diff.Summary.Removed)
	}
}

func TestDiffResources_Modified(t *testing.T) {
	source := []map[string]interface{}{
		{"identifier": "bp1", "title": "Original Title"},
	}
	target := []map[string]interface{}{
		{"identifier": "bp1", "title": "Updated Title"},
	}

	diff := diffResources(source, target, "identifier")

	if diff.Summary.Modified != 1 {
		t.Errorf("expected 1 modified, got %d", diff.Summary.Modified)
	}
	if len(diff.Modified[0].FieldDiffs) == 0 {
		t.Error("expected field diffs for modified resource")
	}
}

func TestDiffResources_Identical(t *testing.T) {
	source := []map[string]interface{}{
		{"identifier": "bp1", "title": "Same Title"},
	}
	target := []map[string]interface{}{
		{"identifier": "bp1", "title": "Same Title"},
	}

	diff := diffResources(source, target, "identifier")

	if diff.Summary.Added != 0 || diff.Summary.Modified != 0 || diff.Summary.Removed != 0 {
		t.Errorf("expected no differences, got added=%d modified=%d removed=%d",
			diff.Summary.Added, diff.Summary.Modified, diff.Summary.Removed)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/modules/compare/... -run TestDiffResources -v`
Expected: FAIL with "undefined: diffResources"

**Step 3: Implement differ**

```go
// internal/modules/compare/differ.go
package compare

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/modules/export"
)

// ExcludedFields contains fields to exclude from comparison.
var ExcludedFields = map[string]bool{
	"_id":       true,
	"id":        true,
	"orgId":     true,
	"createdAt": true,
	"createdBy": true,
	"updatedAt": true,
	"updatedBy": true,
	"icon":      true,
	"color":     true,
}

// Differ computes differences between two organizations.
type Differ struct {
	excludedFields map[string]bool
}

// NewDiffer creates a new differ with default excluded fields.
func NewDiffer() *Differ {
	return &Differ{
		excludedFields: ExcludedFields,
	}
}

// Diff compares source and target org data.
func (d *Differ) Diff(source, target *export.Data) *CompareResult {
	result := &CompareResult{
		Identical: true,
	}

	// Compare each resource type
	result.Blueprints = d.diffBlueprints(source.Blueprints, target.Blueprints)
	result.Actions = d.diffActions(source.Actions, target.Actions)
	result.Scorecards = d.diffScorecards(source.Scorecards, target.Scorecards)
	result.Pages = d.diffPages(source.Pages, target.Pages)
	result.Integrations = d.diffIntegrations(source.Integrations, target.Integrations)
	result.Teams = d.diffTeams(source.Teams, target.Teams)
	result.Users = d.diffUsers(source.Users, target.Users)

	// Check if any differences exist
	result.Identical = d.isIdentical(result)

	return result
}

func (d *Differ) isIdentical(r *CompareResult) bool {
	checks := []DiffSummary{
		r.Blueprints.Summary, r.Actions.Summary, r.Scorecards.Summary,
		r.Pages.Summary, r.Integrations.Summary, r.Teams.Summary, r.Users.Summary,
	}
	for _, s := range checks {
		if s.Added > 0 || s.Modified > 0 || s.Removed > 0 {
			return false
		}
	}
	return true
}

func (d *Differ) diffBlueprints(source, target []api.Blueprint) ResourceDiff {
	return diffResources(toMaps(source), toMaps(target), "identifier")
}

func (d *Differ) diffActions(source, target []api.Action) ResourceDiff {
	return diffResources(toMaps(source), toMaps(target), "identifier")
}

func (d *Differ) diffScorecards(source, target []api.Scorecard) ResourceDiff {
	return diffResources(toMaps(source), toMaps(target), "identifier")
}

func (d *Differ) diffPages(source, target []api.Page) ResourceDiff {
	return diffResources(toMaps(source), toMaps(target), "identifier")
}

func (d *Differ) diffIntegrations(source, target []api.Integration) ResourceDiff {
	return diffResources(toMaps(source), toMaps(target), "installationId")
}

func (d *Differ) diffTeams(source, target []api.Team) ResourceDiff {
	return diffResources(toMaps(source), toMaps(target), "name")
}

func (d *Differ) diffUsers(source, target []api.User) ResourceDiff {
	return diffResources(toMaps(source), toMaps(target), "email")
}

// toMaps converts a slice of typed maps to []map[string]interface{}.
func toMaps[T ~map[string]interface{}](items []T) []map[string]interface{} {
	result := make([]map[string]interface{}, len(items))
	for i, item := range items {
		result[i] = map[string]interface{}(item)
	}
	return result
}

// diffResources compares two slices of resources by identifier.
func diffResources(source, target []map[string]interface{}, idField string) ResourceDiff {
	result := ResourceDiff{}

	// Build lookup maps
	sourceMap := make(map[string]map[string]interface{})
	targetMap := make(map[string]map[string]interface{})

	for _, item := range source {
		if id, ok := item[idField].(string); ok {
			sourceMap[id] = item
		}
	}
	for _, item := range target {
		if id, ok := item[idField].(string); ok {
			targetMap[id] = item
		}
	}

	// Find added (in target but not in source)
	for id, targetItem := range targetMap {
		if _, exists := sourceMap[id]; !exists {
			result.Added = append(result.Added, ResourceChange{
				Identifier: id,
				TargetData: targetItem,
			})
		}
	}

	// Find removed (in source but not in target)
	for id, sourceItem := range sourceMap {
		if _, exists := targetMap[id]; !exists {
			result.Removed = append(result.Removed, ResourceChange{
				Identifier: id,
				SourceData: sourceItem,
			})
		}
	}

	// Find modified (in both, but different)
	for id, sourceItem := range sourceMap {
		if targetItem, exists := targetMap[id]; exists {
			fieldDiffs := diffFields(sourceItem, targetItem, "")
			if len(fieldDiffs) > 0 {
				result.Modified = append(result.Modified, ResourceChange{
					Identifier: id,
					SourceData: sourceItem,
					TargetData: targetItem,
					FieldDiffs: fieldDiffs,
				})
			}
		}
	}

	// Sort for consistent output
	sort.Slice(result.Added, func(i, j int) bool { return result.Added[i].Identifier < result.Added[j].Identifier })
	sort.Slice(result.Removed, func(i, j int) bool { return result.Removed[i].Identifier < result.Removed[j].Identifier })
	sort.Slice(result.Modified, func(i, j int) bool { return result.Modified[i].Identifier < result.Modified[j].Identifier })

	result.Summary = DiffSummary{
		Added:    len(result.Added),
		Modified: len(result.Modified),
		Removed:  len(result.Removed),
	}

	return result
}

// diffFields recursively compares two maps and returns field differences.
func diffFields(source, target map[string]interface{}, prefix string) []FieldDiff {
	var diffs []FieldDiff

	// Collect all keys from both maps
	allKeys := make(map[string]bool)
	for k := range source {
		allKeys[k] = true
	}
	for k := range target {
		allKeys[k] = true
	}

	for key := range allKeys {
		// Skip excluded fields
		if ExcludedFields[key] {
			continue
		}

		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		sourceVal, sourceExists := source[key]
		targetVal, targetExists := target[key]

		if !sourceExists {
			diffs = append(diffs, FieldDiff{
				Path:        path,
				SourceValue: nil,
				TargetValue: targetVal,
			})
		} else if !targetExists {
			diffs = append(diffs, FieldDiff{
				Path:        path,
				SourceValue: sourceVal,
				TargetValue: nil,
			})
		} else if !reflect.DeepEqual(sourceVal, targetVal) {
			// Check if both are maps for recursive comparison
			sourceMap, sourceIsMap := sourceVal.(map[string]interface{})
			targetMap, targetIsMap := targetVal.(map[string]interface{})

			if sourceIsMap && targetIsMap {
				diffs = append(diffs, diffFields(sourceMap, targetMap, path)...)
			} else {
				diffs = append(diffs, FieldDiff{
					Path:        path,
					SourceValue: sourceVal,
					TargetValue: targetVal,
				})
			}
		}
	}

	// Sort for consistent output
	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Path < diffs[j].Path
	})

	return diffs
}

// FormatPath formats a diff path for display.
func FormatPath(path string) string {
	return strings.TrimPrefix(path, ".")
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/modules/compare/... -run TestDiffResources -v`
Expected: All 4 tests PASS

**Step 5: Commit**

```bash
git add internal/modules/compare/differ.go internal/modules/compare/differ_test.go
git commit -m "feat(compare): add differ with field-level comparison"
```

---

## Task 4: Implement Text Output Formatter

**Files:**
- Create: `internal/modules/compare/output_text.go`
- Create: `internal/modules/compare/output_text_test.go`

**Step 1: Write failing test**

```go
// internal/modules/compare/output_text_test.go
package compare

import (
	"bytes"
	"strings"
	"testing"
)

func TestTextFormatter_Summary(t *testing.T) {
	result := &CompareResult{
		Source: "staging",
		Target: "production",
		Blueprints: ResourceDiff{
			Summary: DiffSummary{Added: 2, Modified: 1, Removed: 0},
		},
		Actions: ResourceDiff{
			Summary: DiffSummary{Added: 0, Modified: 0, Removed: 0},
		},
	}

	var buf bytes.Buffer
	formatter := NewTextFormatter(&buf, false, false)
	err := formatter.Format(result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "staging") {
		t.Error("expected output to contain source name")
	}
	if !strings.Contains(output, "2 added") {
		t.Error("expected output to contain '2 added'")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/modules/compare/... -run TestTextFormatter -v`
Expected: FAIL with "undefined: NewTextFormatter"

**Step 3: Implement text formatter**

```go
// internal/modules/compare/output_text.go
package compare

import (
	"fmt"
	"io"
	"strings"
)

// TextFormatter formats comparison results as colored text.
type TextFormatter struct {
	w       io.Writer
	verbose bool
	full    bool
}

// NewTextFormatter creates a new text formatter.
func NewTextFormatter(w io.Writer, verbose, full bool) *TextFormatter {
	return &TextFormatter{
		w:       w,
		verbose: verbose,
		full:    full,
	}
}

// Format outputs the comparison result as text.
func (f *TextFormatter) Format(result *CompareResult) error {
	// Header
	fmt.Fprintf(f.w, "Comparing %s -> %s\n\n", result.Source, result.Target)

	// Summary for each resource type
	f.formatResourceType("Blueprints", result.Blueprints)
	f.formatResourceType("Actions", result.Actions)
	f.formatResourceType("Scorecards", result.Scorecards)
	f.formatResourceType("Pages", result.Pages)
	f.formatResourceType("Integrations", result.Integrations)
	f.formatResourceType("Teams", result.Teams)
	f.formatResourceType("Users", result.Users)
	f.formatResourceType("Automations", result.Automations)

	// Total
	total := f.calculateTotal(result)
	fmt.Fprintf(f.w, "\nTotal: %d added, %d modified, %d removed\n",
		total.Added, total.Modified, total.Removed)

	if result.Identical {
		fmt.Fprintf(f.w, "\nOrganizations are identical.\n")
	}

	return nil
}

func (f *TextFormatter) formatResourceType(name string, diff ResourceDiff) {
	s := diff.Summary

	if s.Added == 0 && s.Modified == 0 && s.Removed == 0 {
		fmt.Fprintf(f.w, "%-14s identical\n", name+":")
		return
	}

	fmt.Fprintf(f.w, "%-14s %d added, %d modified, %d removed\n",
		name+":", s.Added, s.Modified, s.Removed)

	// Verbose: show identifiers
	if f.verbose || f.full {
		if len(diff.Added) > 0 {
			ids := f.getIdentifiers(diff.Added)
			fmt.Fprintf(f.w, "  Added:    %s\n", strings.Join(ids, ", "))
		}
		if len(diff.Modified) > 0 {
			ids := f.getIdentifiers(diff.Modified)
			fmt.Fprintf(f.w, "  Modified: %s\n", strings.Join(ids, ", "))
		}
		if len(diff.Removed) > 0 {
			ids := f.getIdentifiers(diff.Removed)
			fmt.Fprintf(f.w, "  Removed:  %s\n", strings.Join(ids, ", "))
		}
	}

	// Full: show field-level diffs
	if f.full {
		for _, change := range diff.Added {
			fmt.Fprintf(f.w, "\n  [+] %s (added)\n", change.Identifier)
			f.formatData(change.TargetData, "      ")
		}
		for _, change := range diff.Modified {
			fmt.Fprintf(f.w, "\n  [~] %s (modified)\n", change.Identifier)
			for _, fd := range change.FieldDiffs {
				fmt.Fprintf(f.w, "      %s:\n", fd.Path)
				fmt.Fprintf(f.w, "        - %v\n", fd.SourceValue)
				fmt.Fprintf(f.w, "        + %v\n", fd.TargetValue)
			}
		}
		for _, change := range diff.Removed {
			fmt.Fprintf(f.w, "\n  [-] %s (removed)\n", change.Identifier)
		}
	}
}

func (f *TextFormatter) getIdentifiers(changes []ResourceChange) []string {
	ids := make([]string, len(changes))
	for i, c := range changes {
		ids[i] = c.Identifier
	}
	return ids
}

func (f *TextFormatter) formatData(data map[string]interface{}, indent string) {
	for k, v := range data {
		if ExcludedFields[k] {
			continue
		}
		fmt.Fprintf(f.w, "%s%s: %v\n", indent, k, v)
	}
}

func (f *TextFormatter) calculateTotal(result *CompareResult) DiffSummary {
	resources := []ResourceDiff{
		result.Blueprints, result.Actions, result.Scorecards,
		result.Pages, result.Integrations, result.Teams,
		result.Users, result.Automations,
	}

	var total DiffSummary
	for _, r := range resources {
		total.Added += r.Summary.Added
		total.Modified += r.Summary.Modified
		total.Removed += r.Summary.Removed
	}
	return total
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/modules/compare/... -run TestTextFormatter -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/modules/compare/output_text.go internal/modules/compare/output_text_test.go
git commit -m "feat(compare): add text output formatter with verbosity levels"
```

---

## Task 5: Implement JSON Output Formatter

**Files:**
- Create: `internal/modules/compare/output_json.go`
- Create: `internal/modules/compare/output_json_test.go`

**Step 1: Write failing test**

```go
// internal/modules/compare/output_json_test.go
package compare

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestJSONFormatter(t *testing.T) {
	result := &CompareResult{
		Source:    "staging",
		Target:    "production",
		Timestamp: "2026-02-05T19:30:00Z",
		Identical: false,
		Blueprints: ResourceDiff{
			Summary: DiffSummary{Added: 1, Modified: 0, Removed: 0},
			Added: []ResourceChange{
				{Identifier: "new-bp"},
			},
		},
	}

	var buf bytes.Buffer
	formatter := NewJSONFormatter(&buf)
	err := formatter.Format(result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if parsed["source"] != "staging" {
		t.Errorf("expected source 'staging', got %v", parsed["source"])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/modules/compare/... -run TestJSONFormatter -v`
Expected: FAIL with "undefined: NewJSONFormatter"

**Step 3: Implement JSON formatter**

```go
// internal/modules/compare/output_json.go
package compare

import (
	"encoding/json"
	"io"
)

// JSONOutput represents the JSON output structure.
type JSONOutput struct {
	Source    string            `json:"source"`
	Target    string            `json:"target"`
	Timestamp string            `json:"timestamp"`
	Summary   JSONSummary       `json:"summary"`
	Identical bool              `json:"identical"`
	Diffs     map[string]JSONResourceDiff `json:"differences"`
}

// JSONSummary represents the overall summary.
type JSONSummary struct {
	TotalAdded    int `json:"total_added"`
	TotalModified int `json:"total_modified"`
	TotalRemoved  int `json:"total_removed"`
}

// JSONResourceDiff represents differences for a resource type in JSON.
type JSONResourceDiff struct {
	Added    []JSONResourceChange `json:"added,omitempty"`
	Modified []JSONResourceChange `json:"modified,omitempty"`
	Removed  []JSONResourceChange `json:"removed,omitempty"`
}

// JSONResourceChange represents a single changed resource in JSON.
type JSONResourceChange struct {
	Identifier  string                 `json:"identifier"`
	Data        map[string]interface{} `json:"data,omitempty"`
	SourceData  map[string]interface{} `json:"source,omitempty"`
	TargetData  map[string]interface{} `json:"target,omitempty"`
	Changes     []JSONFieldDiff        `json:"changes,omitempty"`
}

// JSONFieldDiff represents a field-level diff in JSON.
type JSONFieldDiff struct {
	Path        string      `json:"path"`
	SourceValue interface{} `json:"source_value"`
	TargetValue interface{} `json:"target_value"`
}

// JSONFormatter formats comparison results as JSON.
type JSONFormatter struct {
	w io.Writer
}

// NewJSONFormatter creates a new JSON formatter.
func NewJSONFormatter(w io.Writer) *JSONFormatter {
	return &JSONFormatter{w: w}
}

// Format outputs the comparison result as JSON.
func (f *JSONFormatter) Format(result *CompareResult) error {
	output := JSONOutput{
		Source:    result.Source,
		Target:    result.Target,
		Timestamp: result.Timestamp,
		Identical: result.Identical,
		Summary: JSONSummary{
			TotalAdded:    f.sumAdded(result),
			TotalModified: f.sumModified(result),
			TotalRemoved:  f.sumRemoved(result),
		},
		Diffs: make(map[string]JSONResourceDiff),
	}

	// Add each resource type
	f.addResourceDiff(output.Diffs, "blueprints", result.Blueprints)
	f.addResourceDiff(output.Diffs, "actions", result.Actions)
	f.addResourceDiff(output.Diffs, "scorecards", result.Scorecards)
	f.addResourceDiff(output.Diffs, "pages", result.Pages)
	f.addResourceDiff(output.Diffs, "integrations", result.Integrations)
	f.addResourceDiff(output.Diffs, "teams", result.Teams)
	f.addResourceDiff(output.Diffs, "users", result.Users)
	f.addResourceDiff(output.Diffs, "automations", result.Automations)

	encoder := json.NewEncoder(f.w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func (f *JSONFormatter) addResourceDiff(diffs map[string]JSONResourceDiff, name string, rd ResourceDiff) {
	if rd.Summary.Added == 0 && rd.Summary.Modified == 0 && rd.Summary.Removed == 0 {
		return
	}

	jrd := JSONResourceDiff{}

	for _, change := range rd.Added {
		jrd.Added = append(jrd.Added, JSONResourceChange{
			Identifier: change.Identifier,
			Data:       change.TargetData,
		})
	}

	for _, change := range rd.Modified {
		jc := JSONResourceChange{
			Identifier: change.Identifier,
			SourceData: change.SourceData,
			TargetData: change.TargetData,
		}
		for _, fd := range change.FieldDiffs {
			jc.Changes = append(jc.Changes, JSONFieldDiff{
				Path:        fd.Path,
				SourceValue: fd.SourceValue,
				TargetValue: fd.TargetValue,
			})
		}
		jrd.Modified = append(jrd.Modified, jc)
	}

	for _, change := range rd.Removed {
		jrd.Removed = append(jrd.Removed, JSONResourceChange{
			Identifier: change.Identifier,
			Data:       change.SourceData,
		})
	}

	diffs[name] = jrd
}

func (f *JSONFormatter) sumAdded(r *CompareResult) int {
	return r.Blueprints.Summary.Added + r.Actions.Summary.Added +
		r.Scorecards.Summary.Added + r.Pages.Summary.Added +
		r.Integrations.Summary.Added + r.Teams.Summary.Added +
		r.Users.Summary.Added + r.Automations.Summary.Added
}

func (f *JSONFormatter) sumModified(r *CompareResult) int {
	return r.Blueprints.Summary.Modified + r.Actions.Summary.Modified +
		r.Scorecards.Summary.Modified + r.Pages.Summary.Modified +
		r.Integrations.Summary.Modified + r.Teams.Summary.Modified +
		r.Users.Summary.Modified + r.Automations.Summary.Modified
}

func (f *JSONFormatter) sumRemoved(r *CompareResult) int {
	return r.Blueprints.Summary.Removed + r.Actions.Summary.Removed +
		r.Scorecards.Summary.Removed + r.Pages.Summary.Removed +
		r.Integrations.Summary.Removed + r.Teams.Summary.Removed +
		r.Users.Summary.Removed + r.Automations.Summary.Removed
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/modules/compare/... -run TestJSONFormatter -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/modules/compare/output_json.go internal/modules/compare/output_json_test.go
git commit -m "feat(compare): add JSON output formatter"
```

---

## Task 6: Implement HTML Output Formatter

**Files:**
- Create: `internal/modules/compare/output_html.go`
- Create: `internal/modules/compare/templates/report.html`
- Create: `internal/modules/compare/templates/simple.html`

**Step 1: Create HTML templates**

```html
<!-- internal/modules/compare/templates/report.html -->
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Port Organization Comparison</title>
    <style>
        :root {
            --green: #22c55e;
            --red: #ef4444;
            --yellow: #eab308;
            --gray: #6b7280;
            --bg: #f9fafb;
            --card-bg: #ffffff;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: var(--bg); padding: 2rem; }
        .container { max-width: 1200px; margin: 0 auto; }
        h1 { color: #1f2937; margin-bottom: 0.5rem; }
        .subtitle { color: var(--gray); margin-bottom: 2rem; }
        .summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1rem; margin-bottom: 2rem; }
        .stat-card { background: var(--card-bg); padding: 1.5rem; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
        .stat-card h3 { font-size: 0.875rem; color: var(--gray); margin-bottom: 0.5rem; }
        .stat-card .value { font-size: 2rem; font-weight: bold; }
        .stat-card .value.added { color: var(--green); }
        .stat-card .value.modified { color: var(--yellow); }
        .stat-card .value.removed { color: var(--red); }
        .section { background: var(--card-bg); border-radius: 8px; margin-bottom: 1rem; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
        .section-header { padding: 1rem 1.5rem; border-bottom: 1px solid #e5e7eb; cursor: pointer; display: flex; justify-content: space-between; align-items: center; }
        .section-header:hover { background: #f3f4f6; }
        .section-title { font-weight: 600; }
        .section-stats { display: flex; gap: 1rem; font-size: 0.875rem; }
        .section-stats span { padding: 0.25rem 0.5rem; border-radius: 4px; }
        .section-stats .added { background: #dcfce7; color: #166534; }
        .section-stats .modified { background: #fef9c3; color: #854d0e; }
        .section-stats .removed { background: #fee2e2; color: #991b1b; }
        .section-content { padding: 1.5rem; display: none; }
        .section.open .section-content { display: block; }
        .change-item { padding: 0.75rem; margin-bottom: 0.5rem; border-radius: 4px; }
        .change-item.added { background: #f0fdf4; border-left: 4px solid var(--green); }
        .change-item.modified { background: #fefce8; border-left: 4px solid var(--yellow); }
        .change-item.removed { background: #fef2f2; border-left: 4px solid var(--red); }
        .change-id { font-weight: 600; margin-bottom: 0.25rem; }
        .field-diff { font-family: monospace; font-size: 0.875rem; margin-top: 0.5rem; padding: 0.5rem; background: rgba(0,0,0,0.03); border-radius: 4px; }
        .field-diff .path { color: var(--gray); }
        .field-diff .old { color: var(--red); }
        .field-diff .new { color: var(--green); }
        .identical { text-align: center; padding: 3rem; color: var(--green); }
        .identical svg { width: 48px; height: 48px; margin-bottom: 1rem; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Organization Comparison</h1>
        <p class="subtitle">{{.Source}} → {{.Target}} | {{.Timestamp}}</p>

        {{if .Identical}}
        <div class="identical">
            <svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
            <h2>Organizations are identical</h2>
            <p>No differences found between source and target.</p>
        </div>
        {{else}}
        <div class="summary">
            <div class="stat-card"><h3>Added</h3><div class="value added">{{.TotalAdded}}</div></div>
            <div class="stat-card"><h3>Modified</h3><div class="value modified">{{.TotalModified}}</div></div>
            <div class="stat-card"><h3>Removed</h3><div class="value removed">{{.TotalRemoved}}</div></div>
        </div>

        {{range .Sections}}
        <div class="section{{if .HasChanges}} open{{end}}">
            <div class="section-header" onclick="this.parentElement.classList.toggle('open')">
                <span class="section-title">{{.Name}}</span>
                <div class="section-stats">
                    {{if gt .Added 0}}<span class="added">+{{.Added}}</span>{{end}}
                    {{if gt .Modified 0}}<span class="modified">~{{.Modified}}</span>{{end}}
                    {{if gt .Removed 0}}<span class="removed">-{{.Removed}}</span>{{end}}
                    {{if not .HasChanges}}<span>identical</span>{{end}}
                </div>
            </div>
            <div class="section-content">
                {{range .AddedItems}}
                <div class="change-item added">
                    <div class="change-id">[+] {{.Identifier}}</div>
                </div>
                {{end}}
                {{range .ModifiedItems}}
                <div class="change-item modified">
                    <div class="change-id">[~] {{.Identifier}}</div>
                    {{range .FieldDiffs}}
                    <div class="field-diff">
                        <span class="path">{{.Path}}:</span><br>
                        <span class="old">- {{.SourceValue}}</span><br>
                        <span class="new">+ {{.TargetValue}}</span>
                    </div>
                    {{end}}
                </div>
                {{end}}
                {{range .RemovedItems}}
                <div class="change-item removed">
                    <div class="change-id">[-] {{.Identifier}}</div>
                </div>
                {{end}}
            </div>
        </div>
        {{end}}
        {{end}}
    </div>
</body>
</html>
```

**Step 2: Create simple HTML template**

```html
<!-- internal/modules/compare/templates/simple.html -->
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Port Organization Comparison</title>
    <style>
        body { font-family: sans-serif; max-width: 800px; margin: 2rem auto; padding: 0 1rem; }
        h1, h2 { color: #1f2937; }
        .added { color: #166534; }
        .modified { color: #854d0e; }
        .removed { color: #991b1b; }
        .summary { margin: 1rem 0; padding: 1rem; background: #f3f4f6; border-radius: 4px; }
        pre { background: #f9fafb; padding: 1rem; overflow-x: auto; }
    </style>
</head>
<body>
    <h1>Organization Comparison</h1>
    <p><strong>Source:</strong> {{.Source}} | <strong>Target:</strong> {{.Target}}</p>
    <p><strong>Generated:</strong> {{.Timestamp}}</p>

    <div class="summary">
        <strong>Summary:</strong>
        <span class="added">{{.TotalAdded}} added</span>,
        <span class="modified">{{.TotalModified}} modified</span>,
        <span class="removed">{{.TotalRemoved}} removed</span>
    </div>

    {{if .Identical}}
    <p class="added"><strong>Organizations are identical.</strong></p>
    {{else}}
    {{range .Sections}}
    <h2>{{.Name}}</h2>
    {{if not .HasChanges}}
    <p>No changes</p>
    {{else}}
    <ul>
        {{range .AddedItems}}<li class="added">[+] {{.Identifier}}</li>{{end}}
        {{range .ModifiedItems}}<li class="modified">[~] {{.Identifier}}</li>{{end}}
        {{range .RemovedItems}}<li class="removed">[-] {{.Identifier}}</li>{{end}}
    </ul>
    {{end}}
    {{end}}
    {{end}}
</body>
</html>
```

**Step 3: Implement HTML formatter**

```go
// internal/modules/compare/output_html.go
package compare

import (
	_ "embed"
	"html/template"
	"io"
)

//go:embed templates/report.html
var reportTemplate string

//go:embed templates/simple.html
var simpleTemplate string

// HTMLSection represents a section in the HTML report.
type HTMLSection struct {
	Name          string
	Added         int
	Modified      int
	Removed       int
	HasChanges    bool
	AddedItems    []ResourceChange
	ModifiedItems []ResourceChange
	RemovedItems  []ResourceChange
}

// HTMLData represents the data passed to HTML templates.
type HTMLData struct {
	Source        string
	Target        string
	Timestamp     string
	Identical     bool
	TotalAdded    int
	TotalModified int
	TotalRemoved  int
	Sections      []HTMLSection
}

// HTMLFormatter formats comparison results as HTML.
type HTMLFormatter struct {
	w      io.Writer
	simple bool
}

// NewHTMLFormatter creates a new HTML formatter.
func NewHTMLFormatter(w io.Writer, simple bool) *HTMLFormatter {
	return &HTMLFormatter{w: w, simple: simple}
}

// Format outputs the comparison result as HTML.
func (f *HTMLFormatter) Format(result *CompareResult) error {
	data := HTMLData{
		Source:    result.Source,
		Target:    result.Target,
		Timestamp: result.Timestamp,
		Identical: result.Identical,
	}

	// Build sections
	data.Sections = []HTMLSection{
		f.buildSection("Blueprints", result.Blueprints),
		f.buildSection("Actions", result.Actions),
		f.buildSection("Scorecards", result.Scorecards),
		f.buildSection("Pages", result.Pages),
		f.buildSection("Integrations", result.Integrations),
		f.buildSection("Teams", result.Teams),
		f.buildSection("Users", result.Users),
		f.buildSection("Automations", result.Automations),
	}

	// Calculate totals
	for _, s := range data.Sections {
		data.TotalAdded += s.Added
		data.TotalModified += s.Modified
		data.TotalRemoved += s.Removed
	}

	// Select template
	tmplStr := reportTemplate
	if f.simple {
		tmplStr = simpleTemplate
	}

	tmpl, err := template.New("report").Parse(tmplStr)
	if err != nil {
		return err
	}

	return tmpl.Execute(f.w, data)
}

func (f *HTMLFormatter) buildSection(name string, diff ResourceDiff) HTMLSection {
	return HTMLSection{
		Name:          name,
		Added:         diff.Summary.Added,
		Modified:      diff.Summary.Modified,
		Removed:       diff.Summary.Removed,
		HasChanges:    diff.Summary.Added > 0 || diff.Summary.Modified > 0 || diff.Summary.Removed > 0,
		AddedItems:    diff.Added,
		ModifiedItems: diff.Modified,
		RemovedItems:  diff.Removed,
	}
}
```

**Step 4: Verify compilation**

Run: `go build ./...`
Expected: Success

**Step 5: Commit**

```bash
git add internal/modules/compare/output_html.go internal/modules/compare/templates/
git commit -m "feat(compare): add HTML output formatter with interactive and simple templates"
```

---

## Task 7: Wire Up Module Execute Function

**Files:**
- Modify: `internal/modules/compare/compare.go`

**Step 1: Update compare.go with full implementation**

```go
// internal/modules/compare/compare.go
package compare

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/port-experimental/port-cli/internal/config"
)

// Module handles organization comparison operations.
type Module struct {
	configManager *config.ConfigManager
}

// NewModule creates a new compare module.
func NewModule(configManager *config.ConfigManager) *Module {
	return &Module{
		configManager: configManager,
	}
}

// Execute runs the comparison and returns results.
func (m *Module) Execute(ctx context.Context, opts Options) (*CompareResult, error) {
	fetcher := NewFetcher(m.configManager)

	// Fetch source data
	sourceOpts := FetchOptions{
		OrgName:      opts.SourceOrg,
		FilePath:     opts.SourceFile,
		ClientID:     opts.SourceClientID,
		ClientSecret: opts.SourceSecret,
	}
	if opts.SourceFile != "" {
		sourceOpts.FilePath = opts.SourceFile
	} else if opts.SourceOrg != "" {
		sourceOpts.OrgName = opts.SourceOrg
	}

	sourceData, err := fetcher.Fetch(ctx, sourceOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch source data: %w", err)
	}

	// Fetch target data
	targetOpts := FetchOptions{
		OrgName:      opts.TargetOrg,
		FilePath:     opts.TargetFile,
		ClientID:     opts.TargetClientID,
		ClientSecret: opts.TargetSecret,
	}
	if opts.TargetFile != "" {
		targetOpts.FilePath = opts.TargetFile
	} else if opts.TargetOrg != "" {
		targetOpts.OrgName = opts.TargetOrg
	}

	targetData, err := fetcher.Fetch(ctx, targetOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch target data: %w", err)
	}

	// Compute differences
	differ := NewDiffer()
	result := differ.Diff(sourceData.Data, targetData.Data)
	result.Source = sourceData.Name
	result.Target = targetData.Name
	result.Timestamp = time.Now().UTC().Format(time.RFC3339)

	return result, nil
}

// FormatOutput formats the result based on options.
func (m *Module) FormatOutput(result *CompareResult, opts Options) error {
	var w io.Writer = os.Stdout

	switch opts.OutputFormat {
	case "json":
		formatter := NewJSONFormatter(w)
		return formatter.Format(result)

	case "html":
		// Write to file for HTML
		filePath := opts.HTMLFile
		if filePath == "" {
			filePath = "comparison-report.html"
		}
		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create HTML file: %w", err)
		}
		defer file.Close()

		formatter := NewHTMLFormatter(file, opts.HTMLSimple)
		if err := formatter.Format(result); err != nil {
			return err
		}
		fmt.Printf("HTML report written to %s\n", filePath)
		return nil

	default: // text
		formatter := NewTextFormatter(w, opts.Verbose, opts.Full)
		return formatter.Format(result)
	}
}
```

**Step 2: Verify compilation**

Run: `go build ./...`
Expected: Success

**Step 3: Commit**

```bash
git add internal/modules/compare/compare.go
git commit -m "feat(compare): wire up module execute with fetcher, differ, and formatters"
```

---

## Task 8: Register CLI Command

**Files:**
- Create: `internal/commands/compare.go`

**Step 1: Implement compare command**

```go
// internal/commands/compare.go
package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/port-experimental/port-cli/internal/config"
	"github.com/port-experimental/port-cli/internal/modules/compare"
	"github.com/spf13/cobra"
)

// RegisterCompare registers the compare command.
func RegisterCompare(rootCmd *cobra.Command) {
	var (
		source           string
		target           string
		sourceClientID   string
		sourceSecret     string
		targetClientID   string
		targetSecret     string
		outputFormat     string
		htmlFile         string
		htmlSimple       bool
		verbose          bool
		full             bool
		include          string
		failOnDiff       bool
	)

	compareCmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare two Port organizations",
		Long: `Compare two Port organizations and show differences.

Compares blueprints, actions, scorecards, pages, integrations, teams, users, and automations
between a source and target organization.

Source and target can be:
- Organization names from config (e.g., 'staging', 'production')
- Export file paths (e.g., './staging-export.tar.gz')

Examples:
  # Compare two configured organizations
  port compare --source staging --target production

  # Compare with verbose output (show identifiers)
  port compare --source staging --target production --verbose

  # Compare with full diff (show field-level changes)
  port compare --source staging --target production --full

  # Compare export files
  port compare --source ./staging.tar.gz --target ./prod.tar.gz

  # Output as JSON
  port compare --source staging --target production --output json

  # Generate HTML report
  port compare --source staging --target production --output html

  # CI/CD mode: fail if differences found
  port compare --source staging --target production --fail-on-diff`,
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := GetGlobalFlags(cmd.Context())
			configManager := config.NewConfigManager(flags.ConfigFile)

			// Determine if inputs are files or org names
			sourceFile := ""
			sourceOrg := ""
			if isFilePath(source) {
				sourceFile = source
			} else {
				sourceOrg = source
			}

			targetFile := ""
			targetOrg := ""
			if isFilePath(target) {
				targetFile = target
			} else {
				targetOrg = target
			}

			// Parse include list
			var includeList []string
			if include != "" {
				includeList = strings.Split(include, ",")
				for i := range includeList {
					includeList[i] = strings.TrimSpace(includeList[i])
				}
			}

			opts := compare.Options{
				SourceOrg:        sourceOrg,
				TargetOrg:        targetOrg,
				SourceFile:       sourceFile,
				TargetFile:       targetFile,
				SourceClientID:   sourceClientID,
				SourceSecret:     sourceSecret,
				TargetClientID:   targetClientID,
				TargetSecret:     targetSecret,
				OutputFormat:     outputFormat,
				HTMLFile:         htmlFile,
				HTMLSimple:       htmlSimple,
				Verbose:          verbose,
				Full:             full,
				IncludeResources: includeList,
				FailOnDiff:       failOnDiff,
			}

			// Create module and execute
			module := compare.NewModule(configManager)
			result, err := module.Execute(cmd.Context(), opts)
			if err != nil {
				return fmt.Errorf("comparison failed: %w", err)
			}

			// Format output
			if err := module.FormatOutput(result, opts); err != nil {
				return fmt.Errorf("failed to format output: %w", err)
			}

			// Exit code handling
			if failOnDiff && !result.Identical {
				os.Exit(1)
			}

			return nil
		},
	}

	compareCmd.Flags().StringVar(&source, "source", "", "Source organization name or export file path (required)")
	compareCmd.Flags().StringVar(&target, "target", "", "Target organization name or export file path (required)")
	compareCmd.MarkFlagRequired("source")
	compareCmd.MarkFlagRequired("target")

	compareCmd.Flags().StringVar(&sourceClientID, "source-client-id", "", "Override source organization client ID")
	compareCmd.Flags().StringVar(&sourceSecret, "source-client-secret", "", "Override source organization client secret")
	compareCmd.Flags().StringVar(&targetClientID, "target-client-id", "", "Override target organization client ID")
	compareCmd.Flags().StringVar(&targetSecret, "target-client-secret", "", "Override target organization client secret")

	compareCmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json, html")
	compareCmd.Flags().StringVar(&htmlFile, "html-file", "comparison-report.html", "Output path for HTML report")
	compareCmd.Flags().BoolVar(&htmlSimple, "html-simple", false, "Generate simple HTML (no interactive features)")

	compareCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show changed resource identifiers")
	compareCmd.Flags().BoolVar(&full, "full", false, "Show full field-level differences")
	compareCmd.Flags().StringVar(&include, "include", "", "Comma-separated list of resource types to compare")
	compareCmd.Flags().BoolVar(&failOnDiff, "fail-on-diff", false, "Exit with code 1 if differences found")

	rootCmd.AddCommand(compareCmd)
}

// isFilePath checks if the input looks like a file path.
func isFilePath(input string) bool {
	return strings.HasSuffix(input, ".tar.gz") ||
		strings.HasSuffix(input, ".json") ||
		strings.HasPrefix(input, "/") ||
		strings.HasPrefix(input, "./") ||
		strings.HasPrefix(input, "../") ||
		strings.Contains(input, string(os.PathSeparator))
}
```

**Step 2: Register command in main.go**

Add to `cmd/port/main.go` in the "Add subcommands" section:

```go
commands.RegisterCompare(rootCmd)
```

**Step 3: Verify compilation**

Run: `go build ./...`
Expected: Success

**Step 4: Test CLI help**

Run: `./bin/port compare --help`
Expected: Shows compare command help with all flags

**Step 5: Commit**

```bash
git add internal/commands/compare.go cmd/port/main.go
git commit -m "feat(compare): register CLI command with all flags"
```

---

## Task 9: Add Integration Tests

**Files:**
- Create: `internal/modules/compare/compare_test.go`

**Step 1: Write integration test**

```go
// internal/modules/compare/compare_test.go
package compare

import (
	"bytes"
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/modules/export"
)

func TestFullComparisonWorkflow(t *testing.T) {
	// Create test data
	sourceData := &export.Data{
		Blueprints: []api.Blueprint{
			{"identifier": "service", "title": "Service", "schema": map[string]interface{}{}},
			{"identifier": "old-bp", "title": "Old Blueprint"},
		},
		Actions: []api.Action{
			{"identifier": "deploy", "title": "Deploy"},
		},
	}

	targetData := &export.Data{
		Blueprints: []api.Blueprint{
			{"identifier": "service", "title": "Service Updated", "schema": map[string]interface{}{}},
			{"identifier": "new-bp", "title": "New Blueprint"},
		},
		Actions: []api.Action{
			{"identifier": "deploy", "title": "Deploy"},
		},
	}

	// Run diff
	differ := NewDiffer()
	result := differ.Diff(sourceData, targetData)
	result.Source = "source-org"
	result.Target = "target-org"
	result.Timestamp = "2026-02-05T19:30:00Z"

	// Verify results
	if result.Identical {
		t.Error("expected differences, got identical")
	}

	if result.Blueprints.Summary.Added != 1 {
		t.Errorf("expected 1 added blueprint, got %d", result.Blueprints.Summary.Added)
	}
	if result.Blueprints.Summary.Modified != 1 {
		t.Errorf("expected 1 modified blueprint, got %d", result.Blueprints.Summary.Modified)
	}
	if result.Blueprints.Summary.Removed != 1 {
		t.Errorf("expected 1 removed blueprint, got %d", result.Blueprints.Summary.Removed)
	}

	// Test text output
	var textBuf bytes.Buffer
	textFormatter := NewTextFormatter(&textBuf, false, false)
	if err := textFormatter.Format(result); err != nil {
		t.Fatalf("text formatter error: %v", err)
	}
	if textBuf.Len() == 0 {
		t.Error("expected text output, got empty")
	}

	// Test JSON output
	var jsonBuf bytes.Buffer
	jsonFormatter := NewJSONFormatter(&jsonBuf)
	if err := jsonFormatter.Format(result); err != nil {
		t.Fatalf("json formatter error: %v", err)
	}
	if jsonBuf.Len() == 0 {
		t.Error("expected JSON output, got empty")
	}

	// Test HTML output
	var htmlBuf bytes.Buffer
	htmlFormatter := NewHTMLFormatter(&htmlBuf, false)
	if err := htmlFormatter.Format(result); err != nil {
		t.Fatalf("html formatter error: %v", err)
	}
	if htmlBuf.Len() == 0 {
		t.Error("expected HTML output, got empty")
	}
}

func TestIdenticalOrganizations(t *testing.T) {
	data := &export.Data{
		Blueprints: []api.Blueprint{
			{"identifier": "service", "title": "Service"},
		},
	}

	differ := NewDiffer()
	result := differ.Diff(data, data)

	if !result.Identical {
		t.Error("expected identical, got differences")
	}
}
```

**Step 2: Run all tests**

Run: `go test ./internal/modules/compare/... -v`
Expected: All tests PASS

**Step 3: Commit**

```bash
git add internal/modules/compare/compare_test.go
git commit -m "test(compare): add integration tests for full workflow"
```

---

## Task 10: Final Polish and Documentation

**Files:**
- Modify: `README.md` (add compare command documentation)

**Step 1: Run full test suite**

Run: `make check`
Expected: All tests and linting pass

**Step 2: Test manually with real data (if available)**

```bash
# Build
make build

# Test with export files (if you have them)
./bin/port compare --source ./staging.tar.gz --target ./prod.tar.gz

# Test help
./bin/port compare --help
```

**Step 3: Commit any final fixes**

```bash
git add -A
git commit -m "chore(compare): final polish and fixes"
```

**Step 4: Push feature branch**

```bash
git push -u origin feature/port-compare
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Module structure and types | `types.go`, `compare.go` |
| 2 | Fetcher for orgs and files | `fetcher.go`, `fetcher_test.go` |
| 3 | Differ with field comparison | `differ.go`, `differ_test.go` |
| 4 | Text output formatter | `output_text.go`, `output_text_test.go` |
| 5 | JSON output formatter | `output_json.go`, `output_json_test.go` |
| 6 | HTML output formatter | `output_html.go`, `templates/` |
| 7 | Wire up module execute | `compare.go` |
| 8 | Register CLI command | `commands/compare.go`, `main.go` |
| 9 | Integration tests | `compare_test.go` |
| 10 | Final polish | README, testing |

**Total: ~10 tasks, each completable in 10-20 minutes**

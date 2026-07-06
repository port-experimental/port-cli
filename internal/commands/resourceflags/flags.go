package resourceflags

import (
	"fmt"
	"slices"
	"strings"
)

// ValidIncludeResources lists accepted --include resource type names.
var ValidIncludeResources = []string{
	"blueprints",
	"entities",
	"scorecards",
	"actions",
	"teams",
	"users",
	"automations",
	"pages",
	"integrations",
	"blueprint-permissions",
	"action-permissions",
	"page-permissions",
}

var validIncludeSet = func() map[string]bool {
	m := make(map[string]bool, len(ValidIncludeResources))
	for _, r := range ValidIncludeResources {
		m[r] = true
	}
	return m
}()

// Filters holds per-resource identifier filters parsed from CLI flags.
type Filters struct {
	Entities     []string
	Scorecards   []string
	Actions      []string
	Pages        []string
	Integrations []string
	Teams        []string
	Users        []string
}

// Selection is the resolved include list, auto-scope signal, and ID filters.
type Selection struct {
	IncludeResources    []string
	AutoScopeBlueprints bool
	Filters             Filters
}

// BuildInput captures parsed CLI state for export/migrate resource selection.
type BuildInput struct {
	IncludeCSV        string
	SkipEntities      bool
	BlueprintsChanged bool
	Changed           map[string]bool
	Filters           Filters
}

// ParseCSV splits a comma-separated flag value into trimmed, non-empty tokens.
func ParseCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// ParseInclude parses a --include CSV value.
func ParseInclude(include string) []string {
	return ParseCSV(include)
}

// ValidateInclude checks resource names and dependent include rules.
func ValidateInclude(includeList []string) error {
	for _, r := range includeList {
		if !validIncludeSet[r] {
			return fmt.Errorf(
				"invalid resource: %s. Valid resources: %s",
				r,
				strings.Join(ValidIncludeResources, ", "),
			)
		}
	}
	if slices.Contains(includeList, "page-permissions") && !slices.Contains(includeList, "pages") {
		return fmt.Errorf("page-permissions requires pages to also be included (add 'pages' to --include)")
	}
	return nil
}

// ReconcileSkipEntities clears skipEntities when --include explicitly requests
// entity-like resources. Returns updated skipEntities and user-facing warnings.
func ReconcileSkipEntities(includeList []string, skipEntities bool) (bool, []string) {
	if !skipEntities || len(includeList) == 0 {
		return skipEntities, nil
	}
	var warnings []string
	for _, r := range includeList {
		switch r {
		case "entities":
			warnings = append(warnings, "Warning: --skip-entities conflicts with --include entities, ignoring --skip-entities")
			return false, warnings
		case "users":
			warnings = append(warnings, "Warning: --skip-entities conflicts with --include users, ignoring --skip-entities")
			skipEntities = false
		case "teams":
			warnings = append(warnings, "Warning: --skip-entities conflicts with --include teams, ignoring --skip-entities")
			skipEntities = false
		}
	}
	return skipEntities, warnings
}

// ParseAndValidateInclude parses and validates a --include CSV value.
func ParseAndValidateInclude(includeCSV string) ([]string, error) {
	includeList := ParseInclude(includeCSV)
	if len(includeList) == 0 {
		return nil, nil
	}
	if err := ValidateInclude(includeList); err != nil {
		return nil, err
	}
	return includeList, nil
}

// BuildSelection resolves the effective include list and auto-scope behavior for
// export and migrate from --include and per-resource flags.
func BuildSelection(in BuildInput) (Selection, error) {
	includeList := ParseInclude(in.IncludeCSV)
	if err := ValidateInclude(includeList); err != nil {
		return Selection{}, err
	}

	blueprintsExplicitlyRequested := in.BlueprintsChanged || slices.Contains(includeList, "blueprints")

	needBlueprints := false
	if len(in.Filters.Entities) > 0 || in.Changed["entities"] {
		includeList = ensureContains(includeList, "entities")
		needBlueprints = true
	}
	if len(in.Filters.Scorecards) > 0 || in.Changed["scorecards"] {
		includeList = ensureContains(includeList, "scorecards")
		needBlueprints = true
	}
	if len(in.Filters.Actions) > 0 || in.Changed["actions"] {
		includeList = ensureContains(includeList, "actions")
		includeList = ensureContains(includeList, "action-permissions")
		needBlueprints = true
	}
	if len(in.Filters.Pages) > 0 || in.Changed["pages"] {
		includeList = ensureContains(includeList, "pages")
		includeList = ensureContains(includeList, "page-permissions")
	}
	if len(in.Filters.Integrations) > 0 || in.Changed["integrations"] {
		includeList = ensureContains(includeList, "integrations")
	}
	if len(in.Filters.Teams) > 0 || in.Changed["teams"] {
		includeList = ensureContains(includeList, "teams")
	}
	if len(in.Filters.Users) > 0 || in.Changed["users"] {
		includeList = ensureContains(includeList, "users")
	}
	if in.BlueprintsChanged || (needBlueprints && len(includeList) > 0) {
		includeList = ensureContains(includeList, "blueprints")
	}

	return Selection{
		IncludeResources:    includeList,
		AutoScopeBlueprints: needBlueprints && !blueprintsExplicitlyRequested,
		Filters:             in.Filters,
	}, nil
}

func ensureContains(list []string, item string) []string {
	if slices.Contains(list, item) {
		return list
	}
	return append(list, item)
}

package resourceflags

import (
	"fmt"

	"github.com/spf13/cobra"
)

// PerResourceFlagRefs points at command-local backing vars for per-resource flags.
type PerResourceFlagRefs struct {
	Blueprints   *string
	Scorecards   *string
	Actions      *string
	Pages        *string
	Integrations *string
	Teams        *string
	Users        *string
	Entities     *string
}

// RegisterInclude registers the shared --include flag.
func RegisterInclude(cmd *cobra.Command, include *string, verb string) {
	cmd.Flags().StringVar(
		include,
		"include",
		"",
		fmt.Sprintf(
			"Comma-separated list of resources to %s (e.g., 'blueprints,pages'). Available: blueprints, entities, scorecards, actions, teams, users, automations, pages, integrations. If not specified, %ss all resources.",
			verb, verb,
		),
	)
}

// RegisterPerResourceFilters registers per-resource ID filter flags used by export and migrate.
func RegisterPerResourceFilters(cmd *cobra.Command, refs PerResourceFlagRefs, verb string) {
	cmd.Flags().StringVarP(
		refs.Blueprints,
		"blueprints",
		"b",
		"",
		fmt.Sprintf(
			"Comma-separated list of blueprint IDs to %s (restricts %s to blueprints resource type; %ss all blueprints if flag set without IDs; pass this flag explicitly to %s the full blueprint set even when combined with --actions/--scorecards/--entities)",
			verb, verb, verb, verb,
		),
	)
	cmd.Flags().StringVar(
		refs.Scorecards,
		"scorecards",
		"",
		fmt.Sprintf(
			"Comma-separated scorecard IDs to %s (restricts %s to scorecards resource type; blueprint schemas %sd alongside are scoped to only the blueprints the selected scorecards belong to — use --blueprints to %s the full set instead)",
			verb, verb, verb, verb,
		),
	)
	cmd.Flags().StringVar(
		refs.Actions,
		"actions",
		"",
		fmt.Sprintf(
			"Comma-separated action IDs to %s (restricts %s to actions resource type; %ss all actions if flag set without IDs; blueprint schemas %sd alongside are scoped to only the blueprints the selected actions belong to — use --blueprints to %s the full set instead)",
			verb, verb, verb, verb, verb,
		),
	)
	cmd.Flags().StringVar(
		refs.Pages,
		"pages",
		"",
		fmt.Sprintf("Comma-separated page IDs to %s (restricts %s to pages resource type)", verb, verb),
	)
	cmd.Flags().StringVar(
		refs.Integrations,
		"integrations",
		"",
		fmt.Sprintf("Comma-separated integration IDs to %s (restricts %s to integrations resource type; %ss integration mapping only)", verb, verb, verb),
	)
	cmd.Flags().StringVar(
		refs.Teams,
		"teams",
		"",
		fmt.Sprintf("Comma-separated team names to %s (restricts %s to teams resource type)", verb, verb),
	)
	cmd.Flags().StringVar(
		refs.Users,
		"users",
		"",
		fmt.Sprintf("Comma-separated user emails to %s (restricts %s to users resource type)", verb, verb),
	)
	cmd.Flags().StringVar(
		refs.Entities,
		"entities",
		"",
		fmt.Sprintf(
			"Comma-separated entity IDs to %s (restricts %s to entities resource type; blueprint schemas %sd alongside are scoped to only the blueprints the selected entities belong to — use --blueprints to %s the full set instead)",
			verb, verb, verb, verb,
		),
	)
}

// Changed reports whether each named flag was explicitly set on the command.
func Changed(flags interface {
	Changed(string) bool
}, names ...string) map[string]bool {
	out := make(map[string]bool, len(names))
	for _, name := range names {
		out[name] = flags.Changed(name)
	}
	return out
}

// ParseFilters parses per-resource CSV flag values into Filters.
func ParseFilters(refs PerResourceFlagRefs) Filters {
	return Filters{
		Entities:     ParseCSV(deref(refs.Entities)),
		Scorecards:   ParseCSV(deref(refs.Scorecards)),
		Actions:      ParseCSV(deref(refs.Actions)),
		Pages:        ParseCSV(deref(refs.Pages)),
		Integrations: ParseCSV(deref(refs.Integrations)),
		Teams:        ParseCSV(deref(refs.Teams)),
		Users:        ParseCSV(deref(refs.Users)),
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

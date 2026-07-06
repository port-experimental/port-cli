package migrate

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/auth"
	"github.com/port-experimental/port-cli/internal/config"
	entitystream "github.com/port-experimental/port-cli/internal/modules/entity_stream"
	"github.com/port-experimental/port-cli/internal/modules/export"
	"github.com/port-experimental/port-cli/internal/modules/import_module"
	"github.com/port-experimental/port-cli/internal/plan"
	"github.com/port-experimental/port-cli/internal/resources"
	systemblueprints "github.com/port-experimental/port-cli/internal/modules/system_blueprints"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// maxConcurrentBlueprints caps how many blueprints exportFromSource fetches
// scorecards/actions/permissions/entity-relevance for in parallel, to avoid
// firing one goroutine per blueprint (100+ simultaneous requests in large
// orgs) and exhausting the read-side rate limit before any response arrives.
// Mirrors export/collector.go's identical bound for the same reason.
const maxConcurrentBlueprints = 10

// Module handles migration between Port organizations.
type Module struct {
	sourceClient *api.Client
	targetClient *api.Client
}

// NewModule creates a new migration module.
func NewModule(sourceToken, targetToken *auth.Token, sourceConfig, targetConfig *config.OrganizationConfig) *Module {
	return NewModuleFromClients(
		api.NewClient(api.ClientOpts{
			Token:        sourceToken,
			ClientID:     sourceConfig.ClientID,
			ClientSecret: sourceConfig.ClientSecret,
			APIURL:       sourceConfig.APIURL,
			Timeout:      0,
		}),
		api.NewClient(api.ClientOpts{
			Token:        targetToken,
			ClientID:     targetConfig.ClientID,
			ClientSecret: targetConfig.ClientSecret,
			APIURL:       targetConfig.APIURL,
			Timeout:      0,
		}),
	)
}

// NewModuleFromClients creates a migration module from pre-built API clients.
func NewModuleFromClients(sourceClient, targetClient *api.Client) *Module {
	return &Module{
		sourceClient: sourceClient,
		targetClient: targetClient,
	}
}

// Options represents migration options.
type Options struct {
	Blueprints                    []string
	DryRun                        bool
	SkipEntities                  bool
	SkipSystemBlueprints          bool // skip _* blueprint schemas and their entities
	SkipSystemBlueprintProperties bool
	IncludeRuleResults            bool // include _rule_result system blueprint entities (included by default)
	IncludeResources              []string
	ExcludeBlueprints             []string // deep: exclude blueprint schema + all its resources
	ExcludeBlueprintSchema        []string // shallow: exclude only the blueprint schema, keep resources
	UsersAsDisabled               bool     // import non-admin users as DISABLED after staging

	// AutoScopeBlueprints, when true, narrows the blueprint schemas returned by
	// exportFromSource to only the blueprints referenced by a matching
	// scorecard, action, or entity (see FilterBlueprintsToReferenced and
	// blueprintHasMatchingEntity). It is false whenever the caller explicitly
	// requested blueprints via --blueprints or --include blueprints.
	AutoScopeBlueprints bool

	// Per-resource ID filters (client-side, applied after bulk fetch)
	Entities     []string
	Scorecards   []string
	Actions      []string
	Pages        []string
	Integrations []string
	Teams        []string
	Users        []string
}

// Result represents the result of a migration operation.
type Result struct {
	Success                              bool
	Message                              string
	BlueprintsCreated                    int
	BlueprintsUpdated                    int
	BlueprintsSkipped                    int
	EntitiesCreated                      int
	EntitiesUpdated                      int
	EntitiesSkipped                      int
	ScorecardsCreated                    int
	ScorecardsUpdated                    int
	ScorecardsSkipped                    int
	ActionsCreated                       int
	ActionsUpdated                       int
	ActionsSkipped                       int
	TeamsCreated                         int
	TeamsUpdated                         int
	TeamsSkipped                         int
	UsersCreated                         int
	UsersUpdated                         int
	UsersSkipped                         int
	PagesCreated                         int
	PagesUpdated                         int
	PagesSkipped                         int
	IntegrationsUpdated                  int
	IntegrationsSkipped                  int
	BlueprintPermissionsUpdated          int
	ActionPermissionsUpdated             int
	PagePermissionsUpdated               int
	BlueprintsToCreate                   []string
	BlueprintsToUpdate                   []string
	BlueprintsToSkip                     []string
	BlueprintPermissionsToUpdate         []string
	ActionPermissionsToUpdate            []string
	PagePermissionsToUpdate              []string
	Errors                               []string
	Warnings                             []string
	DiffResult                           *import_module.DiffResult
	IgnoredRuleResultTargetRelationCount int
	IgnoredRuleResultTargetRelationKeys  []string
}

// Execute performs the migration operation.
func (m *Module) Execute(ctx context.Context, opts Options) (*Result, error) {
	// Export from source
	sourceData, entityBlueprints, cachedMatchedEntities, err := m.exportFromSource(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to export from source: %w", err)
	}
	streamEntities := !opts.SkipEntities && shouldCollect("entities", opts.IncludeResources)

	// Diff validation - compare source data with target organization's current state
	comparer := import_module.NewDiffComparer(m.targetClient)
	diffOpts := import_module.Options{
		SkipEntities:                  opts.SkipEntities || streamEntities,
		SkipSystemBlueprints:          opts.SkipSystemBlueprints,
		SkipSystemBlueprintProperties: opts.SkipSystemBlueprintProperties,
		IncludeRuleResults:            opts.IncludeRuleResults,
		IncludeResources:              opts.IncludeResources,
		ExcludeBlueprints:             opts.ExcludeBlueprints,
		ExcludeBlueprintSchema:        opts.ExcludeBlueprintSchema,
	}
	diffResult, err := comparer.Compare(ctx, sourceData, diffOpts)
	if err != nil {
		return nil, fmt.Errorf("diff comparison failed: %w", err)
	}

	executionPlan := plan.BuildFromDiffResult(diffResult)

	// Use diff result to filter data - only migrate what needs to be created or updated
	filteredData := diffResult.FilterData(sourceData)

	// Dry run - show what would happen
	if opts.DryRun {
		result := m.generateDryRunResult(executionPlan, diffResult)
		if streamEntities {
			if err := m.migrateEntities(ctx, entityBlueprints, opts, result, true, cachedMatchedEntities); err != nil {
				markMigrationStopped(result, diffResult, err)
				return result, fmt.Errorf("streaming entity dry run failed: %w", err)
			}
		}
		return result, nil
	}

	// Import to target using filtered data
	result, err := m.importToTarget(ctx, filteredData, executionPlan, diffResult, opts, streamEntities)
	if err != nil {
		return nil, fmt.Errorf("failed to import to target: %w", err)
	}
	if streamEntities {
		if err := m.migrateEntities(ctx, entityBlueprints, opts, result, false, cachedMatchedEntities); err != nil {
			markMigrationStopped(result, diffResult, err)
			return result, fmt.Errorf("failed to migrate entities: %w", err)
		}
	}

	if len(result.Errors) > 0 {
		result.Success = false
		result.Message = fmt.Sprintf("Migration completed with %d error(s)", len(result.Errors))
	} else {
		result.Success = true
		result.Message = "Migration completed successfully"
	}
	result.DiffResult = diffResult
	return result, nil
}

func markMigrationStopped(result *Result, diffResult *import_module.DiffResult, err error) {
	if result == nil {
		return
	}
	if len(result.Errors) == 0 && err != nil {
		result.Errors = append(result.Errors, err.Error())
	}
	result.Success = false
	result.Message = fmt.Sprintf("Migration stopped with %d error(s)", len(result.Errors))
	result.DiffResult = diffResult
}

// generateDryRunResult generates a dry run result from the execution plan.
func (m *Module) generateDryRunResult(executionPlan *plan.ExecutionPlan, diffResult *import_module.DiffResult) *Result {
	summary := plan.Summarize(executionPlan)
	return &Result{
		Success:                      true,
		Message:                      "Migration validation passed (dry run - no changes applied)",
		BlueprintsCreated:            summary.Created[resources.KindBlueprints],
		BlueprintsUpdated:            summary.Updated[resources.KindBlueprints],
		BlueprintsSkipped:            summary.Skipped[resources.KindBlueprints],
		EntitiesCreated:              summary.Created[resources.KindEntities],
		EntitiesUpdated:              summary.Updated[resources.KindEntities],
		EntitiesSkipped:              summary.Skipped[resources.KindEntities],
		ScorecardsCreated:            summary.Created[resources.KindScorecards],
		ScorecardsUpdated:            summary.Updated[resources.KindScorecards],
		ScorecardsSkipped:            summary.Skipped[resources.KindScorecards],
		ActionsCreated:               summary.Created[resources.KindActions],
		ActionsUpdated:               summary.Updated[resources.KindActions],
		ActionsSkipped:               summary.Skipped[resources.KindActions],
		TeamsCreated:                 summary.Created[resources.KindTeams],
		TeamsUpdated:                 summary.Updated[resources.KindTeams],
		TeamsSkipped:                 summary.Skipped[resources.KindTeams],
		UsersCreated:                 summary.Created[resources.KindUsers],
		UsersUpdated:                 summary.Updated[resources.KindUsers],
		UsersSkipped:                 summary.Skipped[resources.KindUsers],
		PagesCreated:                 summary.Created[resources.KindPages],
		PagesUpdated:                 summary.Updated[resources.KindPages],
		PagesSkipped:                 summary.Skipped[resources.KindPages],
		IntegrationsUpdated:          summary.Updated[resources.KindIntegrations],
		IntegrationsSkipped:          summary.Skipped[resources.KindIntegrations],
		BlueprintPermissionsUpdated:  summary.PermissionUpdates[resources.KindBlueprintPermissions],
		ActionPermissionsUpdated:     summary.PermissionUpdates[resources.KindActionPermissions],
		PagePermissionsUpdated:       summary.PermissionUpdates[resources.KindPagePermissions],
		BlueprintsToCreate:           plan.Identifiers(executionPlan, resources.KindBlueprints, plan.OpCreate),
		BlueprintsToUpdate:           plan.Identifiers(executionPlan, resources.KindBlueprints, plan.OpUpdate),
		BlueprintsToSkip:             plan.Identifiers(executionPlan, resources.KindBlueprints, plan.OpSkip),
		BlueprintPermissionsToUpdate: plan.Identifiers(executionPlan, resources.KindBlueprintPermissions, plan.OpPermissionUpdate),
		ActionPermissionsToUpdate:    plan.Identifiers(executionPlan, resources.KindActionPermissions, plan.OpPermissionUpdate),
		PagePermissionsToUpdate:      plan.Identifiers(executionPlan, resources.KindPagePermissions, plan.OpPermissionUpdate),
		DiffResult:                   diffResult,
	}
}

// shouldCollect checks if a resource type should be collected.
func shouldCollect(resourceType string, includeResources []string) bool {
	if len(includeResources) == 0 {
		return true
	}

	for _, r := range includeResources {
		if r == resourceType {
			return true
		}
	}
	return false
}

// exportFromSource exports metadata from the source organization and returns
// the blueprints eligible for streaming entity migration, plus any entities
// already fetched for those blueprints while deciding AutoScopeBlueprints
// relevance (see blueprintHasMatchingEntity) — migrateEntities reuses these
// instead of re-fetching.
func (m *Module) exportFromSource(ctx context.Context, opts Options) (*export.Data, []api.Blueprint, map[string][]api.Entity, error) {
	// Collect blueprints first
	allBlueprints, err := m.sourceClient.GetBlueprints(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get blueprints: %w", err)
	}

	// Filter blueprints if specified
	var selectedBlueprints []api.Blueprint
	if len(opts.Blueprints) > 0 {
		blueprintSet := make(map[string]bool)
		for _, bpID := range opts.Blueprints {
			blueprintSet[bpID] = true
		}

		for _, bp := range allBlueprints {
			if identifier, ok := bp["identifier"].(string); ok && blueprintSet[identifier] {
				selectedBlueprints = append(selectedBlueprints, bp)
			}
		}
	} else {
		selectedBlueprints = allBlueprints
	}

	// Resolve dependencies
	resolvedBlueprints := m.resolveDependencies(allBlueprints, selectedBlueprints)

	// Apply exclusions: iterBlueprints is used to fetch entities/scorecards/actions,
	// dataBlueprints is what ends up in data.Blueprints (schema output).
	excludeDeep := opts.ExcludeBlueprints
	if !opts.IncludeRuleResults {
		excludeDeep = append(excludeDeep, "_rule_result")
	}
	iterBlueprints, dataBlueprints := systemblueprints.ApplyExclusions(
		resolvedBlueprints,
		excludeDeep,
		opts.ExcludeBlueprintSchema,
		opts.SkipSystemBlueprints,
		opts.SkipSystemBlueprintProperties,
	)
	if !shouldCollect("blueprints", opts.IncludeResources) {
		dataBlueprints = []api.Blueprint{}
	}
	// scopeBlueprintsToReferenced narrows dataBlueprints, once collection below
	// completes, to only the blueprints that produced a matching
	// scorecard/action/entity — see Options.AutoScopeBlueprints doc comment.
	scopeBlueprintsToReferenced := opts.AutoScopeBlueprints && shouldCollect("blueprints", opts.IncludeResources)
	entityBlueprints := make([]api.Blueprint, 0, len(iterBlueprints))
	if !opts.SkipEntities && shouldCollect("entities", opts.IncludeResources) {
		for _, blueprint := range iterBlueprints {
			bpID, _ := blueprint["identifier"].(string)
			if bpID == "" {
				continue
			}
			if opts.SkipSystemBlueprints && strings.HasPrefix(bpID, "_") {
				continue
			}
			entityBlueprints = append(entityBlueprints, blueprint)
		}
	}

	data := &export.Data{
		Blueprints:           dataBlueprints,
		Entities:             []api.Entity{},
		Scorecards:           []api.Scorecard{},
		Actions:              []api.Action{},
		Teams:                []api.Team{},
		Users:                []api.User{},
		Folders:              []api.Folder{},
		Pages:                []api.Page{},
		Integrations:         []api.Integration{},
		BlueprintPermissions: make(map[string]api.Permissions),
		ActionPermissions:    make(map[string]api.Permissions),
		PagePermissions:      make(map[string]api.Permissions),
	}

	// Use errgroup for concurrent collection, bounded by semaphore (see
	// maxConcurrentBlueprints doc comment).
	g, ctx := errgroup.WithContext(ctx)
	sem := semaphore.NewWeighted(maxConcurrentBlueprints)
	var mu sync.Mutex
	referencedBlueprintIDs := make(map[string]bool)

	// Collect scorecards and actions concurrently per blueprint. Entities are
	// migrated later with a bounded pull/push loop per blueprint.
	for _, blueprint := range iterBlueprints {
		bp := blueprint
		bpID, ok := bp["identifier"].(string)
		if !ok {
			continue
		}

		// Collect scorecards
		if shouldCollect("scorecards", opts.IncludeResources) {
			if err := sem.Acquire(ctx, 1); err != nil {
				return nil, nil, nil, err
			}
			g.Go(func() error {
				defer sem.Release(1)
				scorecards, err := m.sourceClient.GetScorecards(ctx, bpID)
				if err != nil {
					if !strings.Contains(err.Error(), "410 Gone") {
						return fmt.Errorf("failed to get scorecards for blueprint %s: %w", bpID, err)
					}
					return nil
				}

				// Ensure scorecards have blueprintIdentifier field
				for i := range scorecards {
					if _, exists := scorecards[i]["blueprintIdentifier"]; !exists {
						scorecards[i]["blueprintIdentifier"] = bpID
					}
				}

				scorecards = export.FilterByField(scorecards, opts.Scorecards, "identifier")
				mu.Lock()
				data.Scorecards = append(data.Scorecards, scorecards...)
				if scopeBlueprintsToReferenced && len(scorecards) > 0 {
					referencedBlueprintIDs[bpID] = true
				}
				mu.Unlock()
				return nil
			})
		}

		// Collect actions
		if shouldCollect("actions", opts.IncludeResources) {
			if err := sem.Acquire(ctx, 1); err != nil {
				return nil, nil, nil, err
			}
			g.Go(func() error {
				defer sem.Release(1)
				actions, err := m.sourceClient.GetActions(ctx, bpID)
				if err != nil {
					if !strings.Contains(err.Error(), "410 Gone") {
						return fmt.Errorf("failed to get actions for blueprint %s: %w", bpID, err)
					}
					return nil
				}

				actions = export.FilterByField(actions, opts.Actions, "identifier")
				mu.Lock()
				data.Actions = append(data.Actions, actions...)
				if scopeBlueprintsToReferenced && len(actions) > 0 {
					referencedBlueprintIDs[bpID] = true
				}
				mu.Unlock()

				// Fetch permissions for each action
				if shouldCollect("action-permissions", opts.IncludeResources) || len(opts.IncludeResources) == 0 {
					for _, action := range actions {
						actionID, ok := action["identifier"].(string)
						if !ok {
							continue
						}
						aID := actionID
						g.Go(func() error {
							perms, err := m.sourceClient.GetActionPermissions(ctx, aID)
							if err != nil {
								mu.Lock()
								data.Warnings = append(data.Warnings, fmt.Sprintf("failed to fetch permissions for action %s: %v", aID, err))
								mu.Unlock()
								return nil
							}
							mu.Lock()
							data.ActionPermissions[aID] = perms
							mu.Unlock()
							return nil
						})
					}
				}
				return nil
			})
		}

		// Collect blueprint permissions
		if shouldCollect("blueprint-permissions", opts.IncludeResources) || len(opts.IncludeResources) == 0 {
			bpIDCopy := bpID
			if err := sem.Acquire(ctx, 1); err != nil {
				return nil, nil, nil, err
			}
			g.Go(func() error {
				defer sem.Release(1)
				perms, err := m.sourceClient.GetBlueprintPermissions(ctx, bpIDCopy)
				if err != nil {
					mu.Lock()
					data.Warnings = append(data.Warnings, fmt.Sprintf("failed to fetch permissions for blueprint %s: %v", bpIDCopy, err))
					mu.Unlock()
					return nil
				}
				mu.Lock()
				data.BlueprintPermissions[bpIDCopy] = perms
				mu.Unlock()
				return nil
			})
		}
	}

	// cachedMatchedEntities holds, per blueprint, the entities found by the
	// relevance pre-scan below when opts.Entities is set. migrateEntities
	// reuses these instead of re-fetching from the source for the same
	// blueprint — see blueprintHasMatchingEntity's doc comment.
	cachedMatchedEntities := make(map[string][]api.Entity)

	// When AutoScopeBlueprints narrowing is active, check each entity-eligible
	// blueprint for at least one matching entity now, so blueprints needed only
	// for --entities are known before the diff/import phase runs — entities
	// themselves are migrated later, by migrateEntities, which runs after
	// blueprint schemas have already been diffed and imported to the target.
	if scopeBlueprintsToReferenced && !opts.SkipEntities && shouldCollect("entities", opts.IncludeResources) {
		for _, blueprint := range entityBlueprints {
			bpID, _ := blueprint["identifier"].(string)
			if bpID == "" {
				continue
			}
			bpIDCopy := bpID
			if err := sem.Acquire(ctx, 1); err != nil {
				return nil, nil, nil, err
			}
			g.Go(func() error {
				defer sem.Release(1)
				found, matched, err := m.blueprintHasMatchingEntity(ctx, bpIDCopy, opts.Entities)
				if err != nil {
					return fmt.Errorf("failed to check entities for blueprint %s: %w", bpIDCopy, err)
				}
				if found {
					mu.Lock()
					referencedBlueprintIDs[bpIDCopy] = true
					if len(matched) > 0 {
						cachedMatchedEntities[bpIDCopy] = matched
					}
					mu.Unlock()
				}
				return nil
			})
		}
	}

	// Collect organization-wide resources
	if !opts.SkipEntities && shouldCollect("teams", opts.IncludeResources) {
		g.Go(func() error {
			teams, err := m.sourceClient.GetTeams(ctx)
			if err != nil {
				return nil // Non-fatal
			}

			teams = export.FilterByField(teams, opts.Teams, "name")
			mu.Lock()
			data.Teams = teams
			mu.Unlock()
			return nil
		})
	}

	if !opts.SkipEntities && shouldCollect("users", opts.IncludeResources) {
		g.Go(func() error {
			users, err := m.sourceClient.GetUsers(ctx)
			if err != nil {
				return nil // Non-fatal
			}

			users = export.FilterByField(users, opts.Users, "email")
			mu.Lock()
			data.Users = users
			mu.Unlock()
			return nil
		})
	}

	// Collect organization-wide automations (via GetAllActions) and merge into actions
	if shouldCollect("actions", opts.IncludeResources) || shouldCollect("automations", opts.IncludeResources) {
		g.Go(func() error {
			allActions, err := m.sourceClient.GetAllActions(ctx)
			if err != nil {
				return nil // Non-fatal
			}

			allActions = export.FilterByField(allActions, opts.Actions, "identifier")
			mu.Lock()
			data.Actions = append(data.Actions, allActions...)
			if scopeBlueprintsToReferenced {
				for _, action := range allActions {
					if bpID := export.ActionBlueprintID(action); bpID != "" {
						referencedBlueprintIDs[bpID] = true
					}
				}
			}
			mu.Unlock()

			// Fetch permissions for each org-wide action
			if shouldCollect("action-permissions", opts.IncludeResources) || len(opts.IncludeResources) == 0 {
				for _, action := range allActions {
					actionID, ok := action["identifier"].(string)
					if !ok {
						continue
					}
					aID := actionID
					g.Go(func() error {
						perms, err := m.sourceClient.GetActionPermissions(ctx, aID)
						if err != nil {
							mu.Lock()
							data.Warnings = append(data.Warnings, fmt.Sprintf("failed to fetch permissions for action %s: %v", aID, err))
							mu.Unlock()
							return nil
						}
						mu.Lock()
						data.ActionPermissions[aID] = perms
						mu.Unlock()
						return nil
					})
				}
			}
			return nil
		})
	}

	if shouldCollect("pages", opts.IncludeResources) {
		g.Go(func() error {
			folders, err := m.sourceClient.GetFolders(ctx)
			if err != nil {
				return nil // Non-fatal
			}
			pages, err := m.sourceClient.GetPages(ctx)
			if err != nil {
				return nil // Non-fatal
			}

			pages = export.FilterByField(pages, opts.Pages, "identifier")
			if len(opts.Pages) > 0 {
				folders = export.FilterFoldersToAncestors(folders, pages)
			}

			mu.Lock()
			data.Folders = folders
			data.Pages = pages
			mu.Unlock()

			// Fetch permissions for each page
			if shouldCollect("page-permissions", opts.IncludeResources) || len(opts.IncludeResources) == 0 {
				for _, page := range pages {
					pageID, ok := page["identifier"].(string)
					if !ok || pageID == "" {
						continue
					}
					pID := pageID
					g.Go(func() error {
						perms, err := m.sourceClient.GetPagePermissions(ctx, pID)
						if err != nil {
							mu.Lock()
							data.Warnings = append(data.Warnings, fmt.Sprintf("failed to fetch permissions for page %s: %v", pID, err))
							mu.Unlock()
							return nil
						}
						mu.Lock()
						data.PagePermissions[pID] = perms
						mu.Unlock()
						return nil
					})
				}
			}
			return nil
		})
	}

	if shouldCollect("integrations", opts.IncludeResources) {
		g.Go(func() error {
			integrations, err := m.sourceClient.GetIntegrations(ctx)
			if err != nil {
				return nil // Non-fatal
			}

			integrations = export.FilterByField(integrations, opts.Integrations, "installationId")
			mu.Lock()
			data.Integrations = integrations
			mu.Unlock()
			return nil
		})
	}

	// Wait for all goroutines to complete
	if err := g.Wait(); err != nil {
		return nil, nil, nil, err
	}

	if scopeBlueprintsToReferenced {
		// Both the schema list and the entity-streaming candidate list narrow
		// to exactly what was referenced (scorecard/action/entity match) — no
		// relation targets are pulled in here. A referenced blueprint's
		// relation target that isn't itself referenced doesn't need its
		// schema included in this migration at all; importToTarget's relation
		// validation checks the target's actual state directly instead (see
		// existingInTarget below), so an already-existing target blueprint is
		// correctly recognized without being part of this run's diff.
		data.Blueprints = export.FilterBlueprintsToReferenced(dataBlueprints, referencedBlueprintIDs)
		entityBlueprints = export.FilterBlueprintsToReferenced(entityBlueprints, referencedBlueprintIDs)
	}

	return data, entityBlueprints, cachedMatchedEntities, nil
}

// blueprintHasMatchingEntity checks whether bpID has at least one entity
// matching entityIDs (or any entity at all, when entityIDs is empty).
//
// When entityIDs is empty, this is answered with a single entities-count API
// call — no entity payload is fetched, so there's nothing to reuse later and
// nothing wasted.
//
// When entityIDs is non-empty, it must page through bpID's entities to find
// every match (not just the first — migrateEntities needs the complete
// matching set later, so stopping early would lose entries). The matches
// found are returned so the caller can cache them: migrateEntities filters
// to this same entityIDs set anyway, so a matched blueprint's entities never
// need to be fetched from the source a second time. This is safe to cache in
// full because it's bounded by len(entityIDs) (a small, caller-provided
// list), not by blueprint size — unlike the "any entity" case, which this
// function never materializes at all.
func (m *Module) blueprintHasMatchingEntity(ctx context.Context, bpID string, entityIDs []string) (bool, []api.Entity, error) {
	if len(entityIDs) == 0 {
		count, err := m.sourceClient.GetEntitiesCount(ctx, bpID)
		if err != nil {
			if strings.Contains(err.Error(), "410 Gone") {
				return false, nil, nil
			}
			return false, nil, err
		}
		return count > 0, nil, nil
	}

	entitySet := make(map[string]bool, len(entityIDs))
	for _, id := range entityIDs {
		entitySet[id] = true
	}
	var matched []api.Entity
	err := m.sourceClient.ForEachEntity(ctx, bpID, func(batch []api.Entity) error {
		for _, entity := range batch {
			id, _ := entity["identifier"].(string)
			if entitySet[id] {
				matched = append(matched, entity)
			}
		}
		return nil
	})
	if err != nil {
		if strings.Contains(err.Error(), "410 Gone") {
			return false, nil, nil
		}
		return false, nil, err
	}
	if len(matched) == 0 {
		return false, nil, nil
	}
	return true, matched, nil
}

// resolveDependencies resolves blueprint dependencies.
// If a blueprint has relations to other blueprints, ensure those blueprints are also included.
func (m *Module) resolveDependencies(allBlueprints, selectedBlueprints []api.Blueprint) []api.Blueprint {
	selectedIDs := make(map[string]bool)
	allBlueprintsMap := make(map[string]api.Blueprint)

	for _, bp := range allBlueprints {
		if identifier, ok := bp["identifier"].(string); ok {
			allBlueprintsMap[identifier] = bp
		}
	}

	for _, bp := range selectedBlueprints {
		if identifier, ok := bp["identifier"].(string); ok {
			selectedIDs[identifier] = true
		}
	}

	result := make([]api.Blueprint, len(selectedBlueprints))
	copy(result, selectedBlueprints)

	toCheck := make([]string, 0, len(selectedIDs))
	for id := range selectedIDs {
		toCheck = append(toCheck, id)
	}

	checked := make(map[string]bool)

	for len(toCheck) > 0 {
		blueprintID := toCheck[len(toCheck)-1]
		toCheck = toCheck[:len(toCheck)-1]

		if checked[blueprintID] {
			continue
		}
		checked[blueprintID] = true

		blueprint, ok := allBlueprintsMap[blueprintID]
		if !ok {
			continue
		}

		// Check relations
		relations, ok := blueprint["relations"].(map[string]interface{})
		if !ok {
			continue
		}

		for _, relation := range relations {
			relationMap, ok := relation.(map[string]interface{})
			if !ok {
				continue
			}

			target, ok := relationMap["target"].(string)
			if !ok || target == "" {
				continue
			}

			if !selectedIDs[target] {
				// Add dependency
				if depBlueprint, exists := allBlueprintsMap[target]; exists {
					result = append(result, depBlueprint)
					selectedIDs[target] = true
					toCheck = append(toCheck, target)
				}
			}
		}
	}

	return result
}

// importToTarget imports filtered data to the target organization by delegating
// to the shared import apply path.
func (m *Module) importToTarget(ctx context.Context, data *export.Data, executionPlan *plan.ExecutionPlan, diffResult *import_module.DiffResult, opts Options, skipEntityImport bool) (*Result, error) {
	importer := import_module.NewImporter(m.targetClient)
	importOpts := import_module.Options{
		SkipEntities:       skipEntityImport,
		IncludeRuleResults: opts.IncludeRuleResults,
		IncludeResources:   opts.IncludeResources,
		UsersAsDisabled:    opts.UsersAsDisabled,
	}

	importResult, err := importer.ApplyFiltered(ctx, data, diffResult, importOpts)
	if err != nil {
		return nil, err
	}

	return migrateResultFromImport(importResult, executionPlan), nil
}

func migrateResultFromImport(importResult *import_module.Result, executionPlan *plan.ExecutionPlan) *Result {
	summary := plan.Summarize(executionPlan)
	result := &Result{
		BlueprintsCreated:                    importResult.BlueprintsCreated,
		BlueprintsUpdated:                    importResult.BlueprintsUpdated,
		BlueprintsSkipped:                    summary.Skipped[resources.KindBlueprints],
		EntitiesCreated:                      importResult.EntitiesCreated,
		EntitiesUpdated:                      importResult.EntitiesUpdated,
		EntitiesSkipped:                      summary.Skipped[resources.KindEntities],
		ScorecardsCreated:                    importResult.ScorecardsCreated,
		ScorecardsUpdated:                    importResult.ScorecardsUpdated,
		ScorecardsSkipped:                    summary.Skipped[resources.KindScorecards],
		ActionsCreated:                       importResult.ActionsCreated,
		ActionsUpdated:                       importResult.ActionsUpdated,
		ActionsSkipped:                       summary.Skipped[resources.KindActions],
		TeamsCreated:                         importResult.TeamsCreated,
		TeamsUpdated:                         importResult.TeamsUpdated,
		TeamsSkipped:                         summary.Skipped[resources.KindTeams],
		UsersCreated:                         importResult.UsersCreated,
		UsersUpdated:                         importResult.UsersUpdated,
		UsersSkipped:                         summary.Skipped[resources.KindUsers],
		PagesCreated:                         importResult.PagesCreated,
		PagesUpdated:                         importResult.PagesUpdated,
		PagesSkipped:                         summary.Skipped[resources.KindPages],
		IntegrationsUpdated:                  importResult.IntegrationsUpdated,
		IntegrationsSkipped:                  summary.Skipped[resources.KindIntegrations],
		BlueprintPermissionsUpdated:          importResult.BlueprintPermissionsUpdated,
		ActionPermissionsUpdated:             importResult.ActionPermissionsUpdated,
		PagePermissionsUpdated:               importResult.PagePermissionsUpdated,
		Errors:                               importResult.Errors,
		IgnoredRuleResultTargetRelationCount: importResult.IgnoredRuleResultTargetRelationCount,
		IgnoredRuleResultTargetRelationKeys:  importResult.IgnoredRuleResultTargetRelationKeys,
	}
	for _, w := range importResult.Warnings {
		result.Warnings = append(result.Warnings, w.Message)
	}
	return result
}


// migrateEntities migrates entities for blueprints one at a time. cachedEntities
// holds, per blueprint, entities already fetched from the source during the
// AutoScopeBlueprints relevance pre-scan (see blueprintHasMatchingEntity) —
// when present for a blueprint, it's used in place of a fresh source fetch.
func (m *Module) migrateEntities(ctx context.Context, blueprints []api.Blueprint, opts Options, result *Result, dryRun bool, cachedEntities map[string][]api.Entity) error {
	if len(blueprints) == 0 {
		return nil
	}
	tempDir, err := os.MkdirTemp("", "port-cli-migrate-entities-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	entityImporter := import_module.NewImporter(m.targetClient)
	importCtx := entityImporter.NewEntityImportContext(ctx)
	importResult := &import_module.Result{}
	flushed := false
	flushImportResult := func() {
		if flushed {
			return
		}
		flushed = true
		result.EntitiesCreated += importResult.EntitiesCreated
		result.EntitiesUpdated += importResult.EntitiesUpdated
		result.Errors = append(result.Errors, entityImporter.CollectedErrors()...)
	}
	currentSource := entitystream.FromAPI(m.targetClient)
	source := entitystream.FromAPI(m.sourceClient)
	streamOpts := import_module.EntityStreamOptions{
		IncludeRuleResults: opts.IncludeRuleResults,
		EntityIDs:          opts.Entities,
		OnEntitySkipped: func(api.Entity) {
			result.EntitiesSkipped++
		},
	}

	for _, blueprint := range blueprints {
		bpID, _ := blueprint["identifier"].(string)
		if bpID == "" {
			continue
		}
		if opts.SkipSystemBlueprints && strings.HasPrefix(bpID, "_") {
			continue
		}
		var iterator entitystream.PageIterator
		if cached, ok := cachedEntities[bpID]; ok {
			iterator = entitystream.EntityIterator(0, func(yield func(api.Entity) error) error {
				for _, entity := range cached {
					if err := yield(entity); err != nil {
						return err
					}
				}
				return nil
			})
		} else {
			iterator = entitystream.BlueprintIterator(source, bpID)
		}
		if err := entityImporter.ImportBlueprintEntities(ctx, bpID, iterator, currentSource, streamOpts, importResult, dryRun, importCtx, tempDir); err != nil {
			flushImportResult()
			result.Errors = append(result.Errors, fmt.Sprintf("Entities %s: %v", bpID, err))
			return fmt.Errorf("entities %s: %w", bpID, err)
		}
	}

	flushImportResult()
	return nil
}

// Close closes both API clients.
func (m *Module) Close() error {
	var errs []error
	if m.sourceClient != nil {
		if err := m.sourceClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if m.targetClient != nil {
		if err := m.targetClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing clients: %v", errs)
	}
	return nil
}


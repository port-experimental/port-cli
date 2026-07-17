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
	"github.com/port-experimental/port-cli/internal/snapshot"
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
	Entities      []string
	Scorecards    []string
	Actions       []string
	Pages         []string
	Integrations  []string
	Teams         []string
	Users         []string
	ErrorHandling import_module.ErrorHandlingOptions
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

	executionPlan := import_module.BuildFromDiffResult(diffResult)

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
	result, err := m.importToTarget(ctx, filteredData, executionPlan, opts, streamEntities)
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
	result := &Result{
		Success:    true,
		Message:    "Migration validation passed (dry run - no changes applied)",
		DiffResult: diffResult,
	}
	populateMigrateCounters(result, plan.ApplyCountersFromSummary(summary))
	populateMigrateDryRunDetails(result, executionPlan)
	return result
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

// exportFromSource exports metadata from the source organization via snapshot.Collector
// and returns blueprints eligible for streaming entity migration, plus any entities
// already fetched for those blueprints while deciding AutoScopeBlueprints relevance.
func (m *Module) exportFromSource(ctx context.Context, opts Options) (*export.Data, []api.Blueprint, map[string][]api.Entity, error) {
	blueprintFilter, err := m.resolvedBlueprintIDs(ctx, opts.Blueprints)
	if err != nil {
		return nil, nil, nil, err
	}

	collectPlan := snapshot.MigrateCollectPlan(
		opts.IncludeRuleResults,
		opts.IncludeResources,
		opts.ExcludeBlueprints,
		opts.ExcludeBlueprintSchema,
		opts.SkipSystemBlueprints,
		opts.SkipSystemBlueprintProperties,
		opts.AutoScopeBlueprints,
		snapshot.Filters{
			Blueprints:   blueprintFilter,
			Entities:     opts.Entities,
			Scorecards:   opts.Scorecards,
			Actions:      opts.Actions,
			Pages:        opts.Pages,
			Integrations: opts.Integrations,
			Teams:        opts.Teams,
			Users:        opts.Users,
		},
	)

	snap, err := snapshot.NewCollector(m.sourceClient).Collect(ctx, "source", collectPlan)
	if err != nil {
		return nil, nil, nil, err
	}
	data := snap.Data
	data.Entities = []api.Entity{}

	m.collectTeamsAndUsers(ctx, opts, data)

	exportOpts := collectPlan.ExportOptions()
	entityBlueprints, err := export.BlueprintsForEntityStreaming(ctx, m.sourceClient, exportOpts)
	if err != nil {
		return nil, nil, nil, err
	}

	cachedMatchedEntities := make(map[string][]api.Entity)
	referenced := data.ReferencedBlueprintIDs
	if referenced == nil {
		referenced = make(map[string]bool)
	}

	scopeBlueprintsToReferenced := opts.AutoScopeBlueprints && shouldCollect("blueprints", opts.IncludeResources)
	streamEntities := !opts.SkipEntities && shouldCollect("entities", opts.IncludeResources)

	if scopeBlueprintsToReferenced && streamEntities {
		g, gCtx := errgroup.WithContext(ctx)
		sem := semaphore.NewWeighted(maxConcurrentBlueprints)
		var mu sync.Mutex
		for _, blueprint := range entityBlueprints {
			bpID, _ := blueprint["identifier"].(string)
			if bpID == "" {
				continue
			}
			bpIDCopy := bpID
			if err := sem.Acquire(gCtx, 1); err != nil {
				return nil, nil, nil, err
			}
			g.Go(func() error {
				defer sem.Release(1)
				found, matched, err := m.blueprintHasMatchingEntity(gCtx, bpIDCopy, opts.Entities)
				if err != nil {
					return fmt.Errorf("failed to check entities for blueprint %s: %w", bpIDCopy, err)
				}
				if found {
					mu.Lock()
					referenced[bpIDCopy] = true
					if len(matched) > 0 {
						cachedMatchedEntities[bpIDCopy] = matched
					}
					mu.Unlock()
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return nil, nil, nil, err
		}
	}

	if scopeBlueprintsToReferenced {
		data.Blueprints = export.FilterBlueprintsToReferenced(data.Blueprints, referenced)
		entityBlueprints = export.FilterBlueprintsToReferenced(entityBlueprints, referenced)
	}

	return data, entityBlueprints, cachedMatchedEntities, nil
}

func (m *Module) resolvedBlueprintIDs(ctx context.Context, blueprintIDs []string) ([]string, error) {
	if len(blueprintIDs) == 0 {
		return nil, nil
	}
	allBlueprints, err := m.sourceClient.GetBlueprints(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get blueprints: %w", err)
	}
	selected := export.FilterByField(allBlueprints, blueprintIDs, "identifier")
	resolved := export.ResolveBlueprintDependencies(allBlueprints, selected)
	ids := make([]string, 0, len(resolved))
	for _, bp := range resolved {
		if id, ok := bp["identifier"].(string); ok && id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (m *Module) collectTeamsAndUsers(ctx context.Context, opts Options, data *export.Data) {
	if opts.SkipEntities {
		return
	}
	if shouldCollect("teams", opts.IncludeResources) {
		teams, err := m.sourceClient.GetTeams(ctx)
		if err == nil {
			data.Teams = export.FilterByField(teams, opts.Teams, "name")
		}
	}
	if shouldCollect("users", opts.IncludeResources) {
		users, err := m.sourceClient.GetUsers(ctx)
		if err == nil {
			data.Users = export.FilterByField(users, opts.Users, "email")
		}
	}
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

// importToTarget imports filtered data to the target organization by delegating
// to the shared import apply path. Permission updates and user-update emails are
// taken from the execution plan via ApplyContext — DiffResult is not required here.
func (m *Module) importToTarget(ctx context.Context, data *export.Data, executionPlan *plan.ExecutionPlan, opts Options, skipEntityImport bool) (*Result, error) {
	importer := import_module.NewImporter(m.targetClient)
	importOpts := import_module.Options{
		SkipEntities:       skipEntityImport,
		IncludeRuleResults: opts.IncludeRuleResults,
		IncludeResources:   opts.IncludeResources,
		UsersAsDisabled:    opts.UsersAsDisabled,
		ErrorHandling:      opts.ErrorHandling,
	}

	applyCtx := import_module.ApplyContextFromPlan(executionPlan)
	importResult, err := importer.ApplyFiltered(ctx, data, applyCtx, importOpts)
	if err != nil {
		return nil, err
	}

	return migrateResultFromImport(importResult, executionPlan), nil
}

func migrateResultFromImport(importResult *import_module.Result, executionPlan *plan.ExecutionPlan) *Result {
	summary := plan.Summarize(executionPlan)
	result := &Result{
		Errors:                               importResult.Errors,
		IgnoredRuleResultTargetRelationCount: importResult.IgnoredRuleResultTargetRelationCount,
		IgnoredRuleResultTargetRelationKeys:  importResult.IgnoredRuleResultTargetRelationKeys,
	}
	populateMigrateCounters(result, import_module.ApplyCountersFromImport(importResult, summary))
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

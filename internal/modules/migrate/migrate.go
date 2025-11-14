package migrate

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/port-labs/port-cli/internal/api"
	"github.com/port-labs/port-cli/internal/config"
	"github.com/port-labs/port-cli/internal/modules/export"
	"github.com/port-labs/port-cli/internal/modules/import_module"
	"golang.org/x/sync/errgroup"
)

// Module handles migration between Port organizations.
type Module struct {
	sourceClient *api.Client
	targetClient *api.Client
}

// NewModule creates a new migration module.
func NewModule(sourceConfig, targetConfig *config.OrganizationConfig) *Module {
	return &Module{
		sourceClient: api.NewClient(sourceConfig.ClientID, sourceConfig.ClientSecret, sourceConfig.APIURL, 0),
		targetClient: api.NewClient(targetConfig.ClientID, targetConfig.ClientSecret, targetConfig.APIURL, 0),
	}
}

// Options represents migration options.
type Options struct {
	Blueprints       []string
	DryRun           bool
	SkipEntities     bool
	IncludeResources []string
}

// Result represents the result of a migration operation.
type Result struct {
	Success             bool
	Message             string
	BlueprintsCreated   int
	BlueprintsUpdated   int
	BlueprintsSkipped   int
	EntitiesCreated     int
	EntitiesUpdated     int
	EntitiesSkipped     int
	ScorecardsCreated   int
	ScorecardsUpdated   int
	ScorecardsSkipped   int
	ActionsCreated      int
	ActionsUpdated      int
	ActionsSkipped      int
	TeamsCreated        int
	TeamsUpdated        int
	TeamsSkipped        int
	UsersCreated        int
	UsersUpdated        int
	UsersSkipped        int
	PagesCreated        int
	PagesUpdated        int
	PagesSkipped        int
	IntegrationsUpdated int
	IntegrationsSkipped int
	Errors              []string
	DiffResult          *import_module.DiffResult
}

// Execute performs the migration operation.
func (m *Module) Execute(ctx context.Context, opts Options) (*Result, error) {
	// Export from source
	sourceData, err := m.exportFromSource(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to export from source: %w", err)
	}

	// Diff validation - compare source data with target organization's current state
	comparer := import_module.NewDiffComparer(m.targetClient)
	diffOpts := import_module.Options{
		SkipEntities:     opts.SkipEntities,
		IncludeResources: opts.IncludeResources,
	}
	diffResult, err := comparer.Compare(ctx, sourceData, diffOpts)
	if err != nil {
		return nil, fmt.Errorf("diff comparison failed: %w", err)
	}

	// Use diff result to filter data - only migrate what needs to be created or updated
	filteredData := diffResult.FilterData(sourceData)

	// Dry run - show what would happen
	if opts.DryRun {
		return m.generateDryRunResult(diffResult), nil
	}

	// Import to target using filtered data
	result, err := m.importToTarget(ctx, filteredData, diffResult)
	if err != nil {
		return nil, fmt.Errorf("failed to import to target: %w", err)
	}

	result.Success = true
	result.Message = "Migration completed successfully"
	result.DiffResult = diffResult
	return result, nil
}

// generateDryRunResult generates a dry run result with accurate predictions.
func (m *Module) generateDryRunResult(diffResult *import_module.DiffResult) *Result {
	return &Result{
		Success:             true,
		Message:             "Migration validation passed (dry run - no changes applied)",
		BlueprintsCreated:   len(diffResult.BlueprintsToCreate),
		BlueprintsUpdated:   len(diffResult.BlueprintsToUpdate),
		BlueprintsSkipped:   len(diffResult.BlueprintsToSkip),
		EntitiesCreated:     len(diffResult.EntitiesToCreate),
		EntitiesUpdated:     len(diffResult.EntitiesToUpdate),
		EntitiesSkipped:     len(diffResult.EntitiesToSkip),
		ScorecardsCreated:   len(diffResult.ScorecardsToCreate),
		ScorecardsUpdated:   len(diffResult.ScorecardsToUpdate),
		ScorecardsSkipped:   len(diffResult.ScorecardsToSkip),
		ActionsCreated:      len(diffResult.ActionsToCreate),
		ActionsUpdated:      len(diffResult.ActionsToUpdate),
		ActionsSkipped:      len(diffResult.ActionsToSkip),
		TeamsCreated:        len(diffResult.TeamsToCreate),
		TeamsUpdated:        len(diffResult.TeamsToUpdate),
		TeamsSkipped:        len(diffResult.TeamsToSkip),
		UsersCreated:        len(diffResult.UsersToCreate),
		UsersUpdated:        len(diffResult.UsersToUpdate),
		UsersSkipped:        len(diffResult.UsersToSkip),
		PagesCreated:        len(diffResult.PagesToCreate),
		PagesUpdated:        len(diffResult.PagesToUpdate),
		PagesSkipped:        len(diffResult.PagesToSkip),
		IntegrationsUpdated: len(diffResult.IntegrationsToUpdate),
		IntegrationsSkipped: len(diffResult.IntegrationsToSkip),
		DiffResult:          diffResult,
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

// exportFromSource exports data from the source organization.
func (m *Module) exportFromSource(ctx context.Context, opts Options) (*export.Data, error) {
	// Collect blueprints first
	allBlueprints, err := m.sourceClient.GetBlueprints(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get blueprints: %w", err)
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

	data := &export.Data{
		Blueprints:   resolvedBlueprints,
		Entities:     []api.Entity{},
		Scorecards:   []api.Scorecard{},
		Actions:      []api.Action{},
		Teams:        []api.Team{},
		Users:        []api.User{},
		Pages:        []api.Page{},
		Integrations: []api.Integration{},
	}

	// Use errgroup for concurrent collection
	g, ctx := errgroup.WithContext(ctx)
	var mu sync.Mutex

	// Collect entities, scorecards, and actions concurrently per blueprint
	for _, blueprint := range resolvedBlueprints {
		bp := blueprint
		bpID, ok := bp["identifier"].(string)
		if !ok {
			continue
		}

		// Collect entities
		if !opts.SkipEntities && shouldCollect("entities", opts.IncludeResources) {
			g.Go(func() error {
				entities, err := m.sourceClient.GetEntities(ctx, bpID, nil)
				if err != nil {
					if !strings.Contains(err.Error(), "410 Gone") {
						return fmt.Errorf("failed to get entities for blueprint %s: %w", bpID, err)
					}
					return nil
				}

				mu.Lock()
				data.Entities = append(data.Entities, entities...)
				mu.Unlock()
				return nil
			})
		}

		// Collect scorecards
		if shouldCollect("scorecards", opts.IncludeResources) {
			g.Go(func() error {
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

				mu.Lock()
				data.Scorecards = append(data.Scorecards, scorecards...)
				mu.Unlock()
				return nil
			})
		}

		// Collect actions
		if shouldCollect("actions", opts.IncludeResources) {
			g.Go(func() error {
				actions, err := m.sourceClient.GetActions(ctx, bpID)
				if err != nil {
					if !strings.Contains(err.Error(), "410 Gone") {
						return fmt.Errorf("failed to get actions for blueprint %s: %w", bpID, err)
					}
					return nil
				}

				mu.Lock()
				data.Actions = append(data.Actions, actions...)
				mu.Unlock()
				return nil
			})
		}
	}

	// Collect organization-wide resources
	if shouldCollect("teams", opts.IncludeResources) {
		g.Go(func() error {
			teams, err := m.sourceClient.GetTeams(ctx)
			if err != nil {
				return nil // Non-fatal
			}

			mu.Lock()
			data.Teams = teams
			mu.Unlock()
			return nil
		})
	}

	if shouldCollect("users", opts.IncludeResources) {
		g.Go(func() error {
			users, err := m.sourceClient.GetUsers(ctx)
			if err != nil {
				return nil // Non-fatal
			}

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

			mu.Lock()
			data.Actions = append(data.Actions, allActions...)
			mu.Unlock()
			return nil
		})
	}

	if shouldCollect("pages", opts.IncludeResources) {
		g.Go(func() error {
			pages, err := m.sourceClient.GetPages(ctx)
			if err != nil {
				return nil // Non-fatal
			}

			mu.Lock()
			data.Pages = pages
			mu.Unlock()
			return nil
		})
	}

	if shouldCollect("integrations", opts.IncludeResources) {
		g.Go(func() error {
			integrations, err := m.sourceClient.GetIntegrations(ctx)
			if err != nil {
				return nil // Non-fatal
			}

			mu.Lock()
			data.Integrations = integrations
			mu.Unlock()
			return nil
		})
	}

	// Wait for all goroutines to complete
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return data, nil
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

// importToTarget imports data to the target organization using diff result.
func (m *Module) importToTarget(ctx context.Context, data *export.Data, diffResult *import_module.DiffResult) (*Result, error) {
	result := &Result{
		Errors: []string{},
	}

	// Create maps to quickly check if items should be created or updated
	blueprintsToCreate := make(map[string]bool)
	blueprintsToUpdate := make(map[string]bool)
	for _, bp := range diffResult.BlueprintsToCreate {
		if id, ok := bp["identifier"].(string); ok {
			blueprintsToCreate[id] = true
		}
	}
	for _, bp := range diffResult.BlueprintsToUpdate {
		if id, ok := bp["identifier"].(string); ok {
			blueprintsToUpdate[id] = true
		}
	}

	entitiesToCreate := make(map[string]bool)
	entitiesToUpdate := make(map[string]bool)
	for _, ent := range diffResult.EntitiesToCreate {
		bpID, ok1 := ent["blueprint"].(string)
		entID, ok2 := ent["identifier"].(string)
		if ok1 && ok2 {
			entitiesToCreate[fmt.Sprintf("%s:%s", bpID, entID)] = true
		}
	}
	for _, ent := range diffResult.EntitiesToUpdate {
		bpID, ok1 := ent["blueprint"].(string)
		entID, ok2 := ent["identifier"].(string)
		if ok1 && ok2 {
			entitiesToUpdate[fmt.Sprintf("%s:%s", bpID, entID)] = true
		}
	}

	// Import blueprints first (needed for other resources) using two-pass strategy
	g, ctx := errgroup.WithContext(ctx)
	var mu sync.Mutex

	// Store relations for each blueprint before stripping
	blueprintRelations := make(map[string]map[string]interface{})
	strippedBlueprints := make([]api.Blueprint, 0, len(data.Blueprints))
	blueprintActions := make(map[string]string) // "create" or "update"

	for _, blueprint := range data.Blueprints {
		identifier, ok := blueprint["identifier"].(string)
		if !ok || identifier == "" {
			continue
		}

		// Only process blueprints that need to be created or updated
		shouldProcess := blueprintsToCreate[identifier] || blueprintsToUpdate[identifier]
		if !shouldProcess {
			continue
		}

		// Extract and store relations
		relations := import_module.ExtractRelations(blueprint)
		if relations != nil && len(relations) > 0 {
			blueprintRelations[identifier] = relations
		}

		// Strip relations for first pass
		strippedBp := import_module.StripRelations(blueprint)
		strippedBlueprints = append(strippedBlueprints, strippedBp)

		// Track what action to take
		if blueprintsToCreate[identifier] {
			blueprintActions[identifier] = "create"
		} else {
			blueprintActions[identifier] = "update"
		}
	}

	// First pass: Import blueprints without relations
	failedBlueprints := make(map[string]api.Blueprint)
	failedBlueprintActions := make(map[string]string)
	successfulBlueprints := make(map[string]bool)

	for _, blueprint := range strippedBlueprints {
		bp := blueprint
		g.Go(func() error {
			identifier, ok := bp["identifier"].(string)
			if !ok || identifier == "" {
				return nil
			}

			apiBp := api.Blueprint(bp)
			action := blueprintActions[identifier]

			if action == "create" {
				_, err := m.targetClient.CreateBlueprint(ctx, apiBp)
				if err != nil {
					mu.Lock()
					// Check if it's a relation error - if so, we'll retry in second pass
					if import_module.IsRelationError(err) {
						failedBlueprints[identifier] = bp
						failedBlueprintActions[identifier] = action
					} else {
						result.Errors = append(result.Errors, fmt.Sprintf("Blueprint %s: %v", identifier, err))
					}
					mu.Unlock()
					return nil
				}
				mu.Lock()
				result.BlueprintsCreated++
				successfulBlueprints[identifier] = true
				mu.Unlock()
			} else if action == "update" {
				_, err := m.targetClient.UpdateBlueprint(ctx, identifier, apiBp)
				if err != nil {
					mu.Lock()
					// Check if it's a relation error - if so, we'll retry in second pass
					if import_module.IsRelationError(err) {
						failedBlueprints[identifier] = bp
						failedBlueprintActions[identifier] = action
					} else {
						result.Errors = append(result.Errors, fmt.Sprintf("Blueprint %s: %v", identifier, err))
					}
					mu.Unlock()
					return nil
				}
				mu.Lock()
				result.BlueprintsUpdated++
				successfulBlueprints[identifier] = true
				mu.Unlock()
			}
			return nil
		})
	}

	// Wait for first pass to complete
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Retry failed blueprints (they might have succeeded now that dependencies exist)
	if len(failedBlueprints) > 0 {
		g, ctx = errgroup.WithContext(ctx)
		for identifier, bp := range failedBlueprints {
			bpID := identifier
			bpCopy := bp
			action := failedBlueprintActions[bpID]
			g.Go(func() error {
				apiBp := api.Blueprint(bpCopy)
				if action == "create" {
					_, err := m.targetClient.CreateBlueprint(ctx, apiBp)
					if err != nil {
						mu.Lock()
						result.Errors = append(result.Errors, fmt.Sprintf("Blueprint %s: %v", bpID, err))
						mu.Unlock()
						return nil
					}
					mu.Lock()
					result.BlueprintsCreated++
					successfulBlueprints[bpID] = true
					mu.Unlock()
				} else if action == "update" {
					_, err := m.targetClient.UpdateBlueprint(ctx, bpID, apiBp)
					if err != nil {
						mu.Lock()
						result.Errors = append(result.Errors, fmt.Sprintf("Blueprint %s: %v", bpID, err))
						mu.Unlock()
						return nil
					}
					mu.Lock()
					result.BlueprintsUpdated++
					successfulBlueprints[bpID] = true
					mu.Unlock()
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return nil, err
		}
	}

	// Second pass: Update blueprints with relations
	if len(blueprintRelations) > 0 {
		g, ctx = errgroup.WithContext(ctx)
		for identifier, relations := range blueprintRelations {
			// Only update blueprints that were successfully created/updated
			if !successfulBlueprints[identifier] {
				continue
			}

			bpID := identifier
			rels := relations
			g.Go(func() error {
				// Validate that relation targets exist before attempting update
				// This helps provide better error messages
				missingTargets := import_module.ValidateRelationTargets(api.Blueprint{"relations": rels}, successfulBlueprints)
				if len(missingTargets) > 0 {
					mu.Lock()
					result.Errors = append(result.Errors, fmt.Sprintf("Blueprint %s: cannot add relations - missing target blueprints: %v", bpID, missingTargets))
					mu.Unlock()
					return nil
				}

				// Fetch existing blueprint first to avoid overwriting other fields
				existingBp, err := m.targetClient.GetBlueprint(ctx, bpID)
				if err != nil {
					mu.Lock()
					result.Errors = append(result.Errors, fmt.Sprintf("Blueprint %s: failed to fetch for relation update: %v", bpID, err))
					mu.Unlock()
					return nil
				}

				// Merge relations into existing blueprint
				existingBp["relations"] = rels
				updateBp := api.Blueprint(existingBp)

				_, err = m.targetClient.UpdateBlueprint(ctx, bpID, updateBp)
				if err != nil {
					mu.Lock()
					// Don't fail the entire import if relation update fails
					// The blueprint was created successfully, relations can be added later
					result.Errors = append(result.Errors, fmt.Sprintf("Blueprint %s: failed to add relations: %v", bpID, err))
					mu.Unlock()
					return nil
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return nil, err
		}
	}

	// Import other resources concurrently
	g, ctx = errgroup.WithContext(ctx)

	// Import entities
	for _, entity := range data.Entities {
		ent := entity
		g.Go(func() error {
			blueprintID, ok1 := ent["blueprint"].(string)
			entityID, ok2 := ent["identifier"].(string)
			if !ok1 || !ok2 || blueprintID == "" || entityID == "" {
				return nil
			}

			key := fmt.Sprintf("%s:%s", blueprintID, entityID)

			if entitiesToCreate[key] {
				_, err := m.targetClient.CreateEntity(ctx, blueprintID, ent)
				if err != nil {
					mu.Lock()
					result.Errors = append(result.Errors, fmt.Sprintf("Entity %s: %v", entityID, err))
					mu.Unlock()
					return nil
				}
				mu.Lock()
				result.EntitiesCreated++
				mu.Unlock()
			} else if entitiesToUpdate[key] {
				_, err := m.targetClient.UpdateEntity(ctx, blueprintID, entityID, ent)
				if err != nil {
					mu.Lock()
					result.Errors = append(result.Errors, fmt.Sprintf("Entity %s: %v", entityID, err))
					mu.Unlock()
					return nil
				}
				mu.Lock()
				result.EntitiesUpdated++
				mu.Unlock()
			}
			return nil
		})
	}

	// Import scorecards - group by blueprint for bulk updates
	scorecardsToCreate := make(map[string]bool)
	scorecardsToUpdate := make(map[string]bool)
	for _, sc := range diffResult.ScorecardsToCreate {
		bpID, ok1 := sc["blueprintIdentifier"].(string)
		scID, ok2 := sc["identifier"].(string)
		if ok1 && ok2 {
			scorecardsToCreate[fmt.Sprintf("%s:%s", bpID, scID)] = true
		}
	}
	for _, sc := range diffResult.ScorecardsToUpdate {
		bpID, ok1 := sc["blueprintIdentifier"].(string)
		scID, ok2 := sc["identifier"].(string)
		if ok1 && ok2 {
			scorecardsToUpdate[fmt.Sprintf("%s:%s", bpID, scID)] = true
		}
	}

	// Group scorecards by blueprint for bulk operations
	scorecardsByBlueprint := make(map[string][]api.Scorecard)
	for _, scorecard := range data.Scorecards {
		sc := scorecard
		blueprintID, ok1 := sc["blueprintIdentifier"].(string)
		scorecardID, ok2 := sc["identifier"].(string)
		if !ok1 || !ok2 || blueprintID == "" || scorecardID == "" {
			continue
		}

		key := fmt.Sprintf("%s:%s", blueprintID, scorecardID)
		// Only include scorecards that need to be created or updated
		if scorecardsToCreate[key] || scorecardsToUpdate[key] {
			apiSc := api.Scorecard(sc)
			scorecardsByBlueprint[blueprintID] = append(scorecardsByBlueprint[blueprintID], apiSc)
		}
	}

	// Process scorecards grouped by blueprint
	for blueprintID, scorecards := range scorecardsByBlueprint {
		bpID := blueprintID
		scs := scorecards
		g.Go(func() error {
			// Separate create and update operations
			toCreate := []api.Scorecard{}
			toUpdate := []api.Scorecard{}

			for _, sc := range scs {
				scID, ok := sc["identifier"].(string)
				if !ok || scID == "" {
					continue
				}
				key := fmt.Sprintf("%s:%s", bpID, scID)
				if scorecardsToCreate[key] {
					toCreate = append(toCreate, sc)
				} else if scorecardsToUpdate[key] {
					toUpdate = append(toUpdate, sc)
				}
			}

			// Create new scorecards individually
			for _, sc := range toCreate {
				_, err := m.targetClient.CreateScorecard(ctx, bpID, sc)
				if err != nil {
					mu.Lock()
					scID, _ := sc["identifier"].(string)
					result.Errors = append(result.Errors, fmt.Sprintf("Scorecard %s: %v", scID, err))
					mu.Unlock()
					continue
				}
				mu.Lock()
				result.ScorecardsCreated++
				mu.Unlock()
			}

			// Update existing scorecards using bulk PUT endpoint
			if len(toUpdate) > 0 {
				_, err := m.targetClient.UpdateScorecards(ctx, bpID, toUpdate)
				if err != nil {
					mu.Lock()
					result.Errors = append(result.Errors, fmt.Sprintf("Scorecards for blueprint %s: %v", bpID, err))
					mu.Unlock()
					return nil
				}
				mu.Lock()
				result.ScorecardsUpdated += len(toUpdate)
				mu.Unlock()
			}

			return nil
		})
	}

	// Import actions
	actionsToCreate := make(map[string]bool)
	actionsToUpdate := make(map[string]bool)
	for _, act := range diffResult.ActionsToCreate {
		if id, ok := act["identifier"].(string); ok {
			actionsToCreate[id] = true
		}
	}
	for _, act := range diffResult.ActionsToUpdate {
		if id, ok := act["identifier"].(string); ok {
			actionsToUpdate[id] = true
		}
	}

	for _, action := range data.Actions {
		act := action
		g.Go(func() error {
			identifier, ok := act["identifier"].(string)
			if !ok || identifier == "" {
				return nil
			}

			apiAction := api.Automation(act)

			if actionsToCreate[identifier] {
				_, err := m.targetClient.CreateAutomation(ctx, apiAction)
				if err != nil {
					mu.Lock()
					result.Errors = append(result.Errors, fmt.Sprintf("Action %s: %v", identifier, err))
					mu.Unlock()
					return nil
				}
				mu.Lock()
				result.ActionsCreated++
				mu.Unlock()
			} else if actionsToUpdate[identifier] {
				_, err := m.targetClient.UpdateAutomation(ctx, identifier, apiAction)
				if err != nil {
					mu.Lock()
					result.Errors = append(result.Errors, fmt.Sprintf("Action %s: %v", identifier, err))
					mu.Unlock()
					return nil
				}
				mu.Lock()
				result.ActionsUpdated++
				mu.Unlock()
			}
			return nil
		})
	}

	// Import teams
	teamsToCreate := make(map[string]bool)
	teamsToUpdate := make(map[string]bool)
	for _, t := range diffResult.TeamsToCreate {
		if name, ok := t["name"].(string); ok {
			teamsToCreate[name] = true
		}
	}
	for _, t := range diffResult.TeamsToUpdate {
		if name, ok := t["name"].(string); ok {
			teamsToUpdate[name] = true
		}
	}

	for _, team := range data.Teams {
		t := team
		g.Go(func() error {
			teamName, ok := t["name"].(string)
			if !ok || teamName == "" {
				return nil
			}

			apiTeam := api.Team(t)

			if teamsToCreate[teamName] {
				_, err := m.targetClient.CreateTeam(ctx, apiTeam)
				if err != nil {
					mu.Lock()
					result.Errors = append(result.Errors, fmt.Sprintf("Team %s: %v", teamName, err))
					mu.Unlock()
					return nil
				}
				mu.Lock()
				result.TeamsCreated++
				mu.Unlock()
			} else if teamsToUpdate[teamName] {
				_, err := m.targetClient.UpdateTeam(ctx, teamName, apiTeam)
				if err != nil {
					mu.Lock()
					result.Errors = append(result.Errors, fmt.Sprintf("Team %s: %v", teamName, err))
					mu.Unlock()
					return nil
				}
				mu.Lock()
				result.TeamsUpdated++
				mu.Unlock()
			}
			return nil
		})
	}

	// Import users
	usersToCreate := make(map[string]bool)
	usersToUpdate := make(map[string]bool)
	for _, u := range diffResult.UsersToCreate {
		if email, ok := u["email"].(string); ok {
			usersToCreate[email] = true
		}
	}
	for _, u := range diffResult.UsersToUpdate {
		if email, ok := u["email"].(string); ok {
			usersToUpdate[email] = true
		}
	}

	for _, user := range data.Users {
		u := user
		g.Go(func() error {
			userEmail, ok := u["email"].(string)
			if !ok || userEmail == "" {
				return nil
			}

			apiUser := api.User(u)

			if usersToCreate[userEmail] {
				_, err := m.targetClient.InviteUser(ctx, apiUser)
				if err != nil {
					mu.Lock()
					result.Errors = append(result.Errors, fmt.Sprintf("User %s: %v", userEmail, err))
					mu.Unlock()
					return nil
				}
				mu.Lock()
				result.UsersCreated++
				mu.Unlock()
			} else if usersToUpdate[userEmail] {
				_, err := m.targetClient.UpdateUser(ctx, userEmail, apiUser)
				if err != nil {
					mu.Lock()
					result.Errors = append(result.Errors, fmt.Sprintf("User %s: %v", userEmail, err))
					mu.Unlock()
					return nil
				}
				mu.Lock()
				result.UsersUpdated++
				mu.Unlock()
			}
			return nil
		})
	}

	// Import pages
	pagesToCreate := make(map[string]bool)
	pagesToUpdate := make(map[string]bool)
	for _, p := range diffResult.PagesToCreate {
		if id, ok := p["identifier"].(string); ok {
			pagesToCreate[id] = true
		}
	}
	for _, p := range diffResult.PagesToUpdate {
		if id, ok := p["identifier"].(string); ok {
			pagesToUpdate[id] = true
		}
	}

	for _, page := range data.Pages {
		p := page
		g.Go(func() error {
			pageID, ok := p["identifier"].(string)
			if !ok || pageID == "" {
				return nil
			}

			apiPage := api.Page(p)

			if pagesToCreate[pageID] {
				_, err := m.targetClient.CreatePage(ctx, apiPage)
				if err != nil {
					mu.Lock()
					result.Errors = append(result.Errors, fmt.Sprintf("Page %s: %v", pageID, err))
					mu.Unlock()
					return nil
				}
				mu.Lock()
				result.PagesCreated++
				mu.Unlock()
			} else if pagesToUpdate[pageID] {
				_, err := m.targetClient.UpdatePage(ctx, pageID, apiPage)
				if err != nil {
					mu.Lock()
					result.Errors = append(result.Errors, fmt.Sprintf("Page %s: %v", pageID, err))
					mu.Unlock()
					return nil
				}
				mu.Lock()
				result.PagesUpdated++
				mu.Unlock()
			}
			return nil
		})
	}

	// Import integrations
	integrationsToUpdate := make(map[string]bool)
	for _, integ := range diffResult.IntegrationsToUpdate {
		if id, ok := integ["identifier"].(string); ok {
			integrationsToUpdate[id] = true
		}
	}

	for _, integration := range data.Integrations {
		integ := integration
		g.Go(func() error {
			integrationID, ok := integ["identifier"].(string)
			if !ok || integrationID == "" {
				return nil
			}

			if integrationsToUpdate[integrationID] {
				configMap := make(map[string]interface{})
				for k, v := range integ {
					configMap[k] = v
				}

				_, err := m.targetClient.UpdateIntegrationConfig(ctx, integrationID, configMap)
				if err != nil {
					mu.Lock()
					result.Errors = append(result.Errors, fmt.Sprintf("Integration %s: %v", integrationID, err))
					mu.Unlock()
					return nil
				}
				mu.Lock()
				result.IntegrationsUpdated++
				mu.Unlock()
			}
			return nil
		})
	}

	// Wait for all imports to complete
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Set skipped counts from diff result
	result.BlueprintsSkipped = len(diffResult.BlueprintsToSkip)
	result.EntitiesSkipped = len(diffResult.EntitiesToSkip)
	result.ScorecardsSkipped = len(diffResult.ScorecardsToSkip)
	result.ActionsSkipped = len(diffResult.ActionsToSkip)
	result.TeamsSkipped = len(diffResult.TeamsToSkip)
	result.UsersSkipped = len(diffResult.UsersToSkip)
	result.PagesSkipped = len(diffResult.PagesToSkip)
	result.IntegrationsSkipped = len(diffResult.IntegrationsToSkip)

	return result, nil
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

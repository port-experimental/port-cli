package import_module

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/config"
	"github.com/port-experimental/port-cli/internal/modules/export"
	"golang.org/x/sync/errgroup"
)

// Module handles importing data to Port.
type Module struct {
	client *api.Client
}

// NewModule creates a new import module.
func NewModule(orgConfig *config.OrganizationConfig) *Module {
	client := api.NewClient(orgConfig.ClientID, orgConfig.ClientSecret, orgConfig.APIURL, 0)
	return &Module{
		client: client,
	}
}

// Options represents import options.
type Options struct {
	InputPath        string
	DryRun           bool
	SkipEntities     bool
	IncludeResources []string
}

// Result represents the result of an import operation.
type Result struct {
	Success             bool
	Message             string
	BlueprintsCreated   int
	BlueprintsUpdated   int
	EntitiesCreated     int
	EntitiesUpdated     int
	ScorecardsCreated   int
	ScorecardsUpdated   int
	ActionsCreated      int
	ActionsUpdated      int
	TeamsCreated        int
	TeamsUpdated        int
	UsersCreated        int
	UsersUpdated        int
	PagesCreated        int
	PagesUpdated        int
	IntegrationsUpdated int
	Errors              []string
	DiffResult          *DiffResult
}

// Execute performs the import operation.
func (m *Module) Execute(ctx context.Context, opts Options) (*Result, error) {
	// Load data
	loader := NewLoader()
	data, err := loader.LoadData(opts.InputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load data: %w", err)
	}

	// Validate data
	if err := loader.ValidateData(data); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Diff validation (always enabled)
	comparer := NewDiffComparer(m.client)
	diffResult, err := comparer.Compare(ctx, data, opts)
	if err != nil {
		return nil, fmt.Errorf("diff comparison failed: %w", err)
	}

	// Use diff result to filter data
	data = diffResult.FilterData(data)

	// Dry run - show what would happen
	if opts.DryRun {
		return m.generateDryRunResult(data, diffResult, opts), nil
	}

	// Import data concurrently
	importer := NewImporter(m.client)
	result, err := importer.Import(ctx, data, opts)
	if err != nil {
		return nil, fmt.Errorf("import failed: %w", err)
	}

	result.Success = true
	result.Message = "Successfully imported data"
	result.DiffResult = diffResult
	return result, nil
}

// generateDryRunResult generates a dry run result with accurate predictions.
func (m *Module) generateDryRunResult(data *export.Data, diffResult *DiffResult, opts Options) *Result {
	if diffResult != nil {
		// Use diff result for accurate counts
		return &Result{
			Success:             true,
			Message:             "Validation passed (dry run - no changes applied)",
			BlueprintsCreated:   len(diffResult.BlueprintsToCreate),
			BlueprintsUpdated:   len(diffResult.BlueprintsToUpdate),
			EntitiesCreated:     len(diffResult.EntitiesToCreate),
			EntitiesUpdated:     len(diffResult.EntitiesToUpdate),
			ScorecardsCreated:   len(diffResult.ScorecardsToCreate),
			ScorecardsUpdated:   len(diffResult.ScorecardsToUpdate),
			ActionsCreated:      len(diffResult.ActionsToCreate),
			ActionsUpdated:      len(diffResult.ActionsToUpdate),
			TeamsCreated:        len(diffResult.TeamsToCreate),
			TeamsUpdated:        len(diffResult.TeamsToUpdate),
			UsersCreated:        len(diffResult.UsersToCreate),
			UsersUpdated:        len(diffResult.UsersToUpdate),
			PagesCreated:        len(diffResult.PagesToCreate),
			PagesUpdated:        len(diffResult.PagesToUpdate),
			IntegrationsUpdated: len(diffResult.IntegrationsToUpdate),
			DiffResult:          diffResult,
		}
	}

	// Fallback to old behavior
	return &Result{
		Success:           true,
		Message:           "Validation passed (dry run - no changes applied)",
		BlueprintsCreated: len(data.Blueprints),
		EntitiesCreated:   len(data.Entities),
	}
}

// Close closes the API client.
func (m *Module) Close() error {
	if m.client != nil {
		return m.client.Close()
	}
	return nil
}

// shouldImport checks if a resource type should be imported.
func shouldImport(resourceType string, includeResources []string) bool {
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

// cleanSystemFields removes system fields that shouldn't be sent to API.
func cleanSystemFields(resource map[string]interface{}, fieldsToRemove []string) map[string]interface{} {
	cleaned := make(map[string]interface{})
	removeSet := make(map[string]bool)
	for _, f := range fieldsToRemove {
		removeSet[f] = true
	}

	for k, v := range resource {
		if !removeSet[k] {
			cleaned[k] = v
		}
	}

	return cleaned
}

// isConflictError checks if an error is a conflict (409) error.
func isConflictError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "409") || strings.Contains(errStr, "Conflict")
}

// Importer handles concurrent importing of data.
type Importer struct {
	client *api.Client
}

// NewImporter creates a new importer.
func NewImporter(client *api.Client) *Importer {
	return &Importer{
		client: client,
	}
}

// Import imports data to Port concurrently.
func (i *Importer) Import(ctx context.Context, data *export.Data, opts Options) (*Result, error) {
	result := &Result{
		Errors: []string{},
	}

	g, ctx := errgroup.WithContext(ctx)
	var mu sync.Mutex

	// Import blueprints first (needed for other resources) using two-pass strategy
	if shouldImport("blueprints", opts.IncludeResources) {
		// Store relations for each blueprint before stripping
		blueprintRelations := make(map[string]map[string]interface{})
		strippedBlueprints := make([]api.Blueprint, 0, len(data.Blueprints))

		for _, blueprint := range data.Blueprints {
			identifier, ok := blueprint["identifier"].(string)
			if !ok || identifier == "" {
				continue
			}

			// Skip system blueprints
			if strings.HasPrefix(identifier, "_") {
				continue
			}

			// Extract and store relations
			relations := ExtractRelations(blueprint)
			if len(relations) > 0 {
				blueprintRelations[identifier] = relations
			}

			// Strip relations for first pass
			strippedBp := StripRelations(blueprint)
			strippedBlueprints = append(strippedBlueprints, strippedBp)
		}

		// First pass: Import blueprints without relations
		failedBlueprints := make(map[string]api.Blueprint)
		successfulBlueprints := make(map[string]bool)

		for _, blueprint := range strippedBlueprints {
			bp := blueprint
			g.Go(func() error {
				identifier, ok := bp["identifier"].(string)
				if !ok || identifier == "" {
					return nil
				}

				// Convert to API type
				apiBp := api.Blueprint(bp)

				// Try create first
				_, err := i.client.CreateBlueprint(ctx, apiBp)
				if err == nil {
					mu.Lock()
					result.BlueprintsCreated++
					successfulBlueprints[identifier] = true
					mu.Unlock()
					return nil
				}

				// If conflict, try update
				if isConflictError(err) {
					_, updateErr := i.client.UpdateBlueprint(ctx, identifier, apiBp)
					if updateErr != nil {
						mu.Lock()
						// Check if it's a relation error - if so, we'll retry in second pass
						if IsRelationError(updateErr) {
							failedBlueprints[identifier] = bp
						} else {
							result.Errors = append(result.Errors, fmt.Sprintf("Blueprint %s: %v", identifier, updateErr))
						}
						mu.Unlock()
						return nil
					}
					mu.Lock()
					result.BlueprintsUpdated++
					successfulBlueprints[identifier] = true
					mu.Unlock()
					return nil
				}

				// Check if it's a relation error - if so, we'll retry in second pass
				mu.Lock()
				if IsRelationError(err) {
					failedBlueprints[identifier] = bp
				} else {
					result.Errors = append(result.Errors, fmt.Sprintf("Blueprint %s: %v", identifier, err))
				}
				mu.Unlock()
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
				g.Go(func() error {
					apiBp := api.Blueprint(bpCopy)
					_, err := i.client.CreateBlueprint(ctx, apiBp)
					if err == nil {
						mu.Lock()
						result.BlueprintsCreated++
						successfulBlueprints[bpID] = true
						mu.Unlock()
						return nil
					}

					if isConflictError(err) {
						_, updateErr := i.client.UpdateBlueprint(ctx, bpID, apiBp)
						if updateErr != nil {
							mu.Lock()
							// Check if it's still a relation error - if so, log it but don't fail completely
							if IsRelationError(updateErr) {
								result.Errors = append(result.Errors, fmt.Sprintf("Blueprint %s: relation target still missing after retry: %v", bpID, updateErr))
							} else {
								result.Errors = append(result.Errors, fmt.Sprintf("Blueprint %s: %v", bpID, updateErr))
							}
							mu.Unlock()
							return nil
						}
						mu.Lock()
						result.BlueprintsUpdated++
						successfulBlueprints[bpID] = true
						mu.Unlock()
						return nil
					}

					// Check if it's still a relation error
					mu.Lock()
					if IsRelationError(err) {
						result.Errors = append(result.Errors, fmt.Sprintf("Blueprint %s: relation target still missing after retry: %v", bpID, err))
					} else {
						result.Errors = append(result.Errors, fmt.Sprintf("Blueprint %s: %v", bpID, err))
					}
					mu.Unlock()
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
					missingTargets := ValidateRelationTargets(api.Blueprint{"relations": rels}, successfulBlueprints)
					if len(missingTargets) > 0 {
						mu.Lock()
						result.Errors = append(result.Errors, fmt.Sprintf("Blueprint %s: cannot add relations - missing target blueprints: %v", bpID, missingTargets))
						mu.Unlock()
						return nil
					}

					// Fetch existing blueprint first to avoid overwriting other fields
					existingBp, err := i.client.GetBlueprint(ctx, bpID)
					if err != nil {
						mu.Lock()
						result.Errors = append(result.Errors, fmt.Sprintf("Blueprint %s: failed to fetch for relation update: %v", bpID, err))
						mu.Unlock()
						return nil
					}

					// Merge relations into existing blueprint
					existingBp["relations"] = rels
					updateBp := api.Blueprint(existingBp)

					_, err = i.client.UpdateBlueprint(ctx, bpID, updateBp)
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
	}

	// Wait for blueprints to complete before importing dependent resources
	// (This is already handled above, but keeping for clarity)

	// Import other resources concurrently
	g, ctx = errgroup.WithContext(ctx)

	// Import entities
	if !opts.SkipEntities && shouldImport("entities", opts.IncludeResources) {
		for _, entity := range data.Entities {
			ent := entity
			g.Go(func() error {
				blueprintID, ok1 := ent["blueprint"].(string)
				entityID, ok2 := ent["identifier"].(string)
				if !ok1 || !ok2 || blueprintID == "" || entityID == "" {
					return nil
				}

				apiEntity := api.Entity(ent)

				_, err := i.client.CreateEntity(ctx, blueprintID, apiEntity)
				if err == nil {
					mu.Lock()
					result.EntitiesCreated++
					mu.Unlock()
					return nil
				}

				if isConflictError(err) {
					_, updateErr := i.client.UpdateEntity(ctx, blueprintID, entityID, apiEntity)
					if updateErr != nil {
						mu.Lock()
						result.Errors = append(result.Errors, fmt.Sprintf("Entity %s: %v", entityID, updateErr))
						mu.Unlock()
						return nil
					}
					mu.Lock()
					result.EntitiesUpdated++
					mu.Unlock()
					return nil
				}

				mu.Lock()
				result.Errors = append(result.Errors, fmt.Sprintf("Entity %s: %v", entityID, err))
				mu.Unlock()
				return nil
			})
		}
	}

	// Import scorecards - group by blueprint for bulk updates
	if shouldImport("scorecards", opts.IncludeResources) {
		// Group scorecards by blueprint
		scorecardsByBlueprint := make(map[string][]api.Scorecard)
		for _, scorecard := range data.Scorecards {
			sc := scorecard
			blueprintID, ok1 := sc["blueprintIdentifier"].(string)
			scorecardID, ok2 := sc["identifier"].(string)
			if !ok1 || !ok2 || blueprintID == "" || scorecardID == "" {
				continue
			}
			cleaned := cleanSystemFields(sc, []string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id"})
			apiSc := api.Scorecard(cleaned)
			scorecardsByBlueprint[blueprintID] = append(scorecardsByBlueprint[blueprintID], apiSc)
		}

		// Process scorecards grouped by blueprint
		for blueprintID, scorecards := range scorecardsByBlueprint {
			bpID := blueprintID
			scs := scorecards
			g.Go(func() error {
				// Try to create all scorecards first, collect conflicts for bulk update
				toUpdate := []api.Scorecard{}

				for _, sc := range scs {
					scID, ok := sc["identifier"].(string)
					if !ok || scID == "" {
						continue
					}

					_, err := i.client.CreateScorecard(ctx, bpID, sc)
					if err == nil {
						mu.Lock()
						result.ScorecardsCreated++
						mu.Unlock()
					} else if isConflictError(err) {
						// Will be updated via bulk update
						toUpdate = append(toUpdate, sc)
					} else {
						mu.Lock()
						result.Errors = append(result.Errors, fmt.Sprintf("Scorecard %s: %v", scID, err))
						mu.Unlock()
					}
				}

				// Update existing scorecards using bulk PUT endpoint
				if len(toUpdate) > 0 {
					_, err := i.client.UpdateScorecards(ctx, bpID, toUpdate)
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
	}

	// Import actions (includes both blueprint actions and automations)
	if shouldImport("actions", opts.IncludeResources) || shouldImport("automations", opts.IncludeResources) {
		for _, action := range data.Actions {
			act := action
			g.Go(func() error {
				actionID, ok := act["identifier"].(string)
				if !ok || actionID == "" {
					return nil
				}

				cleaned := cleanSystemFields(act, []string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id"})
				apiAction := api.Automation(cleaned)

				// Use CreateAutomation endpoint for both actions and automations
				_, err := i.client.CreateAutomation(ctx, apiAction)
				if err == nil {
					mu.Lock()
					result.ActionsCreated++
					mu.Unlock()
					return nil
				}

				if isConflictError(err) {
					_, updateErr := i.client.UpdateAutomation(ctx, actionID, apiAction)
					if updateErr != nil {
						mu.Lock()
						result.Errors = append(result.Errors, fmt.Sprintf("Action %s: %v", actionID, updateErr))
						mu.Unlock()
						return nil
					}
					mu.Lock()
					result.ActionsUpdated++
					mu.Unlock()
					return nil
				}

				mu.Lock()
				result.Errors = append(result.Errors, fmt.Sprintf("Action %s: %v", actionID, err))
				mu.Unlock()
				return nil
			})
		}
	}

	// Import teams
	if shouldImport("teams", opts.IncludeResources) {
		for _, team := range data.Teams {
			t := team
			g.Go(func() error {
				teamName, ok := t["name"].(string)
				if !ok || teamName == "" {
					return nil
				}

				apiTeam := api.Team(t)

				_, err := i.client.CreateTeam(ctx, apiTeam)
				if err == nil {
					mu.Lock()
					result.TeamsCreated++
					mu.Unlock()
					return nil
				}

				if isConflictError(err) {
					_, updateErr := i.client.UpdateTeam(ctx, teamName, apiTeam)
					if updateErr != nil {
						mu.Lock()
						result.Errors = append(result.Errors, fmt.Sprintf("Team %s: %v", teamName, updateErr))
						mu.Unlock()
						return nil
					}
					mu.Lock()
					result.TeamsUpdated++
					mu.Unlock()
					return nil
				}

				mu.Lock()
				result.Errors = append(result.Errors, fmt.Sprintf("Team %s: %v", teamName, err))
				mu.Unlock()
				return nil
			})
		}
	}

	// Import users
	if shouldImport("users", opts.IncludeResources) {
		for _, user := range data.Users {
			u := user
			g.Go(func() error {
				userEmail, ok := u["email"].(string)
				if !ok || userEmail == "" {
					return nil
				}

				cleaned := cleanSystemFields(u, []string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id"})
				apiUser := api.User(cleaned)

				// Try to invite/create user first
				_, err := i.client.InviteUser(ctx, apiUser)
				if err == nil {
					mu.Lock()
					result.UsersCreated++
					mu.Unlock()
					return nil
				}

				// If conflict (user already exists), try update
				if isConflictError(err) {
					_, updateErr := i.client.UpdateUser(ctx, userEmail, apiUser)
					if updateErr != nil {
						mu.Lock()
						result.Errors = append(result.Errors, fmt.Sprintf("User %s: %v", userEmail, updateErr))
						mu.Unlock()
						return nil
					}
					mu.Lock()
					result.UsersUpdated++
					mu.Unlock()
					return nil
				}

				mu.Lock()
				result.Errors = append(result.Errors, fmt.Sprintf("User %s: %v", userEmail, err))
				mu.Unlock()
				return nil
			})
		}
	}

	// Import pages
	if shouldImport("pages", opts.IncludeResources) {
		for _, page := range data.Pages {
			p := page
			g.Go(func() error {
				pageID, ok1 := p["identifier"].(string)
				if !ok1 || pageID == "" {
					return nil
				}

				cleaned := cleanSystemFields(p, []string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id", "protected", "after", "section", "sidebar"})
				apiPage := api.Page(cleaned)

				_, err := i.client.CreatePage(ctx, apiPage)
				if err == nil {
					mu.Lock()
					result.PagesCreated++
					mu.Unlock()
					return nil
				}

				if isConflictError(err) {
					_, updateErr := i.client.UpdatePage(ctx, pageID, apiPage)
					if updateErr != nil {
						mu.Lock()
						result.Errors = append(result.Errors, fmt.Sprintf("Page %s: %v", pageID, updateErr))
						mu.Unlock()
						return nil
					}
					mu.Lock()
					result.PagesUpdated++
					mu.Unlock()
					return nil
				}

				mu.Lock()
				result.Errors = append(result.Errors, fmt.Sprintf("Page %s: %v", pageID, err))
				mu.Unlock()
				return nil
			})
		}
	}

	// Import integrations (update config only)
	if shouldImport("integrations", opts.IncludeResources) {
		for _, integration := range data.Integrations {
			integ := integration
			g.Go(func() error {
				integrationID, ok := integ["identifier"].(string)
				if !ok || integrationID == "" {
					return nil
				}

				// Check if context is already canceled
				select {
				case <-ctx.Done():
					mu.Lock()
					result.Errors = append(result.Errors, fmt.Sprintf("Integration %s: %v", integrationID, ctx.Err()))
					mu.Unlock()
					return nil
				default:
				}

				// Convert to map[string]interface{} for config update
				configMap := make(map[string]interface{})
				for k, v := range integ {
					configMap[k] = v
				}

				_, err := i.client.UpdateIntegrationConfig(ctx, integrationID, configMap)
				if err != nil {
					// Don't fail the entire import for integration errors
					// Context cancellation is expected if another goroutine failed
					mu.Lock()
					if ctx.Err() != nil {
						// Context was canceled, likely by another goroutine
						result.Errors = append(result.Errors, fmt.Sprintf("Integration %s: operation canceled", integrationID))
					} else {
						result.Errors = append(result.Errors, fmt.Sprintf("Integration %s: %v", integrationID, err))
					}
					mu.Unlock()
					return nil
				}

				mu.Lock()
				result.IntegrationsUpdated++
				mu.Unlock()
				return nil
			})
		}
	}

	// Wait for all imports to complete
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return result, nil
}

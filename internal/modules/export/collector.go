package export

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/port-experimental/port-cli/internal/api"
	"golang.org/x/sync/errgroup"
)

// Options represents export options.
type Options struct {
	OutputPath             string
	Blueprints             []string
	Format                 string
	SkipEntities           bool
	SkipSystemBlueprints   bool // skip _* blueprint schemas and their entities
	IncludeRuleResults     bool // include the _rule_result system blueprint and its entities (excluded by default)
	IncludeResources       []string
	ExcludeBlueprints      []string // deep: exclude blueprint schema + all its resources
	ExcludeBlueprintSchema []string // shallow: exclude only the blueprint schema, keep resources
}

// Validate validates export options.
func (o *Options) Validate() error {
	if o.OutputPath == "" {
		return fmt.Errorf("output_path is required")
	}

	if o.Format != "" && o.Format != "json" && o.Format != "tar" {
		return fmt.Errorf("format must be 'json' or 'tar'")
	}

	return nil
}

// Data represents collected export data.
type Data struct {
	Blueprints []api.Blueprint
	Entities   []api.Entity
	Scorecards []api.Scorecard
	Actions    []api.Action
	// BlueprintPermissions maps blueprint identifier -> permissions object.
	BlueprintPermissions map[string]api.Permissions
	// ActionPermissions maps action identifier -> permissions object.
	ActionPermissions map[string]api.Permissions
	Teams             []api.Team
	Users             []api.User
	Folders           []api.Folder
	Pages             []api.Page
	Integrations      []api.Integration
	TimeoutErrors     []string // Blueprints that timed out during export
}

// Collector collects data from Port API concurrently.
type Collector struct {
	client *api.Client
}

// NewCollector creates a new collector.
func NewCollector(client *api.Client) *Collector {
	return &Collector{
		client: client,
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

// isTimeoutError checks if an error is a timeout error (504 Gateway Timeout).
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	// Check for various timeout indicators
	return strings.Contains(errStr, "504") ||
		strings.Contains(errStr, "gateway timeout") ||
		strings.Contains(errStr, "timeout_error") ||
		strings.Contains(errStr, "request was too long") ||
		strings.Contains(errStr, "timeout")
}

// Collect collects all data from Port API concurrently.
func (c *Collector) Collect(ctx context.Context, opts Options) (*Data, error) {
	data := &Data{
		Blueprints:           []api.Blueprint{},
		Entities:             []api.Entity{},
		Scorecards:           []api.Scorecard{},
		Actions:              []api.Action{},
		Teams:                []api.Team{},
		Users:                []api.User{},
		Folders:              []api.Folder{},
		Pages:                []api.Page{},
		Integrations:         []api.Integration{},
		TimeoutErrors:        []string{},
		BlueprintPermissions: make(map[string]api.Permissions),
		ActionPermissions:    make(map[string]api.Permissions),
	}

	// Collect blueprints first (needed for other resources)
	var blueprints []api.Blueprint
	if shouldCollect("blueprints", opts.IncludeResources) {
		allBlueprints, err := c.client.GetBlueprints(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get blueprints: %w", err)
		}

		// Filter blueprints if specified
		if len(opts.Blueprints) > 0 {
			blueprintSet := make(map[string]bool)
			for _, bpID := range opts.Blueprints {
				blueprintSet[bpID] = true
			}

			for _, bp := range allBlueprints {
				if identifier, ok := bp["identifier"].(string); ok && blueprintSet[identifier] {
					blueprints = append(blueprints, bp)
				}
			}
		} else {
			blueprints = allBlueprints
		}

		excludeDeep := opts.ExcludeBlueprints
		if !opts.IncludeRuleResults {
			excludeDeep = append(excludeDeep, "_rule_result")
		}
		excludeSchema := opts.ExcludeBlueprintSchema
		if opts.SkipSystemBlueprints {
			for _, bp := range blueprints {
				id, _ := bp["identifier"].(string)
				if strings.HasPrefix(id, "_") {
					excludeSchema = append(excludeSchema, id)
				}
			}
		}
		iterBlueprints, dataBlueprints := ApplyBlueprintExclusions(blueprints, excludeDeep, excludeSchema)
		data.Blueprints = dataBlueprints
		blueprints = iterBlueprints
	} else {
		// Still need blueprints for dependent resources
		allBlueprints, err := c.client.GetBlueprints(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get blueprints: %w", err)
		}

		if len(opts.Blueprints) > 0 {
			blueprintSet := make(map[string]bool)
			for _, bpID := range opts.Blueprints {
				blueprintSet[bpID] = true
			}

			for _, bp := range allBlueprints {
				if identifier, ok := bp["identifier"].(string); ok && blueprintSet[identifier] {
					blueprints = append(blueprints, bp)
				}
			}
		} else {
			blueprints = allBlueprints
		}

		// Discard dataList: blueprints are not written to output in this branch (shouldCollect("blueprints") is false)
		excludeDeep2 := opts.ExcludeBlueprints
		if !opts.IncludeRuleResults {
			excludeDeep2 = append(excludeDeep2, "_rule_result")
		}
		excludeSchema2 := opts.ExcludeBlueprintSchema
		if opts.SkipSystemBlueprints {
			for _, bp := range blueprints {
				id, _ := bp["identifier"].(string)
				if strings.HasPrefix(id, "_") {
					excludeSchema2 = append(excludeSchema2, id)
				}
			}
		}
		iterBlueprints, _ := ApplyBlueprintExclusions(blueprints, excludeDeep2, excludeSchema2)
		blueprints = iterBlueprints
	}

	// Use errgroup for concurrent collection
	g, ctx := errgroup.WithContext(ctx)
	var mu sync.Mutex
	var timeoutErrors []string // Track timeout errors separately

	// Collect entities, scorecards, and actions concurrently per blueprint
	for _, blueprint := range blueprints {
		bp := blueprint
		bpID, ok := bp["identifier"].(string)
		if !ok {
			continue
		}

		// Collect entities
		skipEntitiesForBP := opts.SkipEntities || (opts.SkipSystemBlueprints && strings.HasPrefix(bpID, "_"))
		if !skipEntitiesForBP && shouldCollect("entities", opts.IncludeResources) {
			g.Go(func() error {
				entities, err := c.client.GetEntities(ctx, bpID, nil)
				if err != nil {
					// Handle expected errors gracefully
					if strings.Contains(err.Error(), "410 Gone") {
						// Blueprint without entities - expected case
						return nil
					}

					// Handle timeout errors gracefully - skip this blueprint instead of failing entire export
					if isTimeoutError(err) {
						// Collect timeout error but don't fail the export
						mu.Lock()
						timeoutErrors = append(timeoutErrors, fmt.Sprintf("Blueprint %s: timeout getting entities (skipped)", bpID))
						mu.Unlock()
						// Return nil to allow export to continue
						return nil
					}

					// Other errors are still failures
					return fmt.Errorf("failed to get entities for blueprint %s: %w", bpID, err)
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
				scorecards, err := c.client.GetScorecards(ctx, bpID)
				if err != nil {
					// Silent skip for expected errors
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

		// Collect actions (and their permissions)
		if shouldCollect("actions", opts.IncludeResources) {
			g.Go(func() error {
				actions, err := c.client.GetActions(ctx, bpID)
				if err != nil {
					// Silent skip for expected errors
					if !strings.Contains(err.Error(), "410 Gone") {
						return fmt.Errorf("failed to get actions for blueprint %s: %w", bpID, err)
					}
					return nil
				}

				mu.Lock()
				data.Actions = append(data.Actions, actions...)
				mu.Unlock()

				// Fetch permissions for each action
				for _, action := range actions {
					actionID, ok := action["identifier"].(string)
					if !ok {
						continue
					}
					aID := actionID // capture for goroutine closure
					g.Go(func() error {
						perms, err := c.client.GetActionPermissions(ctx, aID)
						if err != nil {
							// Non-fatal: skip silently
							return nil
						}
						mu.Lock()
						data.ActionPermissions[aID] = perms
						mu.Unlock()
						return nil
					})
				}
				return nil
			})
		}

		// Collect blueprint permissions
		if shouldCollect("blueprint-permissions", opts.IncludeResources) || len(opts.IncludeResources) == 0 {
			bpIDCopy := bpID // capture for goroutine closure
			g.Go(func() error {
				perms, err := c.client.GetBlueprintPermissions(ctx, bpIDCopy)
				if err != nil {
					// Non-fatal: permissions fetch failure should not abort the export
					return nil
				}
				mu.Lock()
				data.BlueprintPermissions[bpIDCopy] = perms
				mu.Unlock()
				return nil
			})
		}
	}

	// Collect organization-wide resources
	if !opts.SkipEntities && shouldCollect("teams", opts.IncludeResources) {
		g.Go(func() error {
			teams, err := c.client.GetTeams(ctx)
			if err != nil {
				return fmt.Errorf("failed to get teams: %w", err)
			}

			mu.Lock()
			data.Teams = teams
			mu.Unlock()
			return nil
		})
	}

	// Collect users
	if !opts.SkipEntities && shouldCollect("users", opts.IncludeResources) {
		g.Go(func() error {
			users, err := c.client.GetUsers(ctx)
			if err != nil {
				return fmt.Errorf("failed to get users: %w", err)
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
			allActions, err := c.client.GetAllActions(ctx)
			if err != nil {
				return fmt.Errorf("failed to get all actions/automations: %w", err)
			}

			mu.Lock()
			data.Actions = append(data.Actions, allActions...)
			mu.Unlock()

			// Fetch permissions for each org-wide action
			for _, action := range allActions {
				actionID, ok := action["identifier"].(string)
				if !ok {
					continue
				}
				aID := actionID // capture for goroutine closure
				g.Go(func() error {
					perms, err := c.client.GetActionPermissions(ctx, aID)
					if err != nil {
						// Non-fatal: skip silently
						return nil
					}
					mu.Lock()
					data.ActionPermissions[aID] = perms
					mu.Unlock()
					return nil
				})
			}
			return nil
		})
	}

	if shouldCollect("pages", opts.IncludeResources) {
		g.Go(func() error {
			folders, err := c.client.GetFolders(ctx)
			if err != nil {
				return fmt.Errorf("failed to get folders: %w", err)
			}
			pages, err := c.client.GetPages(ctx)
			if err != nil {
				return fmt.Errorf("failed to get pages: %w", err)
			}

			mu.Lock()
			data.Folders = folders
			data.Pages = pages
			mu.Unlock()
			return nil
		})
	}

	if shouldCollect("integrations", opts.IncludeResources) {
		g.Go(func() error {
			integrations, err := c.client.GetIntegrations(ctx)
			if err != nil {
				return fmt.Errorf("failed to get integrations: %w", err)
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

	// Attach timeout errors to data
	data.TimeoutErrors = timeoutErrors

	return data, nil
}

// ApplyBlueprintExclusions returns two filtered slices from all:
//   - iterList: used to iterate for fetching entities/scorecards/actions (deep-excluded removed, schema-only kept)
//   - dataList: written to data.Blueprints for export output (both deep and schema-only excluded)
func ApplyBlueprintExclusions(all []api.Blueprint, excludeDeep, excludeSchema []string) (iterList, dataList []api.Blueprint) {
	if len(excludeDeep) == 0 && len(excludeSchema) == 0 {
		return all, all
	}
	deepSet := make(map[string]bool, len(excludeDeep))
	for _, id := range excludeDeep {
		deepSet[id] = true
	}
	schemaSet := make(map[string]bool, len(excludeSchema))
	for _, id := range excludeSchema {
		schemaSet[id] = true
	}

	for _, bp := range all {
		id, _ := bp["identifier"].(string)
		if deepSet[id] {
			// Deep exclusion: skip in both lists
			continue
		}
		// In iteration list (fetches dependent resources) for everyone not deep-excluded
		iterList = append(iterList, bp)
		if schemaSet[id] {
			// Schema-only exclusion: skip in data (output) list only
			continue
		}
		dataList = append(dataList, bp)
	}
	return iterList, dataList
}

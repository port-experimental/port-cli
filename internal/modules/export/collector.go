package export

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/port-labs/port-cli/internal/api"
	"golang.org/x/sync/errgroup"
)

// Options represents export options.
type Options struct {
	OutputPath      string
	Blueprints      []string
	Format          string
	SkipEntities    bool
	IncludeResources []string
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
	Blueprints   []api.Blueprint
	Entities     []api.Entity
	Scorecards   []api.Scorecard
	Actions      []api.Action
	Teams        []api.Team
	Users        []api.User
	Pages        []api.Page
	Integrations []api.Integration
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

// Collect collects all data from Port API concurrently.
func (c *Collector) Collect(ctx context.Context, opts Options) (*Data, error) {
	data := &Data{
		Blueprints:   []api.Blueprint{},
		Entities:     []api.Entity{},
		Scorecards:   []api.Scorecard{},
		Actions:      []api.Action{},
		Teams:        []api.Team{},
		Users:        []api.User{},
		Pages:        []api.Page{},
		Integrations: []api.Integration{},
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

		data.Blueprints = blueprints
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
	}

	// Use errgroup for concurrent collection
	g, ctx := errgroup.WithContext(ctx)
	var mu sync.Mutex

	// Collect entities, scorecards, and actions concurrently per blueprint
	for _, blueprint := range blueprints {
		bp := blueprint
		bpID, ok := bp["identifier"].(string)
		if !ok {
			continue
		}

		// Collect entities
		if !opts.SkipEntities && shouldCollect("entities", opts.IncludeResources) {
			g.Go(func() error {
				entities, err := c.client.GetEntities(ctx, bpID, nil)
				if err != nil {
					// Only warn for unexpected errors (410 Gone is expected for blueprints without entities)
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

		// Collect actions
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
				return nil
			})
		}
	}

	// Collect organization-wide resources
	if shouldCollect("teams", opts.IncludeResources) {
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
	if shouldCollect("users", opts.IncludeResources) {
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
			return nil
		})
	}

	if shouldCollect("pages", opts.IncludeResources) {
		g.Go(func() error {
			pages, err := c.client.GetPages(ctx)
			if err != nil {
				return fmt.Errorf("failed to get pages: %w", err)
			}

			mu.Lock()
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

	return data, nil
}


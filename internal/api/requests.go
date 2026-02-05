package api

import (
	"context"
	"encoding/json"
	"fmt"
)

// Blueprint represents a Port blueprint.
type Blueprint map[string]interface{}

// Entity represents a Port entity.
type Entity map[string]interface{}

// Scorecard represents a Port scorecard.
type Scorecard map[string]interface{}

// Action represents a Port action.
type Action map[string]interface{}

// Team represents a Port team.
type Team map[string]interface{}

// User represents a Port user.
type User map[string]interface{}

// Automation represents a Port automation.
type Automation map[string]interface{}

// Page represents a Port page.
type Page map[string]interface{}

// Integration represents a Port integration.
type Integration map[string]interface{}

// GetBlueprints retrieves all blueprints.
func (c *Client) GetBlueprints(ctx context.Context) ([]Blueprint, error) {
	resp, err := c.request(ctx, "GET", "/blueprints", nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Blueprints []Blueprint `json:"blueprints"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode blueprints: %w", err)
	}

	return result.Blueprints, nil
}

// GetBlueprint retrieves a specific blueprint.
func (c *Client) GetBlueprint(ctx context.Context, identifier string) (Blueprint, error) {
	resp, err := c.request(ctx, "GET", fmt.Sprintf("/blueprints/%s", identifier), nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Blueprint Blueprint `json:"blueprint"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode blueprint: %w", err)
	}

	return result.Blueprint, nil
}

// CreateBlueprint creates a new blueprint.
func (c *Client) CreateBlueprint(ctx context.Context, blueprint Blueprint) (Blueprint, error) {
	resp, err := c.request(ctx, "POST", "/blueprints", blueprint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Blueprint Blueprint `json:"blueprint"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode blueprint: %w", err)
	}

	return result.Blueprint, nil
}

// UpdateBlueprint updates an existing blueprint.
func (c *Client) UpdateBlueprint(ctx context.Context, identifier string, blueprint Blueprint) (Blueprint, error) {
	resp, err := c.request(ctx, "PUT", fmt.Sprintf("/blueprints/%s", identifier), blueprint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Blueprint Blueprint `json:"blueprint"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode blueprint: %w", err)
	}

	return result.Blueprint, nil
}

// DeleteBlueprint deletes a blueprint.
func (c *Client) DeleteBlueprint(ctx context.Context, identifier string) error {
	resp, err := c.request(ctx, "DELETE", fmt.Sprintf("/blueprints/%s", identifier), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// GetEntities retrieves entities for a blueprint.
func (c *Client) GetEntities(ctx context.Context, blueprintIdentifier string, params map[string]string) ([]Entity, error) {
	resp, err := c.request(ctx, "GET", fmt.Sprintf("/blueprints/%s/entities", blueprintIdentifier), nil, params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Entities []Entity `json:"entities"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode entities: %w", err)
	}

	return result.Entities, nil
}

// GetEntity retrieves a specific entity.
func (c *Client) GetEntity(ctx context.Context, blueprintIdentifier, entityIdentifier string) (Entity, error) {
	resp, err := c.request(ctx, "GET", fmt.Sprintf("/blueprints/%s/entities/%s", blueprintIdentifier, entityIdentifier), nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Entity Entity `json:"entity"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode entity: %w", err)
	}

	return result.Entity, nil
}

// CreateEntity creates a new entity.
func (c *Client) CreateEntity(ctx context.Context, blueprintIdentifier string, entity Entity) (Entity, error) {
	resp, err := c.request(ctx, "POST", fmt.Sprintf("/blueprints/%s/entities", blueprintIdentifier), entity, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Entity Entity `json:"entity"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode entity: %w", err)
	}

	return result.Entity, nil
}

// UpdateEntity updates an existing entity.
func (c *Client) UpdateEntity(ctx context.Context, blueprintIdentifier, entityIdentifier string, entity Entity) (Entity, error) {
	resp, err := c.request(ctx, "PUT", fmt.Sprintf("/blueprints/%s/entities/%s", blueprintIdentifier, entityIdentifier), entity, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Entity Entity `json:"entity"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode entity: %w", err)
	}

	return result.Entity, nil
}

// DeleteEntity deletes an entity.
func (c *Client) DeleteEntity(ctx context.Context, blueprintIdentifier, entityIdentifier string) error {
	resp, err := c.request(ctx, "DELETE", fmt.Sprintf("/blueprints/%s/entities/%s", blueprintIdentifier, entityIdentifier), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// GetScorecards retrieves scorecards for a blueprint.
func (c *Client) GetScorecards(ctx context.Context, blueprintIdentifier string) ([]Scorecard, error) {
	resp, err := c.request(ctx, "GET", fmt.Sprintf("/blueprints/%s/scorecards", blueprintIdentifier), nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Scorecards []Scorecard `json:"scorecards"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode scorecards: %w", err)
	}

	return result.Scorecards, nil
}

// GetAllScorecards retrieves all scorecards (organization-wide).
func (c *Client) GetAllScorecards(ctx context.Context) ([]Scorecard, error) {
	resp, err := c.request(ctx, "GET", "/scorecards", nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Scorecards []Scorecard `json:"scorecards"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode scorecards: %w", err)
	}

	return result.Scorecards, nil
}

// CreateScorecard creates a new scorecard for a blueprint.
func (c *Client) CreateScorecard(ctx context.Context, blueprintIdentifier string, scorecard Scorecard) (Scorecard, error) {
	resp, err := c.request(ctx, "POST", fmt.Sprintf("/blueprints/%s/scorecards", blueprintIdentifier), scorecard, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Scorecard Scorecard `json:"scorecard"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode scorecard: %w", err)
	}

	return result.Scorecard, nil
}

// UpdateScorecard updates an existing scorecard.
func (c *Client) UpdateScorecard(ctx context.Context, blueprintIdentifier, scorecardIdentifier string, scorecard Scorecard) (Scorecard, error) {
	resp, err := c.request(ctx, "PATCH", fmt.Sprintf("/blueprints/%s/scorecards/%s", blueprintIdentifier, scorecardIdentifier), scorecard, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Scorecard Scorecard `json:"scorecard"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode scorecard: %w", err)
	}

	return result.Scorecard, nil
}

// UpdateScorecards updates multiple scorecards for a blueprint using bulk PUT endpoint.
// The API expects the array of scorecards directly (not wrapped in an object).
func (c *Client) UpdateScorecards(ctx context.Context, blueprintIdentifier string, scorecards []Scorecard) ([]Scorecard, error) {
	// Send array directly - API does not expect {"scorecards": [...]} wrapper
	resp, err := c.request(ctx, "PUT", fmt.Sprintf("/blueprints/%s/scorecards", blueprintIdentifier), scorecards, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Scorecards []Scorecard `json:"scorecards"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode scorecards: %w", err)
	}

	return result.Scorecards, nil
}

// DeleteScorecard deletes a scorecard.
func (c *Client) DeleteScorecard(ctx context.Context, blueprintIdentifier, scorecardIdentifier string) error {
	resp, err := c.request(ctx, "DELETE", fmt.Sprintf("/blueprints/%s/scorecards/%s", blueprintIdentifier, scorecardIdentifier), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// GetActions retrieves actions for a blueprint.
func (c *Client) GetActions(ctx context.Context, blueprintIdentifier string) ([]Action, error) {
	resp, err := c.request(ctx, "GET", fmt.Sprintf("/blueprints/%s/actions", blueprintIdentifier), nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Actions []Action `json:"actions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode actions: %w", err)
	}

	return result.Actions, nil
}

// CreateAction creates a blueprint-level action.
func (c *Client) CreateAction(ctx context.Context, blueprintIdentifier string, action Action) (Action, error) {
	resp, err := c.request(ctx, "POST", fmt.Sprintf("/blueprints/%s/actions", blueprintIdentifier), action, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Action Action `json:"action"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode action: %w", err)
	}

	return result.Action, nil
}

// UpdateAction updates an existing blueprint-level action.
func (c *Client) UpdateAction(ctx context.Context, blueprintIdentifier, actionIdentifier string, action Action) (Action, error) {
	resp, err := c.request(ctx, "PATCH", fmt.Sprintf("/blueprints/%s/actions/%s", blueprintIdentifier, actionIdentifier), action, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Action Action `json:"action"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode action: %w", err)
	}

	return result.Action, nil
}

// DeleteAction deletes a blueprint-level action.
func (c *Client) DeleteAction(ctx context.Context, blueprintIdentifier, actionIdentifier string) error {
	resp, err := c.request(ctx, "DELETE", fmt.Sprintf("/blueprints/%s/actions/%s", blueprintIdentifier, actionIdentifier), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// GetTeams retrieves all teams.
func (c *Client) GetTeams(ctx context.Context) ([]Team, error) {
	resp, err := c.request(ctx, "GET", "/teams", nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Teams []Team `json:"teams"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode teams: %w", err)
	}

	return result.Teams, nil
}

// CreateTeam creates a new team.
func (c *Client) CreateTeam(ctx context.Context, team Team) (Team, error) {
	resp, err := c.request(ctx, "POST", "/teams", team, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Team Team `json:"team"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode team: %w", err)
	}

	return result.Team, nil
}

// UpdateTeam updates an existing team.
func (c *Client) UpdateTeam(ctx context.Context, teamName string, team Team) (Team, error) {
	resp, err := c.request(ctx, "PATCH", fmt.Sprintf("/teams/%s", teamName), team, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Team Team `json:"team"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode team: %w", err)
	}

	return result.Team, nil
}

// DeleteTeam deletes a team.
func (c *Client) DeleteTeam(ctx context.Context, teamName string) error {
	resp, err := c.request(ctx, "DELETE", fmt.Sprintf("/teams/%s", teamName), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// GetUsers retrieves all users in the organization.
func (c *Client) GetUsers(ctx context.Context) ([]User, error) {
	resp, err := c.request(ctx, "GET", "/users", nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Users []User `json:"users"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode users: %w", err)
	}

	return result.Users, nil
}

// GetUser retrieves a specific user by email.
func (c *Client) GetUser(ctx context.Context, userEmail string) (User, error) {
	resp, err := c.request(ctx, "GET", fmt.Sprintf("/users/%s", userEmail), nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		User User `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode user: %w", err)
	}

	return result.User, nil
}

// InviteUser invites a user to the organization.
func (c *Client) InviteUser(ctx context.Context, user User) (User, error) {
	// The API expects the user to be wrapped in an "invitee" property
	payload := map[string]interface{}{
		"invitee": user,
	}
	resp, err := c.request(ctx, "POST", "/users/invite", payload, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		User User `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode user: %w", err)
	}

	return result.User, nil
}

// UpdateUser updates an existing user.
func (c *Client) UpdateUser(ctx context.Context, userEmail string, user User) (User, error) {
	resp, err := c.request(ctx, "PATCH", fmt.Sprintf("/users/%s", userEmail), user, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		User User `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode user: %w", err)
	}

	return result.User, nil
}

// GetAllActions retrieves all actions and automations (organization-wide).
func (c *Client) GetAllActions(ctx context.Context) ([]Action, error) {
	resp, err := c.request(ctx, "GET", "/actions", nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Actions []Action `json:"actions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode actions: %w", err)
	}

	return result.Actions, nil
}

// CreateAutomation creates a new automation (organization-wide action).
func (c *Client) CreateAutomation(ctx context.Context, automation Automation) (Automation, error) {
	resp, err := c.request(ctx, "POST", "/actions", automation, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Action Automation `json:"action"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode automation: %w", err)
	}

	return result.Action, nil
}

// UpdateAutomation updates an existing automation.
func (c *Client) UpdateAutomation(ctx context.Context, automationIdentifier string, automation Automation) (Automation, error) {
	resp, err := c.request(ctx, "PUT", fmt.Sprintf("/actions/%s", automationIdentifier), automation, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Action Automation `json:"action"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode automation: %w", err)
	}

	return result.Action, nil
}

// DeleteAutomation deletes an automation.
func (c *Client) DeleteAutomation(ctx context.Context, automationIdentifier string) error {
	resp, err := c.request(ctx, "DELETE", fmt.Sprintf("/actions/%s", automationIdentifier), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// GetPages retrieves all pages.
func (c *Client) GetPages(ctx context.Context) ([]Page, error) {
	resp, err := c.request(ctx, "GET", "/pages", nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Pages []Page `json:"pages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode pages: %w", err)
	}

	return result.Pages, nil
}

// CreatePage creates a new page.
func (c *Client) CreatePage(ctx context.Context, page Page) (Page, error) {
	resp, err := c.request(ctx, "POST", "/pages", page, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Page Page `json:"page"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode page: %w", err)
	}

	return result.Page, nil
}

// GetPage retrieves a single page by identifier.
func (c *Client) GetPage(ctx context.Context, pageIdentifier string) (Page, error) {
	resp, err := c.request(ctx, "GET", fmt.Sprintf("/pages/%s", pageIdentifier), nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Page Page `json:"page"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode page: %w", err)
	}

	return result.Page, nil
}

// UpdatePage updates an existing page.
func (c *Client) UpdatePage(ctx context.Context, pageIdentifier string, page Page) (Page, error) {
	resp, err := c.request(ctx, "PATCH", fmt.Sprintf("/pages/%s", pageIdentifier), page, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Page Page `json:"page"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode page: %w", err)
	}

	return result.Page, nil
}

// DeletePage deletes a page.
func (c *Client) DeletePage(ctx context.Context, pageIdentifier string) error {
	resp, err := c.request(ctx, "DELETE", fmt.Sprintf("/pages/%s", pageIdentifier), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// GetIntegrations retrieves all integrations.
func (c *Client) GetIntegrations(ctx context.Context) ([]Integration, error) {
	resp, err := c.request(ctx, "GET", "/integration", nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Integrations []Integration `json:"integrations"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode integrations: %w", err)
	}

	return result.Integrations, nil
}

// UpdateIntegrationConfig updates an integration's configuration.
func (c *Client) UpdateIntegrationConfig(ctx context.Context, integrationIdentifier string, config map[string]interface{}) (Integration, error) {
	resp, err := c.request(ctx, "PATCH", fmt.Sprintf("/integration/%s/config", integrationIdentifier), config, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Integration Integration `json:"integration"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode integration: %w", err)
	}

	return result.Integration, nil
}

// DeleteIntegration deletes an integration.
func (c *Client) DeleteIntegration(ctx context.Context, integrationIdentifier string) error {
	resp, err := c.request(ctx, "DELETE", fmt.Sprintf("/integration/%s", integrationIdentifier), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

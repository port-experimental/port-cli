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

// Folder represents a Port sidebar folder.
type Folder map[string]interface{}

// Integration represents a Port integration.
type Integration map[string]interface{}

// Permissions represents Port resource permissions.
type Permissions map[string]interface{}

// MigrationRequest represents a Port migration request.
type MigrationRequest struct {
	SourceBlueprint string                 `json:"sourceBlueprint"`
	Mapping         map[string]interface{} `json:"mapping"`
}

// Migration represents a Port migration job.
type Migration map[string]interface{}

type RequestParams struct {
	Method   string
	Endpoint string
	Data     any
	Params   map[string]string
}

func (c *Client) Request(ctx context.Context, params RequestParams) (any, error) {
	resp, err := c.request(ctx, params.Method, params.Endpoint, params.Data, params.Params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return result, nil
}

// GetBlueprints retrieves all blueprints.
func (c *Client) GetBlueprints(ctx context.Context) ([]Blueprint, error) {
	return doEnvelope[[]Blueprint](c, ctx, "GET", "/blueprints", nil, nil, "blueprints", "failed to decode blueprints")
}

// GetBlueprint retrieves a specific blueprint.
func (c *Client) GetBlueprint(ctx context.Context, identifier string) (Blueprint, error) {
	return doEnvelope[Blueprint](c, ctx, "GET", fmt.Sprintf("/blueprints/%s", identifier), nil, nil, "blueprint", "failed to decode blueprint")
}

// CreateBlueprint creates a new blueprint.
func (c *Client) CreateBlueprint(ctx context.Context, blueprint Blueprint) (Blueprint, error) {
	return doEnvelope[Blueprint](c, ctx, "POST", "/blueprints", blueprint, nil, "blueprint", "failed to decode blueprint")
}

// UpdateBlueprint updates an existing blueprint.
func (c *Client) UpdateBlueprint(ctx context.Context, identifier string, blueprint Blueprint) (Blueprint, error) {
	return doEnvelope[Blueprint](c, ctx, "PUT", fmt.Sprintf("/blueprints/%s", identifier), blueprint, nil, "blueprint", "failed to decode blueprint")
}

// PatchBlueprint updates an existing blueprint with a partial payload (PATCH).
func (c *Client) PatchBlueprint(ctx context.Context, identifier string, blueprint Blueprint) (Blueprint, error) {
	return doEnvelope[Blueprint](c, ctx, "PATCH", fmt.Sprintf("/blueprints/%s", identifier), blueprint, nil, "blueprint", "failed to decode blueprint")
}

// DeleteBlueprint deletes a blueprint.
func (c *Client) DeleteBlueprint(ctx context.Context, identifier string) error {
	return c.doNoContent(ctx, "DELETE", fmt.Sprintf("/blueprints/%s", identifier), nil, nil)
}

func migrationFromResponse(result map[string]interface{}) Migration {
	if migration, ok := result["migration"].(map[string]interface{}); ok {
		return Migration(migration)
	}
	return Migration(result)
}

// CreateMigration starts a Port migration.
func (c *Client) CreateMigration(ctx context.Context, migration MigrationRequest) (Migration, error) {
	resp, err := c.request(ctx, "POST", "/migrations", migration, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode migration result: %w", err)
	}
	return migrationFromResponse(result), nil
}

// GetMigration retrieves a Port migration job.
func (c *Client) GetMigration(ctx context.Context, identifier string) (Migration, error) {
	resp, err := c.request(ctx, "GET", fmt.Sprintf("/migrations/%s", identifier), nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode migration result: %w", err)
	}
	return migrationFromResponse(result), nil
}

// GetEntities retrieves entities for a blueprint.
func (c *Client) GetEntities(ctx context.Context, blueprintIdentifier string, params map[string]string) ([]Entity, error) {
	return doEnvelope[[]Entity](c, ctx, "GET", fmt.Sprintf("/blueprints/%s/entities", blueprintIdentifier), nil, params, "entities", "failed to decode entities")
}

const entitySearchPaginationThreshold = 10000

// GetEntitiesCount retrieves the number of entities for a blueprint.
func (c *Client) GetEntitiesCount(ctx context.Context, blueprintIdentifier string) (int, error) {
	return doEnvelope[int](c, ctx, "GET", fmt.Sprintf("/blueprints/%s/entities-count", blueprintIdentifier), nil, nil, "count", "failed to decode entities count")
}

// ForEachEntity retrieves all entities for a blueprint and calls yield with
// each returned batch. Blueprints above the threshold use the search endpoint
// because it supports cursor pagination; smaller blueprints use the canonical
// GET endpoint.
func (c *Client) ForEachEntity(ctx context.Context, blueprintIdentifier string, yield func([]Entity) error) error {
	count, err := c.GetEntitiesCount(ctx, blueprintIdentifier)
	if err != nil {
		return err
	}

	if count > entitySearchPaginationThreshold {
		return c.ForEachEntityPage(ctx, blueprintIdentifier, paginatedEntitySearchBody(), yield)
	}

	entities, err := c.GetEntities(ctx, blueprintIdentifier, nil)
	if err != nil {
		return err
	}
	if len(entities) == 0 {
		return nil
	}
	return yield(entities)
}

func paginatedEntitySearchBody() map[string]interface{} {
	return map[string]interface{}{
		"query": map[string]interface{}{
			"combinator": "and",
			"rules":      []interface{}{},
		},
		"limit": 1000,
	}
}

// SearchEntities queries entities for a blueprint using Port's search endpoint.
// Pages are fetched sequentially (each page's cursor depends on the previous
// response), so this cannot be parallelized client-side. For large blueprints
// this is still far better than GetEntities, which makes a single unbounded
// request that 504s above ~10k entities.
func (c *Client) SearchEntities(ctx context.Context, blueprintIdentifier string, body map[string]interface{}) ([]Entity, error) {
	// Pre-allocate a reasonable capacity to avoid repeated slice growth.
	all := make([]Entity, 0, 256)
	err := c.ForEachEntityPage(ctx, blueprintIdentifier, body, func(entities []Entity) error {
		all = append(all, entities...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

// ForEachEntityPage queries entities for a blueprint using Port's search
// endpoint and calls yield once for each returned page.
func (c *Client) ForEachEntityPage(ctx context.Context, blueprintIdentifier string, body map[string]interface{}, yield func([]Entity) error) error {
	var from string
	for {
		pageBody := cloneBody(body)
		if from != "" {
			pageBody["from"] = from
		}
		resp, err := c.request(ctx, "POST", fmt.Sprintf("/blueprints/%s/entities/search", blueprintIdentifier), pageBody, nil)
		if err != nil {
			return err
		}

		var result struct {
			Entities []Entity `json:"entities"`
			Next     string   `json:"next"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return fmt.Errorf("failed to decode entities: %w", err)
		}
		resp.Body.Close()

		if len(result.Entities) > 0 {
			if err := yield(result.Entities); err != nil {
				return err
			}
		}
		if result.Next == "" {
			return nil
		}
		from = result.Next
	}
}

// cloneBody performs a shallow top-level copy of the request body map so that
// pagination can add a "from" key without mutating the original. Nested values
// (e.g. "query", "rules") are shared by reference; callers must not mutate
// them between pages.
func cloneBody(body map[string]interface{}) map[string]interface{} {
	cloned := make(map[string]interface{}, len(body)+1)
	for k, v := range body {
		cloned[k] = v
	}
	return cloned
}

// TopSearchEntities queries entities using Port's top-search endpoint, which
// supports server-side sorting.
func (c *Client) TopSearchEntities(ctx context.Context, blueprintIdentifier string, body map[string]interface{}) ([]Entity, error) {
	return doEnvelope[[]Entity](c, ctx, "POST", fmt.Sprintf("/blueprints/%s/entities/top-search", blueprintIdentifier), body, nil, "entities", "failed to decode entities")
}

// GetEntity retrieves a specific entity.
func (c *Client) GetEntity(ctx context.Context, blueprintIdentifier, entityIdentifier string) (Entity, error) {
	return doEnvelope[Entity](c, ctx, "GET", fmt.Sprintf("/blueprints/%s/entities/%s", blueprintIdentifier, entityIdentifier), nil, nil, "entity", "failed to decode entity")
}

// CreateEntity creates a new entity.
func (c *Client) CreateEntity(ctx context.Context, blueprintIdentifier string, entity Entity) (Entity, error) {
	return doEnvelope[Entity](c, ctx, "POST", fmt.Sprintf("/blueprints/%s/entities", blueprintIdentifier), entity, nil, "entity", "failed to decode entity")
}

// UpdateEntity updates an existing entity.
func (c *Client) UpdateEntity(ctx context.Context, blueprintIdentifier, entityIdentifier string, entity Entity) (Entity, error) {
	return doEnvelope[Entity](c, ctx, "PUT", fmt.Sprintf("/blueprints/%s/entities/%s", blueprintIdentifier, entityIdentifier), entity, nil, "entity", "failed to decode entity")
}

// DeleteEntity deletes an entity.
func (c *Client) DeleteEntity(ctx context.Context, blueprintIdentifier, entityIdentifier string) error {
	return c.doNoContent(ctx, "DELETE", fmt.Sprintf("/blueprints/%s/entities/%s", blueprintIdentifier, entityIdentifier), nil, nil)
}

// BulkDeleteEntities deletes multiple entities for a blueprint.
func (c *Client) BulkDeleteEntities(ctx context.Context, blueprintIdentifier string, entityIdentifiers []string, deleteDependents bool) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"entities": entityIdentifiers,
	}

	params := map[string]string{}
	if deleteDependents {
		params["delete_dependents"] = "true"
	} else {
		params["delete_dependents"] = "false"
	}

	resp, err := c.request(ctx, "POST", fmt.Sprintf("/blueprints/%s/bulk/entities/delete", blueprintIdentifier), payload, params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// GetScorecards retrieves scorecards for a blueprint.
func (c *Client) GetScorecards(ctx context.Context, blueprintIdentifier string) ([]Scorecard, error) {
	return doEnvelope[[]Scorecard](c, ctx, "GET", fmt.Sprintf("/blueprints/%s/scorecards", blueprintIdentifier), nil, nil, "scorecards", "failed to decode scorecards")
}

// GetAllScorecards retrieves all scorecards (organization-wide).
func (c *Client) GetAllScorecards(ctx context.Context) ([]Scorecard, error) {
	return doEnvelope[[]Scorecard](c, ctx, "GET", "/scorecards", nil, nil, "scorecards", "failed to decode scorecards")
}

// CreateScorecard creates a new scorecard for a blueprint.
func (c *Client) CreateScorecard(ctx context.Context, blueprintIdentifier string, scorecard Scorecard) (Scorecard, error) {
	return doEnvelope[Scorecard](c, ctx, "POST", fmt.Sprintf("/blueprints/%s/scorecards", blueprintIdentifier), scorecard, nil, "scorecard", "failed to decode scorecard")
}

// UpdateScorecard updates an existing scorecard.
func (c *Client) UpdateScorecard(ctx context.Context, blueprintIdentifier, scorecardIdentifier string, scorecard Scorecard) (Scorecard, error) {
	return doEnvelope[Scorecard](c, ctx, "PATCH", fmt.Sprintf("/blueprints/%s/scorecards/%s", blueprintIdentifier, scorecardIdentifier), scorecard, nil, "scorecard", "failed to decode scorecard")
}

// UpdateScorecards updates multiple scorecards for a blueprint using bulk PUT endpoint.
// The API expects the array of scorecards directly (not wrapped in an object).
func (c *Client) UpdateScorecards(ctx context.Context, blueprintIdentifier string, scorecards []Scorecard) ([]Scorecard, error) {
	return doEnvelope[[]Scorecard](c, ctx, "PUT", fmt.Sprintf("/blueprints/%s/scorecards", blueprintIdentifier), scorecards, nil, "scorecards", "failed to decode scorecards")
}

// DeleteScorecard deletes a scorecard.
func (c *Client) DeleteScorecard(ctx context.Context, blueprintIdentifier, scorecardIdentifier string) error {
	return c.doNoContent(ctx, "DELETE", fmt.Sprintf("/blueprints/%s/scorecards/%s", blueprintIdentifier, scorecardIdentifier), nil, nil)
}

// GetActions retrieves actions for a blueprint using the organization-wide
// actions endpoint and client-side blueprint filtering.
func (c *Client) GetActions(ctx context.Context, blueprintIdentifier string) ([]Action, error) {
	allActions, err := c.GetAllActions(ctx)
	if err != nil {
		return nil, err
	}

	actions := make([]Action, 0)
	for _, action := range allActions {
		if SelfServiceActionBlueprintID(action) == blueprintIdentifier {
			actions = append(actions, action)
		}
	}
	return actions, nil
}

// CreateAction creates a blueprint-level action using the organization-wide
// actions endpoint.
func (c *Client) CreateAction(ctx context.Context, blueprintIdentifier string, action Action) (Action, error) {
	action = ActionWithBlueprintIdentifier(action, blueprintIdentifier)
	return doEnvelope[Action](c, ctx, "POST", "/actions", action, nil, "action", "failed to decode action")
}

// UpdateAction updates an existing blueprint-level action using the
// organization-wide actions endpoint.
func (c *Client) UpdateAction(ctx context.Context, blueprintIdentifier, actionIdentifier string, action Action) (Action, error) {
	action = ActionWithBlueprintIdentifier(action, blueprintIdentifier)
	return doEnvelope[Action](c, ctx, "PUT", fmt.Sprintf("/actions/%s", actionIdentifier), action, nil, "action", "failed to decode action")
}

// DeleteAction deletes a blueprint-level action using the organization-wide
// actions endpoint.
func (c *Client) DeleteAction(ctx context.Context, blueprintIdentifier, actionIdentifier string) error {
	return c.DeleteActionByID(ctx, actionIdentifier)
}

// DeleteActionByID deletes an action using the organization-wide actions endpoint.
func (c *Client) DeleteActionByID(ctx context.Context, actionIdentifier string) error {
	return c.doNoContent(ctx, "DELETE", fmt.Sprintf("/actions/%s", actionIdentifier), nil, nil)
}

// ActionBlueprintID extracts the blueprint identifier an action or automation
// references, if any. Self-service actions carry it at trigger.blueprintIdentifier;
// automations can carry it at trigger.event.blueprintIdentifier.
func ActionBlueprintID(action Action) string {
	trigger, ok := action["trigger"].(map[string]interface{})
	if !ok {
		return ""
	}
	if bpID, ok := trigger["blueprintIdentifier"].(string); ok && bpID != "" {
		return bpID
	}
	if event, ok := trigger["event"].(map[string]interface{}); ok {
		if bpID, ok := event["blueprintIdentifier"].(string); ok {
			return bpID
		}
	}
	return ""
}

// SelfServiceActionBlueprintID extracts the blueprint identifier from a
// non-automation action. Automations are excluded even when their event
// references a blueprint.
func SelfServiceActionBlueprintID(action Action) string {
	if IsAutomationAction(action) {
		return ""
	}
	trigger, ok := action["trigger"].(map[string]interface{})
	if !ok {
		return ""
	}
	bpID, _ := trigger["blueprintIdentifier"].(string)
	return bpID
}

// IsAutomationAction reports whether an action record is an automation.
func IsAutomationAction(action Action) bool {
	trigger, ok := action["trigger"].(map[string]interface{})
	if !ok {
		return false
	}
	triggerType, _ := trigger["type"].(string)
	return triggerType == "automation"
}

// ActionWithBlueprintIdentifier returns a shallow copy of action with
// trigger.blueprintIdentifier set.
func ActionWithBlueprintIdentifier(action Action, blueprintIdentifier string) Action {
	if blueprintIdentifier == "" {
		return action
	}

	out := make(Action, len(action)+1)
	for k, v := range action {
		out[k] = v
	}

	trigger, _ := out["trigger"].(map[string]interface{})
	clonedTrigger := make(map[string]interface{}, len(trigger)+1)
	for k, v := range trigger {
		clonedTrigger[k] = v
	}
	clonedTrigger["blueprintIdentifier"] = blueprintIdentifier
	out["trigger"] = clonedTrigger
	return out
}

// GetTeams retrieves all teams.
func (c *Client) GetTeams(ctx context.Context) ([]Team, error) {
	return doEnvelope[[]Team](c, ctx, "GET", "/teams", nil, nil, "teams", "failed to decode teams")
}

// CreateTeam creates a new team.
func (c *Client) CreateTeam(ctx context.Context, team Team) (Team, error) {
	return doEnvelope[Team](c, ctx, "POST", "/teams", team, nil, "team", "failed to decode team")
}

// UpdateTeam updates an existing team.
func (c *Client) UpdateTeam(ctx context.Context, teamName string, team Team) (Team, error) {
	return doEnvelope[Team](c, ctx, "PATCH", fmt.Sprintf("/teams/%s", teamName), team, nil, "team", "failed to decode team")
}

// DeleteTeam deletes a team.
func (c *Client) DeleteTeam(ctx context.Context, teamName string) error {
	return c.doNoContent(ctx, "DELETE", fmt.Sprintf("/teams/%s", teamName), nil, nil)
}

// GetUsers retrieves all users in the organization.
func (c *Client) GetUsers(ctx context.Context) ([]User, error) {
	return doEnvelope[[]User](c, ctx, "GET", "/users", nil, nil, "users", "failed to decode users")
}

// GetUser retrieves a specific user by email.
func (c *Client) GetUser(ctx context.Context, userEmail string) (User, error) {
	return doEnvelope[User](c, ctx, "GET", fmt.Sprintf("/users/%s", userEmail), nil, nil, "user", "failed to decode user")
}

// BulkEntityError is a single per-entity failure returned by the bulk entity endpoint.
type BulkEntityError struct {
	Identifier string  `json:"identifier"`
	Index      float64 `json:"index"`
	StatusCode float64 `json:"statusCode"`
	Error      string  `json:"error"`
	Message    string  `json:"message"`
}

// CreateUserEntitiesBulk creates up to 20 _user blueprint entities in one call.
// Set upsert=true to overwrite existing entities; false returns 409 errors for conflicts.
func (c *Client) CreateUserEntitiesBulk(ctx context.Context, entities []Entity, upsert bool) ([]BulkEntityError, error) {
	return c.BulkUpsertEntities(ctx, "_user", entities, upsert)
}

// BulkUpsertEntities upserts up to 20 entities for any blueprint in one call.
// Set upsert=true to overwrite existing entities; false returns 409 errors for conflicts.
func (c *Client) BulkUpsertEntities(ctx context.Context, blueprintID string, entities []Entity, upsert bool) ([]BulkEntityError, error) {
	payload := map[string]interface{}{
		"entities": entities,
	}
	path := fmt.Sprintf("/blueprints/%s/entities/bulk?upsert=%t", blueprintID, upsert)
	return doEnvelope[[]BulkEntityError](c, ctx, "POST", path, payload, nil, "errors", "failed to decode bulk entity upsert result")
}

// GetAllActions retrieves all actions and automations (organization-wide).
func (c *Client) GetAllActions(ctx context.Context) ([]Action, error) {
	return doEnvelope[[]Action](c, ctx, "GET", "/actions", nil, nil, "actions", "failed to decode actions")
}

// CreateAutomation creates a new automation (organization-wide action).
func (c *Client) CreateAutomation(ctx context.Context, automation Automation) (Automation, error) {
	return doEnvelope[Automation](c, ctx, "POST", "/actions", automation, nil, "action", "failed to decode automation")
}

// UpdateAutomation updates an existing automation.
func (c *Client) UpdateAutomation(ctx context.Context, automationIdentifier string, automation Automation) (Automation, error) {
	return doEnvelope[Automation](c, ctx, "PUT", fmt.Sprintf("/actions/%s", automationIdentifier), automation, nil, "action", "failed to decode automation")
}

// DeleteAutomation deletes an automation.
func (c *Client) DeleteAutomation(ctx context.Context, automationIdentifier string) error {
	return c.DeleteActionByID(ctx, automationIdentifier)
}

// GetPages retrieves all pages.
func (c *Client) GetPages(ctx context.Context) ([]Page, error) {
	return doEnvelope[[]Page](c, ctx, "GET", "/pages", nil, nil, "pages", "failed to decode pages")
}

// CreatePage creates a new page.
func (c *Client) CreatePage(ctx context.Context, page Page) (Page, error) {
	return doEnvelope[Page](c, ctx, "POST", "/pages", page, nil, "page", "failed to decode page")
}

// GetPage retrieves a single page by identifier.
func (c *Client) GetPage(ctx context.Context, pageIdentifier string) (Page, error) {
	return doEnvelope[Page](c, ctx, "GET", fmt.Sprintf("/pages/%s", pageIdentifier), nil, nil, "page", "failed to decode page")
}

// UpdatePage updates an existing page.
func (c *Client) UpdatePage(ctx context.Context, pageIdentifier string, page Page) (Page, error) {
	return doEnvelope[Page](c, ctx, "PATCH", fmt.Sprintf("/pages/%s", pageIdentifier), page, nil, "page", "failed to decode page")
}

// DeletePage deletes a page.
func (c *Client) DeletePage(ctx context.Context, pageIdentifier string) error {
	return c.doNoContent(ctx, "DELETE", fmt.Sprintf("/pages/%s", pageIdentifier), nil, nil)
}

// GetFolders retrieves sidebar folders from the catalog sidebar.
func (c *Client) GetFolders(ctx context.Context) ([]Folder, error) {
	resp, err := c.request(ctx, "GET", "/sidebars/catalog", nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode folders: %w", err)
	}

	var folders []Folder
	collectFoldersFromSidebarResponse(raw, &folders)

	seen := make(map[string]bool, len(folders))
	unique := make([]Folder, 0, len(folders))
	for _, folder := range folders {
		identifier, _ := folder["identifier"].(string)
		if identifier == "" || seen[identifier] {
			continue
		}
		seen[identifier] = true
		unique = append(unique, folder)
	}

	return unique, nil
}

func collectFoldersFromSidebarResponse(value interface{}, folders *[]Folder) {
	switch v := value.(type) {
	case map[string]interface{}:
		if sidebarType, ok := v["sidebarType"].(string); ok && sidebarType == "folder" {
			*folders = append(*folders, Folder(v))
		}
		for _, nested := range v {
			collectFoldersFromSidebarResponse(nested, folders)
		}
	case []interface{}:
		for _, item := range v {
			collectFoldersFromSidebarResponse(item, folders)
		}
	}
}

// CreateFolder creates a sidebar folder under the catalog sidebar.
func (c *Client) CreateFolder(ctx context.Context, folder Folder) error {
	return c.doNoContent(ctx, "POST", "/sidebars/catalog/folders", folder, nil)
}

// DeleteFolder deletes a sidebar folder from the catalog sidebar.
func (c *Client) DeleteFolder(ctx context.Context, folderIdentifier string) error {
	return c.doNoContent(ctx, "DELETE", fmt.Sprintf("/sidebars/catalog/folders/%s", folderIdentifier), nil, nil)
}

// GetIntegrations retrieves all integrations.
func (c *Client) GetIntegrations(ctx context.Context) ([]Integration, error) {
	return doEnvelope[[]Integration](c, ctx, "GET", "/integration", nil, nil, "integrations", "failed to decode integrations")
}

// UpdateIntegrationConfig updates an integration's configuration.
func (c *Client) UpdateIntegrationConfig(ctx context.Context, integrationIdentifier string, config map[string]interface{}) (Integration, error) {
	return doEnvelope[Integration](c, ctx, "PATCH", fmt.Sprintf("/integration/%s/config", integrationIdentifier), config, nil, "integration", "failed to decode integration")
}

// DeleteIntegration deletes an integration.
func (c *Client) DeleteIntegration(ctx context.Context, integrationIdentifier string) error {
	return c.doNoContent(ctx, "DELETE", fmt.Sprintf("/integration/%s", integrationIdentifier), nil, nil)
}

// GetBlueprintPermissions retrieves permissions for a blueprint.
func (c *Client) GetBlueprintPermissions(ctx context.Context, blueprintIdentifier string) (Permissions, error) {
	return doEnvelope[Permissions](c, ctx, "GET", fmt.Sprintf("/blueprints/%s/permissions", blueprintIdentifier), nil, nil, "permissions", "failed to decode blueprint permissions")
}

// UpdateBlueprintPermissions updates permissions for a blueprint.
func (c *Client) UpdateBlueprintPermissions(ctx context.Context, blueprintIdentifier string, permissions Permissions) (Permissions, error) {
	return doEnvelope[Permissions](c, ctx, "PATCH", fmt.Sprintf("/blueprints/%s/permissions", blueprintIdentifier), permissions, nil, "permissions", "failed to decode updated blueprint permissions")
}

// GetActionPermissions retrieves permissions for an action.
func (c *Client) GetActionPermissions(ctx context.Context, actionIdentifier string) (Permissions, error) {
	return doEnvelope[Permissions](c, ctx, "GET", fmt.Sprintf("/actions/%s/permissions", actionIdentifier), nil, nil, "permissions", "failed to decode action permissions")
}

// UpdateActionPermissions updates permissions for an action.
func (c *Client) UpdateActionPermissions(ctx context.Context, actionIdentifier string, permissions Permissions) (Permissions, error) {
	return doEnvelope[Permissions](c, ctx, "PATCH", fmt.Sprintf("/actions/%s/permissions", actionIdentifier), permissions, nil, "permissions", "failed to decode updated action permissions")
}

// GetPagePermissions retrieves permissions for a page.
func (c *Client) GetPagePermissions(ctx context.Context, pageIdentifier string) (Permissions, error) {
	return doEnvelope[Permissions](c, ctx, "GET", fmt.Sprintf("/pages/%s/permissions", pageIdentifier), nil, nil, "permissions", "failed to decode page permissions")
}

// UpdatePagePermissions updates permissions for a page.
func (c *Client) UpdatePagePermissions(ctx context.Context, pageIdentifier string, permissions Permissions) (Permissions, error) {
	return doEnvelope[Permissions](c, ctx, "PATCH", fmt.Sprintf("/pages/%s/permissions", pageIdentifier), permissions, nil, "permissions", "failed to decode updated page permissions")
}

// ActionRun represents a Port action run.
type ActionRun map[string]interface{}

// GetActionRuns retrieves all action runs.
func (c *Client) GetActionRuns(ctx context.Context) ([]ActionRun, error) {
	return doEnvelope[[]ActionRun](c, ctx, "GET", "/actions/runs", nil, nil, "runs", "failed to decode action runs")
}

// GetActionRun retrieves a specific action run.
func (c *Client) GetActionRun(ctx context.Context, runID string) (ActionRun, error) {
	return doEnvelope[ActionRun](c, ctx, "GET", fmt.Sprintf("/actions/runs/%s", runID), nil, nil, "run", "failed to decode action run")
}

// UpdateActionRun updates an action run (set status, message, link, logs).
func (c *Client) UpdateActionRun(ctx context.Context, runID string, body map[string]interface{}) (ActionRun, error) {
	return doEnvelope[ActionRun](c, ctx, "PATCH", fmt.Sprintf("/actions/runs/%s", runID), body, nil, "run", "failed to decode action run")
}

// ApproveActionRun approves or declines an action run.
func (c *Client) ApproveActionRun(ctx context.Context, runID string, body map[string]interface{}) (ActionRun, error) {
	return doEnvelope[ActionRun](c, ctx, "PATCH", fmt.Sprintf("/actions/runs/%s/approval", runID), body, nil, "run", "failed to decode action run")
}

// ExecuteAction creates a new action run for the given action identifier.
func (c *Client) ExecuteAction(ctx context.Context, actionID string, body map[string]interface{}) (ActionRun, error) {
	return doEnvelope[ActionRun](c, ctx, "POST", fmt.Sprintf("/actions/%s/runs", actionID), body, nil, "run", "failed to decode action run")
}

// Webhook represents a Port webhook.
type Webhook map[string]interface{}

// GetWebhooks retrieves all webhooks.
func (c *Client) GetWebhooks(ctx context.Context) ([]Webhook, error) {
	return doEnvelope[[]Webhook](c, ctx, "GET", "/webhooks", nil, nil, "webhooks", "failed to decode webhooks")
}

// GetWebhook retrieves a specific webhook.
func (c *Client) GetWebhook(ctx context.Context, id string) (Webhook, error) {
	return doEnvelope[Webhook](c, ctx, "GET", fmt.Sprintf("/webhooks/%s", id), nil, nil, "webhook", "failed to decode webhook")
}

// CreateWebhook creates a new webhook.
func (c *Client) CreateWebhook(ctx context.Context, body map[string]interface{}) (Webhook, error) {
	return doEnvelope[Webhook](c, ctx, "POST", "/webhooks", body, nil, "webhook", "failed to decode webhook")
}

// UpdateWebhook updates an existing webhook.
func (c *Client) UpdateWebhook(ctx context.Context, id string, body map[string]interface{}) (Webhook, error) {
	return doEnvelope[Webhook](c, ctx, "PATCH", fmt.Sprintf("/webhooks/%s", id), body, nil, "webhook", "failed to decode webhook")
}

// DeleteWebhook deletes a webhook.
func (c *Client) DeleteWebhook(ctx context.Context, id string) error {
	return c.doNoContent(ctx, "DELETE", fmt.Sprintf("/webhooks/%s", id), nil, nil)
}

// AuditLog represents a Port audit log entry.
type AuditLog map[string]interface{}

// GetAuditLogs retrieves the organization audit log.
func (c *Client) GetAuditLogs(ctx context.Context) ([]AuditLog, error) {
	return doEnvelope[[]AuditLog](c, ctx, "GET", "/audit-log", nil, nil, "audits", "failed to decode audit logs")
}

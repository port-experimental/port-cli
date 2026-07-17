package commands

import "github.com/port-experimental/port-cli/internal/api"

// allFactoryResourceSpecs returns every APIResourceSpec registered through the
// descriptor factory (including permissions child groups). Used by contract
// sweeps so new specs cannot land without command-tree/flag assertions.
func allFactoryResourceSpecs() []APIResourceSpec {
	return []APIResourceSpec{
		blueprintsResourceSpec(),
		entitiesResourceSpec(),
		pagesResourceSpec(),
		teamsResourceSpec(),
		usersResourceSpec(),
		scorecardsResourceSpec(),
		actionsResourceSpec(),
		webhooksResourceSpec(),
		actionRunsResourceSpec(),
		auditResourceSpec(),
		agentsResourceSpec(),
		aiResourceSpec(),
		integrationsResourceSpec(),
		migrationsResourceSpec(),
		organizationResourceSpec(),
		secretsResourceSpec(),
		workflowsResourceSpec(),
		workflowRunsResourceSpec(),
		permissionsChildSpec(
			"blueprints", "blueprint",
			(*api.Client).GetBlueprintPermissions,
			(*api.Client).UpdateBlueprintPermissions,
		),
		permissionsChildSpec(
			"actions", "action",
			(*api.Client).GetActionPermissions,
			(*api.Client).UpdateActionPermissions,
		),
		permissionsChildSpec(
			"pages", "page",
			(*api.Client).GetPagePermissions,
			(*api.Client).UpdatePagePermissions,
		),
	}
}

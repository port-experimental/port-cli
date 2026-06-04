//go:build e2e

package skills

// Seed catalog identifiers (stable across Port demo-skills seed; versions resolved from API).
const (
	SeedGroupPlatform   = "platform-engineering"
	SeedGroupOperations = "operations"
	SeedGroupSecurity   = "security"

	SeedSkillLocalDevSetup        = "local-dev-setup"
	SeedSkillPortAPIClient        = "port-api-client"
	SeedSkillIntegrationsOverview = "integrations-overview"
	SeedSkillMCPTroubleshooting   = "mcp-troubleshooting"
	SeedSkillWorkflowAutomation   = "workflow-automation"
	SeedSkillSecurityPRReview     = "security-pr-review"
)

var seedAllGroups = []string{SeedGroupPlatform, SeedGroupOperations, SeedGroupSecurity}

// seedCatalogSkillIDs is the set of skills written by yarn seed:general (scripts/demo-skills).
var seedCatalogSkillIDs = map[string]bool{
	SeedSkillLocalDevSetup:        true,
	SeedSkillPortAPIClient:        true,
	SeedSkillIntegrationsOverview: true,
	SeedSkillMCPTroubleshooting:   true,
	SeedSkillWorkflowAutomation:   true,
	SeedSkillSecurityPRReview:     true,
}

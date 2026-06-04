//go:build e2e

package skills

// Demo catalog identifiers (stable across demo seed; versions are resolved from the API).
const (
	DemoGroupRequired  = "demo-engineering-required"
	DemoGroupOptional  = "demo-engineering-optional"
	DemoGroupSecurity  = "demo-security-manual"

	DemoSkillOnboarding   = "demo-onboarding"
	DemoSkillAPIGuide     = "demo-api-guide"
	DemoSkillStandalone   = "demo-standalone"
	DemoSkillTroubleshoot = "demo-troubleshoot"
	DemoSkillWorkflows    = "demo-workflows"
	DemoSkillSecurity     = "demo-security-review"
)

var demoAllGroups = []string{DemoGroupRequired, DemoGroupOptional, DemoGroupSecurity}

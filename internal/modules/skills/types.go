package skills

// SkillFile is one file in a skill directory tree (from ai-service).
type SkillFile struct {
	Path    string
	Content string
}

// SkillLocation controls where a skill is written on disk.
type SkillLocation string

const (
	SkillLocationGlobal  SkillLocation = "global"
	SkillLocationProject SkillLocation = "project"

	NoGroupDir    = "_skills_without_group"
	PortSkillsDir = "port"
)

// Skill is a catalog skill ready to sync to disk.
type Skill struct {
	Identifier  string
	Title       string
	Description string
	GroupIDs    []string
	Required    bool
	AutoSync    bool
	Location    SkillLocation
	Files       []SkillFile
}

// SkillGroup is a skill group from the catalog.
type SkillGroup struct {
	Identifier string
	Title      string
	Required   bool
	AutoSync   bool
	SkillIDs   []string
}

// FetchedSkills contains skills split by whether they are required.
type FetchedSkills struct {
	Required []Skill
	Optional []Skill
	Groups   []SkillGroup
}

func parseSkillLocation(raw string) SkillLocation {
	if raw == string(SkillLocationProject) {
		return SkillLocationProject
	}
	return SkillLocationGlobal
}

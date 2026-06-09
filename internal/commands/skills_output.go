package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/modules/skills"
	"github.com/port-experimental/port-cli/internal/styles"
)

func printLoadResult(result *skills.LoadSkillsResult) {
	fmt.Fprintf(os.Stderr,
		"%s %d skill(s) synced\n",
		styles.CheckMark,
		result.SkillCount,
	)

	if len(result.TargetResults) == 0 {
		return
	}

	var globalTargets, projectTargets, copilotRepoTargets []skills.TargetResult
	for _, t := range result.TargetResults {
		switch {
		case t.GitHubCopilotRepo:
			copilotRepoTargets = append(copilotRepoTargets, t)
		case t.IsProject:
			projectTargets = append(projectTargets, t)
		default:
			globalTargets = append(globalTargets, t)
		}
	}

	if len(globalTargets) > 0 {
		fmt.Fprintln(os.Stderr)
		for _, t := range globalTargets {
			fmt.Fprintf(os.Stderr, "  %s %s/skills/port/  %s  %s\n",
				styles.Circle,
				t.Path,
				styles.GlobalLabel,
				styles.Faint.Render(fmt.Sprintf("%d skills", t.SkillCount)),
			)
		}
	}

	if len(projectTargets) > 0 {
		fmt.Fprintln(os.Stderr)
		for _, t := range projectTargets {
			fmt.Fprintf(os.Stderr, "  %s %s/skills/port/  %s  %s\n",
				styles.Circle,
				t.Path,
				styles.ProjectLabel,
				styles.Faint.Render(fmt.Sprintf("%d skills", t.SkillCount)),
			)
		}
	}

	if len(copilotRepoTargets) > 0 {
		fmt.Fprintln(os.Stderr)
		for _, t := range copilotRepoTargets {
			fmt.Fprintf(os.Stderr, "  %s %s/skills/port/  %s  %s\n",
				styles.Circle,
				t.Path,
				styles.CopilotRepoLabel,
				styles.Faint.Render(fmt.Sprintf("%d skills · not synced to a global directory", t.SkillCount)),
			)
		}
	}
	fmt.Fprintln(os.Stderr)
}

func printSkillsStatus(status *skills.StatusResult) {
	fmt.Println("\nPort Skills Status")
	fmt.Println(strings.Repeat("─", 40))
	fmt.Printf("Last synced:     %s\n", valueOrNone(status.LastSyncedAt))
	fmt.Printf("\nHook targets (%d):\n", len(status.Targets))
	for _, t := range status.Targets {
		fmt.Printf("  - %s/skills/port/\n", t)
	}
	fmt.Printf("\nProject directories (%d):\n", len(status.ProjectDirs))
	if len(status.ProjectDirs) == 0 {
		fmt.Println("  (none)")
	}
	for _, d := range status.ProjectDirs {
		fmt.Printf("  - %s\n", d)
	}
	fmt.Printf("\nSkill selection:\n")
	if status.SelectAll {
		fmt.Println("  Groups:           all")
		fmt.Println("  Ungrouped skills: all")
	} else {
		if status.SelectAllGroups {
			fmt.Println("  Groups:           all")
		} else {
			fmt.Printf("  Groups (%d):\n", len(status.SelectedGroups))
			if len(status.SelectedGroups) == 0 {
				fmt.Println("    (none)")
			}
			for _, g := range status.SelectedGroups {
				fmt.Printf("    - %s\n", g)
			}
		}
		if status.SelectAllUngrouped {
			fmt.Println("  Ungrouped skills: all")
		} else {
			fmt.Printf("  Ungrouped skills (%d):\n", len(status.SelectedSkills))
			if len(status.SelectedSkills) == 0 {
				fmt.Println("    (none)")
			}
			for _, s := range status.SelectedSkills {
				fmt.Printf("    - %s\n", s)
			}
		}
	}
}

func printSkillsSearchResults(entries []api.SkillCatalogEntry, query string) {
	fmt.Printf("%s %d skill(s) matching %q:\n\n", styles.CheckMark, len(entries), query)
	for _, entry := range entries {
		s := entry.Skill
		title := strings.TrimSpace(s.Title)
		if title == "" || title == s.Identifier {
			fmt.Printf("  %s\n", styles.Bold.Render(s.Identifier))
		} else {
			fmt.Printf("  %s  %s\n", styles.Bold.Render(s.Identifier), title)
		}
		if loc := catalogPropString(s.Properties, "location"); loc != "" {
			fmt.Printf("    %s\n", styles.Faint.Render("location: "+loc))
		}
		if entry.Version != nil {
			if v := catalogPropString(entry.Version.Properties, "version"); v != "" {
				fmt.Printf("    %s\n", styles.Faint.Render("version: "+v))
			}
		}
	}
}

func printSkillsCatalog(entries []api.SkillCatalogEntry) {
	for _, entry := range entries {
		printSkillCatalogEntryCompact(entry)
	}
}

func printSkillCatalogEntryCompact(entry api.SkillCatalogEntry) {
	s := entry.Skill
	title := strings.TrimSpace(s.Title)
	if title == "" || title == s.Identifier {
		fmt.Printf("  %s\n", styles.Bold.Render(s.Identifier))
	} else {
		fmt.Printf("  %s  %s\n", styles.Bold.Render(s.Identifier), title)
	}
	if loc := catalogPropString(s.Properties, "location"); loc != "" {
		fmt.Printf("    %s\n", styles.Faint.Render("location: "+loc))
	}
	if entry.Version != nil {
		if v := catalogPropString(entry.Version.Properties, "version"); v != "" {
			line := "version: " + v
			if active := skillActiveVersionLabel(entry); active == "yes" {
				line += " (published)"
			}
			fmt.Printf("    %s\n", styles.Faint.Render(line))
		}
	} else {
		fmt.Printf("    %s\n", styles.Faint.Render("version: (none)"))
	}
}

func printSkillField(label, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	fmt.Printf("    %-16s %s\n", styles.Faint.Render(label+":"), value)
}

func catalogPropString(props map[string]interface{}, key string) string {
	if props == nil {
		return ""
	}
	raw, ok := props[key]
	if !ok || raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func formatCatalogTime(iso *string) string {
	if iso == nil {
		return ""
	}
	return strings.TrimSpace(*iso)
}

func skillActiveVersionLabel(entry api.SkillCatalogEntry) string {
	activeID := catalogRelationID(entry.Skill.Relations, "skill_active_version")
	if activeID == "" {
		return ""
	}
	if entry.Version == nil {
		return "not set"
	}
	if entry.Version.Identifier == activeID {
		return "yes"
	}
	return "no (resolved version is not active)"
}

func catalogRelationID(relations map[string]interface{}, key string) string {
	if relations == nil {
		return ""
	}
	raw, ok := relations[key]
	if !ok || raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]interface{}:
		if id, ok := v["identifier"].(string); ok {
			return strings.TrimSpace(id)
		}
		if id, ok := v["$identifier"].(string); ok {
			return strings.TrimSpace(id)
		}
	}
	return strings.TrimSpace(fmt.Sprint(raw))
}

func displayCatalogTitle(title, identifier string) string {
	title = strings.TrimSpace(title)
	if title == "" || title == identifier {
		return ""
	}
	return title
}

func printSkillsCatalogJSON(resp *api.SkillsSummaryResponse) error {
	if resp == nil {
		resp = &api.SkillsSummaryResponse{OK: true}
	}
	payload := *resp
	payload.OK = true
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

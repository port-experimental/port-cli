package commands

import (
	"context"
	"fmt"
	"strings"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/port-experimental/port-cli/internal/api/aiservice"
	"github.com/port-experimental/port-cli/internal/config"
	"github.com/port-experimental/port-cli/internal/modules/skills"
	"github.com/port-experimental/port-cli/internal/styles"
)

func configuredHookTargetNames(configManager *config.ConfigManager) ([]string, error) {
	if configManager == nil {
		return nil, nil
	}
	skillsCfg, err := configManager.LoadSkillsConfig()
	if err != nil {
		return nil, err
	}
	return skills.ResolveTargetNames(skillsCfg.Targets, skills.DefaultHookTargets()), nil
}

func unconfiguredHookTargets(configManager *config.ConfigManager) ([]skills.HookTarget, error) {
	configuredNames, err := configuredHookTargetNames(configManager)
	if err != nil {
		return nil, err
	}
	configured := toStringSet(configuredNames)
	allTargets := skills.DefaultHookTargets()
	var out []skills.HookTarget
	for _, t := range allTargets {
		if !configured[t.Name] {
			out = append(out, t)
		}
	}
	return out, nil
}

func resolveTargetsByName(names []string) ([]skills.HookTarget, error) {
	allTargets := skills.DefaultHookTargets()
	byName := make(map[string]skills.HookTarget, len(allTargets))
	for _, t := range allTargets {
		byName[t.Name] = t
	}
	var resolved []skills.HookTarget
	var unknown []string
	for _, name := range names {
		t, ok := byName[name]
		if !ok {
			unknown = append(unknown, name)
			continue
		}
		resolved = append(resolved, t)
	}
	if len(unknown) > 0 {
		return nil, fmt.Errorf("unknown AI tool(s): %s", strings.Join(unknown, ", "))
	}
	return resolved, nil
}

func promptAddTargetSelection(available []skills.HookTarget, configuredToolNames []string) ([]skills.HookTarget, error) {
	if len(available) == 0 {
		return nil, nil
	}
	targetOptions := make([]huh.Option[string], 0, len(available))
	for _, t := range available {
		label := t.Name
		if t.Note != "" {
			label = fmt.Sprintf("%s (%s)", t.Name, t.Note)
		}
		targetOptions = append(targetOptions, huh.NewOption(label, t.Name))
	}
	description := "Only tools not yet configured are listed. Use space to select, enter to confirm."
	if len(configuredToolNames) > 0 {
		description = fmt.Sprintf(
			"%s\n\nIf you don't select any tools here, added skills will sync to your existing tools: %s.",
			description,
			strings.Join(configuredToolNames, ", "),
		)
	}
	var selectedNames []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Add hooks for which AI tools?").
				Description(description).
				Options(targetOptions...).
				Height(len(targetOptions) + 4).
				Value(&selectedNames),
		),
	).WithHeight(0).WithTheme(&styles.FormTheme{})
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("prompt error: %w", err)
	}
	if len(selectedNames) == 0 && len(configuredToolNames) > 0 {
		lipgloss.Printf(
			"\n%s No new tools selected — skills will sync to: %s\n",
			styles.QuestionMark,
			styles.Bold.Render(strings.Join(configuredToolNames, ", ")),
		)
	}
	return resolveTargetsByName(selectedNames)
}

func promptAddGroupSelection(groups []skills.SkillGroup) ([]string, error) {
	if len(groups) == 0 {
		return nil, nil
	}
	groupOptions := make([]huh.Option[string], 0, len(groups))
	for _, g := range groups {
		groupOptions = append(groupOptions, huh.NewOption(groupLabel(g), g.Identifier))
	}
	var selected []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which skill groups would you like to add?").
				Description("Groups already in your selection are not shown. Use space to select, enter to confirm.").
				Options(groupOptions...).
				Height(len(groupOptions) + 4).
				Value(&selected),
		),
	).WithHeight(0).WithTheme(&styles.FormTheme{})
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("prompt error: %w", err)
	}
	return selected, nil
}

func promptAddSkillSelection(available []skills.Skill) ([]string, error) {
	if len(available) == 0 {
		return nil, nil
	}
	skillOptions := make([]huh.Option[string], 0, len(available))
	for _, s := range available {
		label := skillLabel(s)
		if len(s.GroupIDs) > 0 {
			label = fmt.Sprintf("%s (%s)", label, strings.Join(s.GroupIDs, ", "))
		}
		skillOptions = append(skillOptions, huh.NewOption(label, s.Identifier))
	}
	var selected []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which skills would you like to add?").
				Description("Skills already in your selection are not shown. Use space to select, enter to confirm.").
				Options(skillOptions...).
				Height(len(skillOptions) + 4).
				Value(&selected),
		),
	).WithHeight(0).WithTheme(&styles.FormTheme{})
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("prompt error: %w", err)
	}
	return selected, nil
}

func promptTargetSelection(configManager *config.ConfigManager) ([]skills.HookTarget, error) {
	allTargets := skills.DefaultHookTargets()

	var preSelected []string
	if configManager != nil {
		if skillsCfg, err := configManager.LoadSkillsConfig(); err == nil {
			preSelected = skills.ResolveTargetNames(skillsCfg.Targets, allTargets)
		}
	}

	targetOptions := make([]huh.Option[string], 0, len(allTargets))
	for _, t := range allTargets {
		label := t.Name
		if t.Note != "" {
			label = fmt.Sprintf("%s (%s)", t.Name, t.Note)
		}
		opt := huh.NewOption(label, t.Name)
		for _, ps := range preSelected {
			if ps == t.Name {
				opt = opt.Selected(true)
				break
			}
		}
		targetOptions = append(targetOptions, opt)
	}

	selectedNames := make([]string, len(preSelected))
	copy(selectedNames, preSelected)
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which AI tools should have hooks installed?").
				Description("Use space to select/deselect, enter to confirm.").
				Options(targetOptions...).
				Height(len(targetOptions) + 4).
				Value(&selectedNames),
		),
	).WithHeight(0).WithTheme(&styles.FormTheme{})
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("prompt error: %w", err)
	}

	if len(selectedNames) == 0 {
		return nil, fmt.Errorf("no AI tools selected — nothing to install")
	}

	nameSet := make(map[string]bool, len(selectedNames))
	for _, n := range selectedNames {
		nameSet[n] = true
	}
	var targets []skills.HookTarget
	for _, t := range allTargets {
		if nameSet[t.Name] {
			targets = append(targets, t)
		}
	}
	return targets, nil
}

func configuredHookTargets(configManager *config.ConfigManager) ([]skills.HookTarget, error) {
	names, err := configuredHookTargetNames(configManager)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, nil
	}
	return resolveTargetsByName(names)
}

func promptRemoveGroupSelection(groups []skills.SkillGroup) ([]string, error) {
	if len(groups) == 0 {
		return nil, nil
	}
	groupOptions := make([]huh.Option[string], 0, len(groups))
	for _, g := range groups {
		groupOptions = append(groupOptions, huh.NewOption(groupLabel(g), g.Identifier))
	}
	var selected []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which skill groups would you like to remove?").
				Description("Only groups currently in your selection are listed. Use space to select, enter to confirm.").
				Options(groupOptions...).
				Height(len(groupOptions) + 4).
				Value(&selected),
		),
	).WithHeight(0).WithTheme(&styles.FormTheme{})
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("prompt error: %w", err)
	}
	return selected, nil
}

func promptRemoveSkillSelection(available []skills.Skill) ([]string, error) {
	if len(available) == 0 {
		return nil, nil
	}
	skillOptions := make([]huh.Option[string], 0, len(available))
	for _, s := range available {
		label := skillLabel(s)
		if len(s.GroupIDs) > 0 {
			label = fmt.Sprintf("%s (%s)", label, strings.Join(s.GroupIDs, ", "))
		}
		skillOptions = append(skillOptions, huh.NewOption(label, s.Identifier))
	}
	var selected []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which skills would you like to remove?").
				Description("Only skills currently in your selection are listed. Use space to select, enter to confirm.").
				Options(skillOptions...).
				Height(len(skillOptions) + 4).
				Value(&selected),
		),
	).WithHeight(0).WithTheme(&styles.FormTheme{})
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("prompt error: %w", err)
	}
	return selected, nil
}

func promptRemoveTargetSelection(configured []skills.HookTarget) ([]skills.HookTarget, error) {
	if len(configured) == 0 {
		return nil, nil
	}
	targetOptions := make([]huh.Option[string], 0, len(configured))
	for _, t := range configured {
		label := t.Name
		if t.Note != "" {
			label = fmt.Sprintf("%s (%s)", t.Name, t.Note)
		}
		targetOptions = append(targetOptions, huh.NewOption(label, t.Name))
	}
	var selectedNames []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Remove hooks for which AI tools?").
				Description("Only tools currently configured are listed. Use space to select, enter to confirm.").
				Options(targetOptions...).
				Height(len(targetOptions) + 4).
				Value(&selectedNames),
		),
	).WithHeight(0).WithTheme(&styles.FormTheme{})
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("prompt error: %w", err)
	}
	return resolveTargetsByName(selectedNames)
}

// buildLoadSkillsOpts fetches skill groups from ai-service for interactive selection,
// then loads the sync catalog with team defaults plus include/exclude adjustments.
func buildLoadSkillsOpts(ctx context.Context, mod *skills.Module, promptSelection bool) (skills.LoadSkillsOptions, *skills.FetchedSkills, error) {
	if !promptSelection {
		return skills.LoadSkillsOptions{}, nil, nil
	}

	catalogGroups, err := mod.FetchSkillGroups(ctx)
	if err != nil {
		return skills.LoadSkillsOptions{}, nil, fmt.Errorf("failed to fetch skill groups from Port: %w", err)
	}

	if len(catalogGroups) == 0 {
		lipgloss.Printf("%s No skill groups found in Port.\n", styles.QuestionMark)
	}

	preselected := skills.PreselectedGroupIDs(catalogGroups)
	selectedGroups, err := promptGroupSelectionCatalog(catalogGroups, preselected)
	if err != nil {
		return skills.LoadSkillsOptions{}, nil, err
	}

	includeGroups, excludeGroups := skills.GroupSelectionFromCatalog(catalogGroups, selectedGroups)

	fetched, err := mod.FetchSkillsWithQuery(ctx, skills.FetchSkillsQuery{
		IncludeGroups:  includeGroups,
		ExcludeGroups:  excludeGroups,
		TeamsDefault:   true,
	})
	if err != nil {
		return skills.LoadSkillsOptions{}, nil, fmt.Errorf("failed to fetch skills from Port: %w", err)
	}

	var ungroupedSkills []skills.Skill
	for _, s := range fetched.Skills {
		if len(s.GroupIDs) == 0 {
			ungroupedSkills = append(ungroupedSkills, s)
		}
	}

	selectAllUngrouped, selectedSkills, err := promptUngroupedSelection(ungroupedSkills)
	if err != nil {
		return skills.LoadSkillsOptions{}, nil, err
	}

	return skills.LoadSkillsOptions{
		TeamGroupDefaults:  true,
		IncludeGroups:      includeGroups,
		ExcludeGroups:      excludeGroups,
		SelectAllUngrouped: selectAllUngrouped,
		SelectedSkills:     selectedSkills,
	}, fetched, nil
}

func promptGroupSelectionCatalog(groups []aiservice.SkillGroupCatalogEntry, initialSelected []string) ([]string, error) {
	if len(groups) == 0 {
		return nil, nil
	}

	selected := append([]string(nil), initialSelected...)
	if len(initialSelected) > 0 {
		lipgloss.Printf(
			"\n%s Pre-selected %d group(s) owned by your team(s). Adjust the selection below.\n\n",
			styles.CheckMark,
			len(initialSelected),
		)
	}

	groupOptions := make([]huh.Option[string], 0, len(groups))
	for _, g := range groups {
		groupOptions = append(groupOptions, huh.NewOption(groupCatalogLabel(g), g.Identifier))
	}
	pickForm := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which skill groups would you like to sync?").
				Description("Groups owned by your teams are pre-selected. Use space to select/deselect, enter to confirm.").
				Options(groupOptions...).
				Height(len(groupOptions) + 4).
				Value(&selected),
		),
	).WithHeight(0).WithTheme(&styles.FormTheme{})
	if err := pickForm.Run(); err != nil {
		return nil, fmt.Errorf("prompt error: %w", err)
	}

	selectedSet := toStringSet(selected)
	lipgloss.Printf("\n%s Groups:\n", styles.CheckMark)
	for _, g := range groups {
		if selectedSet[g.Identifier] {
			lipgloss.Printf("  %s %s\n", styles.CheckMark, groupCatalogLabel(g))
		} else {
			lipgloss.Printf("  %s %s\n", styles.Circle, groupCatalogLabel(g))
		}
	}
	fmt.Println()

	return selected, nil
}

func groupCatalogLabel(g aiservice.SkillGroupCatalogEntry) string {
	title := strings.TrimSpace(g.Title)
	if title != "" && title != g.Identifier {
		return fmt.Sprintf("%s (%s)", title, g.Identifier)
	}
	return g.Identifier
}

func promptUngroupedSelection(ungroupedSkills []skills.Skill) (selectAll bool, selected []string, err error) {
	if len(ungroupedSkills) == 0 {
		return false, nil, nil
	}

	syncAll := false
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Sync all skills without a group?").
				Description(fmt.Sprintf("%d skill(s) are not part of any group. Yes = sync all, No = pick specific ones.", len(ungroupedSkills))).
				Value(&syncAll),
		),
	).WithTheme(&styles.FormTheme{})
	if err := form.Run(); err != nil {
		return false, nil, fmt.Errorf("prompt error: %w", err)
	}

	if syncAll {
		lipgloss.Printf("\n%s All ungrouped skills selected:\n", styles.CheckMark)
		for _, s := range ungroupedSkills {
			lipgloss.Printf("  %s %s\n", styles.CheckMark, skillLabel(s))
		}
		fmt.Println()
		return true, nil, nil
	}

	skillOptions := make([]huh.Option[string], 0, len(ungroupedSkills))
	for _, s := range ungroupedSkills {
		skillOptions = append(skillOptions, huh.NewOption(skillLabel(s), s.Identifier))
	}
	pickForm := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which ungrouped skills would you like to sync?").
				Description("These skills have no group. Use space to select/deselect, enter to confirm.").
				Options(skillOptions...).
				Height(len(skillOptions) + 4).
				Value(&selected),
		),
	).WithHeight(0).WithTheme(&styles.FormTheme{})
	if err := pickForm.Run(); err != nil {
		return false, nil, fmt.Errorf("prompt error: %w", err)
	}

	selectedSet := toStringSet(selected)
	lipgloss.Printf("\n%s Ungrouped skills:\n", styles.CheckMark)
	for _, s := range ungroupedSkills {
		if selectedSet[s.Identifier] {
			lipgloss.Printf("  %s %s\n", styles.CheckMark, skillLabel(s))
		} else {
			lipgloss.Printf("  %s %s\n", styles.Circle, skillLabel(s))
		}
	}
	fmt.Println()

	return false, selected, nil
}

func groupLabel(g skills.SkillGroup) string {
	if g.Title != "" {
		return g.Title
	}
	return g.Identifier
}

func skillLabel(s skills.Skill) string {
	if s.Title != "" {
		return s.Title
	}
	return s.Identifier
}

func toStringSet(slice []string) map[string]bool {
	s := make(map[string]bool, len(slice))
	for _, v := range slice {
		s[v] = true
	}
	return s
}

func valueOrNone(s string) string {
	if s == "" {
		return "(not set)"
	}
	return s
}

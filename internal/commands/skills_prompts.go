package commands

import (
	"context"
	"fmt"
	"strings"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
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

// buildLoadSkillsOpts fetches the catalog from ai-service for interactive selection
// and returns LoadSkillsOptions plus the same FetchedSkills for LoadSkills to reuse.
func buildLoadSkillsOpts(ctx context.Context, mod *skills.Module, promptSelection bool) (skills.LoadSkillsOptions, *skills.FetchedSkills, error) {
	if !promptSelection {
		return skills.LoadSkillsOptions{}, nil, nil
	}

	fetched, err := mod.FetchSkills(ctx)
	if err != nil {
		return skills.LoadSkillsOptions{}, nil, fmt.Errorf("failed to fetch skills from Port: %w", err)
	}

	if len(fetched.Required) > 0 {
		requiredNames := make([]string, 0, len(fetched.Required))
		for _, s := range fetched.Required {
			name := s.Title
			if name == "" {
				name = s.Identifier
			}
			requiredNames = append(requiredNames, name)
		}
		lipgloss.Printf(
			"\n%s Required skills (always synced regardless of selection):\n  %s\n\n",
			styles.CheckMark,
			strings.Join(requiredNames, ", "),
		)
	}

	if len(fetched.Optional) == 0 && len(fetched.Groups) == 0 {
		lipgloss.Printf("%s No optional skills found — only required skills will be synced.\n", styles.QuestionMark)
		return skills.LoadSkillsOptions{}, fetched, nil
	}

	var requiredGroups, optionalGroups []skills.SkillGroup
	for _, g := range fetched.Groups {
		if g.Required {
			requiredGroups = append(requiredGroups, g)
		} else {
			optionalGroups = append(optionalGroups, g)
		}
	}

	if len(requiredGroups) > 0 {
		requiredGroupNames := make([]string, 0, len(requiredGroups))
		for _, g := range requiredGroups {
			requiredGroupNames = append(requiredGroupNames, groupLabel(g))
		}
		lipgloss.Printf(
			"%s Required groups (always synced regardless of selection): %s\n\n",
			styles.CheckMark,
			strings.Join(requiredGroupNames, ", "),
		)
	}

	selectAllGroups, selectedGroups, err := promptGroupSelection(optionalGroups)
	if err != nil {
		return skills.LoadSkillsOptions{}, nil, err
	}

	var ungroupedSkills []skills.Skill
	for _, s := range fetched.Optional {
		if len(s.GroupIDs) == 0 {
			ungroupedSkills = append(ungroupedSkills, s)
		}
	}

	selectAllUngrouped, selectedSkills, err := promptUngroupedSelection(ungroupedSkills)
	if err != nil {
		return skills.LoadSkillsOptions{}, nil, err
	}

	return skills.LoadSkillsOptions{
		SelectAllGroups:    selectAllGroups,
		SelectAllUngrouped: selectAllUngrouped,
		SelectedGroups:     selectedGroups,
		SelectedSkills:     selectedSkills,
	}, fetched, nil
}

func promptGroupSelection(groups []skills.SkillGroup) (selectAll bool, selected []string, err error) {
	if len(groups) == 0 {
		return false, nil, nil
	}

	syncAll := false
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Sync all skill groups?").
				Description(fmt.Sprintf("%d group(s) available. Yes = sync all groups, No = pick specific groups.", len(groups))).
				Value(&syncAll),
		),
	).WithTheme(&styles.FormTheme{})
	if err := form.Run(); err != nil {
		return false, nil, fmt.Errorf("prompt error: %w", err)
	}

	if syncAll {
		lipgloss.Printf("\n%s All groups selected:\n", styles.CheckMark)
		for _, g := range groups {
			lipgloss.Printf("  %s %s\n", styles.CheckMark, groupLabel(g))
		}
		fmt.Println()
		return true, nil, nil
	}

	groupOptions := make([]huh.Option[string], 0, len(groups))
	for _, g := range groups {
		groupOptions = append(groupOptions, huh.NewOption(groupLabel(g), g.Identifier))
	}
	pickForm := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which skill groups would you like to sync?").
				Description("Use space to select/deselect, enter to confirm.").
				Options(groupOptions...).
				Height(len(groupOptions) + 4).
				Value(&selected),
		),
	).WithHeight(0).WithTheme(&styles.FormTheme{})
	if err := pickForm.Run(); err != nil {
		return false, nil, fmt.Errorf("prompt error: %w", err)
	}

	selectedSet := toStringSet(selected)
	lipgloss.Printf("\n%s Groups:\n", styles.CheckMark)
	for _, g := range groups {
		if selectedSet[g.Identifier] {
			lipgloss.Printf("  %s %s\n", styles.CheckMark, groupLabel(g))
		} else {
			lipgloss.Printf("  %s %s\n", styles.Circle, groupLabel(g))
		}
	}
	fmt.Println()

	return false, selected, nil
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

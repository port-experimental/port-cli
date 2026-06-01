package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/port-experimental/port-cli/internal/modules/skills"
	"github.com/port-experimental/port-cli/internal/styles"
)

func printLoadResult(result *skills.LoadSkillsResult) {
	total := result.RequiredCount + result.SelectedCount
	fmt.Fprintf(os.Stderr,
		"%s %d skill(s) synced (%d required, %d selected)\n",
		styles.CheckMark,
		total,
		result.RequiredCount,
		result.SelectedCount,
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

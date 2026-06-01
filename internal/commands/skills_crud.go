package commands

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/port-experimental/port-cli/internal/modules/skills"
	"github.com/port-experimental/port-cli/internal/styles"
	"github.com/spf13/cobra"
)

func registerSkillsCreate() *cobra.Command {
	var (
		identifier  string
		title       string
		description string
		location    string
		published   bool
	)

	cmd := &cobra.Command{
		Use:   "create <path-to-skill-folder>",
		Short: "Create a Port skill from a local skill directory",
		Long: `Create a new _skill entity and initial _skill_version via Port ai-service.

The folder must include SKILL.md at its root. All files under the folder are
uploaded with paths relative to the folder root. The skill identifier defaults
to the folder name unless --identifier is set or SKILL.md frontmatter defines name.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			flags := GetGlobalFlags(ctx)
			mod, _, err := newSkillsModuleWithFlags(ctx, flags, skillsOrgName(cmd))
			if err != nil {
				return err
			}

			result, err := mod.CreateSkillFromFolder(ctx, args[0], skills.PackSkillFolderOptions{
				Identifier:  identifier,
				Title:       title,
				Description: description,
				Location:    location,
			}, published)
			if err != nil {
				return err
			}

			lipgloss.Printf("%s Created skill %s (version %s, %s)\n",
				styles.CheckMark,
				styles.Bold.Render(result.SkillIdentifier),
				result.Version,
				result.ReleaseState,
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&identifier, "identifier", "", "Skill identifier (default: folder name or SKILL.md name)")
	cmd.Flags().StringVar(&title, "title", "", "Skill title (default: identifier or SKILL.md title)")
	cmd.Flags().StringVar(&description, "description", "", "Skill description (default: SKILL.md frontmatter)")
	cmd.Flags().StringVar(&location, "location", "", "Skill location: global or project (default: global)")
	cmd.Flags().BoolVar(&published, "published", false, "Publish the initial version immediately")
	return cmd
}

func registerSkillsEdit() *cobra.Command {
	var (
		title       string
		description string
		location    string
		published   bool
	)

	cmd := &cobra.Command{
		Use:   "edit <skill-identifier> <path-to-skill-folder>",
		Short: "Edit a Port skill by uploading a new version from a local folder",
		Long: `Create a new semver patch _skill_version for an existing skill via Port ai-service.

The folder must include SKILL.md at its root. File paths are relative to the folder root.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			flags := GetGlobalFlags(ctx)
			mod, _, err := newSkillsModuleWithFlags(ctx, flags, skillsOrgName(cmd))
			if err != nil {
				return err
			}

			result, err := mod.EditSkillFromFolder(ctx, args[0], args[1], skills.PackSkillFolderOptions{
				Title:       title,
				Description: description,
				Location:    location,
			}, published)
			if err != nil {
				return err
			}

			lipgloss.Printf("%s Updated skill %s (version %s, %s)\n",
				styles.CheckMark,
				styles.Bold.Render(result.SkillIdentifier),
				result.Version,
				result.ReleaseState,
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Override skill title on the _skill entity")
	cmd.Flags().StringVar(&description, "description", "", "Version description (default: SKILL.md frontmatter)")
	cmd.Flags().StringVar(&location, "location", "", "Override skill location: global or project")
	cmd.Flags().BoolVar(&published, "published", false, "Publish this version (unpublishes other published versions)")
	return cmd
}

func registerSkillsArchive() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive <skill-identifier>",
		Short: "Archive all versions of a Port skill",
		Long:  `Sets release_state to archived on all versions of the skill via Port ai-service.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			flags := GetGlobalFlags(ctx)
			mod, _, err := newSkillsModuleWithFlags(ctx, flags, skillsOrgName(cmd))
			if err != nil {
				return err
			}

			result, err := mod.ArchiveSkill(ctx, args[0])
			if err != nil {
				return err
			}

			lipgloss.Printf("%s Archived skill %s (%d version(s))\n",
				styles.CheckMark,
				styles.Bold.Render(result.SkillIdentifier),
				result.VersionsArchived,
			)
			return nil
		},
	}
	return cmd
}

func registerSkillsList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Port skill entity identifiers",
		Long: `List _skill entity identifiers in your Port organization via Port ai-service.

This is a read-only command — it does not sync or modify any local files.
Use 'port skills sync' to download skill files to your machine.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			flags := GetGlobalFlags(ctx)

			mod, _, err := newSkillsModuleWithFlags(ctx, flags, skillsOrgName(cmd))
			if err != nil {
				return err
			}

			ids, err := mod.ListSkillIdentifiers(ctx)
			if err != nil {
				return fmt.Errorf("failed to list skills: %w", err)
			}

			if len(ids) == 0 {
				fmt.Println("No skills found.")
				return nil
			}
			for _, id := range ids {
				fmt.Println(id)
			}
			return nil
		},
	}
}

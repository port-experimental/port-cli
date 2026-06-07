package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/port-experimental/port-cli/internal/api/aiservice"
	"github.com/port-experimental/port-cli/internal/modules/skills"
	"github.com/port-experimental/port-cli/internal/styles"
	"github.com/spf13/cobra"
)

func registerSkillsUpload() *cobra.Command {
	var (
		identifier  string
		title       string
		description string
		location    string
		publish     bool
		published   bool
		versionBump string
	)

	cmd := &cobra.Command{
		Use:   "upload <path-to-skill-folder-or-bundle>",
		Short: "Upload Port skill(s) from local skill directories (create or new version)",
		Long: `Upload skill content to Port via ai-service (upsert).

Accepts either a single skill directory (SKILL.md at the root) or a bundle directory
that contains one or more skill folders at any depth (e.g. ./claude/skills). Symlinks
to skill directories are supported. Search stops at each folder that contains SKILL.md.

The folder name must match the SKILL.md frontmatter name: field when present.
Re-uploading an existing skill creates a new patch version instead of failing.

Non-interactive example:
  port skills upload ./my-skill --identifier my-skill --publish --location project
  port skills upload ./claude/skills`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			flags := GetGlobalFlags(ctx)
			mod, _, err := newSkillsModuleWithFlags(ctx, flags, skillsOrgName(cmd))
			if err != nil {
				return err
			}

			roots, err := skills.DiscoverSkillRoots(args[0])
			if err != nil {
				return err
			}

			packLocation, err := packSkillLocationFromFlag(cmd, location)
			if err != nil {
				return err
			}

			publishFlag := resolveSkillsPublishFlag(cmd, publish, published)
			bump, err := parseVersionBump(versionBump)
			if err != nil {
				return err
			}
			writeOpts := skills.UploadSkillWriteOptions{
				Publish:     publishFlag,
				VersionBump: bump,
			}

			packOpts := skills.PackSkillFolderOptions{
				Title:       title,
				Description: description,
				Location:    packLocation,
			}

			if len(roots) == 1 {
				if identifier != "" {
					packOpts.Identifier = identifier
				}
				result, err := mod.UploadSkillFromFolder(ctx, roots[0], packOpts, writeOpts)
				if err != nil {
					return err
				}
				printSkillUploadSuccess(result, packLocation, publishFlag)
				return nil
			}

			if identifier != "" {
				return fmt.Errorf("--identifier cannot be used when uploading multiple skills from a bundle directory")
			}

			packs := make([]skills.SkillPackWithFolder, 0, len(roots))
			for _, root := range roots {
				pack, err := skills.PackSkillFolder(root, packOpts)
				if err != nil {
					return fmt.Errorf("%s: %w", root, err)
				}
				packs = append(packs, skills.SkillPackWithFolder{
					Pack:       pack,
					FolderBase: filepath.Base(root),
				})
			}

			batch, err := mod.UploadSkillsBatch(ctx, packs, writeOpts)
			if err != nil {
				return err
			}
			return printBatchUploadResults(batch)
		},
	}

	cmd.Flags().StringVar(&identifier, "identifier", "", "Skill identifier for single-skill upload (must match folder name)")
	cmd.Flags().StringVar(&title, "title", "", "Skill title (default: identifier or SKILL.md title)")
	cmd.Flags().StringVar(&description, "description", "", "Skill description (default: SKILL.md frontmatter)")
	cmd.Flags().StringVar(&location, "location", "global", "Skill location: global or project (default: global; SKILL.md frontmatter used when flag omitted)")
	cmd.Flags().BoolVar(&publish, "publish", false, "Set the new version as the skill active version")
	cmd.Flags().BoolVar(&published, "published", false, "Deprecated alias for --publish")
	cmd.Flags().StringVar(&versionBump, "version-bump", "patch", "Semver increment for the new version: patch, minor, or major")
	_ = cmd.Flags().MarkHidden("published")
	return cmd
}

func registerSkillsPublish() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish <skill-identifier>",
		Short: "Publish the latest version of a Port skill",
		Long: `Sets skill_active_version on the _skill entity to the latest _skill_version by semver.

Does not upload new files. Use 'port skills upload' to create a new version from local files.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			flags := GetGlobalFlags(ctx)
			mod, _, err := newSkillsModuleWithFlags(ctx, flags, skillsOrgName(cmd))
			if err != nil {
				return err
			}
			result, err := mod.PublishSkill(ctx, args[0])
			if err != nil {
				return err
			}
			lipgloss.Printf("%s Published skill %s (version %s)\n",
				styles.CheckMark,
				styles.Bold.Render(result.SkillIdentifier),
				result.Version,
			)
			return nil
		},
	}
	return cmd
}

func registerSkillsLoad() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "load <skill-identifier>",
		Short: "Download one published skill to configured local targets",
		Long:  `Fetches a skill from Port ai-service and writes it under each configured tool's skills/port/ tree.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			flags := GetGlobalFlags(ctx)
			mod, _, err := newSkillsModuleWithFlags(ctx, flags, skillsOrgName(cmd))
			if err != nil {
				return err
			}
			if err := mod.LoadSkillToLocal(ctx, args[0]); err != nil {
				return err
			}
			lipgloss.Printf("%s Loaded skill %s to configured targets\n", styles.CheckMark, styles.Bold.Render(args[0]))
			return nil
		},
	}
	return cmd
}

func registerSkillsUnload() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unload <skill-identifier>",
		Short: "Remove a skill from local skills/port/ directories",
		Long:  `Deletes local copies of the skill under skills/port/ for each configured target. Does not change Port.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := GetGlobalFlags(cmd.Context())
			mod, _, err := newSkillsModuleWithFlags(cmd.Context(), flags, skillsOrgName(cmd))
			if err != nil {
				return err
			}
			if err := mod.UnloadSkillLocal(args[0]); err != nil {
				return err
			}
			lipgloss.Printf("%s Removed local skill %s\n", styles.CheckMark, styles.Bold.Render(args[0]))
			return nil
		},
	}
	return cmd
}

func registerSkillsUnpublish() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unpublish <skill-identifier>",
		Short: "Clear the active version for a Port skill",
		Long:  `Clears skill_active_version on the _skill entity so the skill is no longer published.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			flags := GetGlobalFlags(ctx)
			mod, _, err := newSkillsModuleWithFlags(ctx, flags, skillsOrgName(cmd))
			if err != nil {
				return err
			}
			if err := mod.UnpublishSkill(ctx, args[0]); err != nil {
				return err
			}
			lipgloss.Printf("%s Unpublished skill %s\n", styles.CheckMark, styles.Bold.Render(args[0]))
			return nil
		},
	}
	return cmd
}

// packSkillLocationFromFlag returns a location for PackSkillFolder when --location was set.
func packSkillLocationFromFlag(cmd *cobra.Command, location string) (string, error) {
	if cmd == nil || !cmd.Flags().Changed("location") {
		return "", nil
	}
	return skills.NormalizeSkillLocation(location)
}

func parseVersionBump(value string) (aiservice.VersionBump, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "patch":
		return aiservice.VersionBumpPatch, nil
	case "minor":
		return aiservice.VersionBumpMinor, nil
	case "major":
		return aiservice.VersionBumpMajor, nil
	default:
		return "", fmt.Errorf("version-bump must be patch, minor, or major")
	}
}

func resolveSkillsPublishFlag(cmd *cobra.Command, publish, published bool) bool {
	if cmd != nil && cmd.Flags().Changed("publish") {
		return publish
	}
	if cmd != nil && cmd.Flags().Changed("published") {
		return published
	}
	return publish || published
}

func skillActiveVersionStatus(active bool) string {
	if active {
		return "active version set"
	}
	return "active version unchanged"
}

func printSkillUploadSuccess(result *aiservice.SkillVersionWriteResponse, location string, publish bool) {
	locLabel := ""
	if location != "" {
		locLabel = ", location " + styles.Faint.Render(location)
	}
	_ = publish
	lipgloss.Printf("%s Uploaded skill %s (version %s, %s%s)\n",
		styles.CheckMark,
		styles.Bold.Render(result.SkillIdentifier),
		result.Version,
		skillActiveVersionStatus(result.ActiveVersionSet),
		locLabel,
	)
}

func printBatchUploadResults(batch *aiservice.BatchUploadSkillsResponse) error {
	if batch == nil {
		return fmt.Errorf("empty batch upload response")
	}

	var failed []string
	for _, item := range batch.Results {
		if item.OK {
			version := ""
			if item.Result != nil {
				version = item.Result.Version
			}
			lipgloss.Printf("%s Uploaded skill %s (version %s)\n",
				styles.CheckMark,
				styles.Bold.Render(item.Identifier),
				version,
			)
			continue
		}
		msg := item.Identifier
		if item.Error != nil {
			msg = fmt.Sprintf("%s: %s", item.Identifier, item.Error.Message)
		}
		failed = append(failed, msg)
		lipgloss.Fprintf(os.Stderr, "%s Failed %s\n", styles.ExclamationMark, msg)
	}

	if len(failed) == 0 {
		return nil
	}
	return fmt.Errorf("failed to upload %d skill(s): %s", len(failed), strings.Join(failed, "; "))
}

func registerSkillsList() *cobra.Command {
	var (
		jsonOut            bool
		page               int
		pageSize           int
		includeUnpublished bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Port skills in your organization",
		Long: `List skills in your Port organization via Port ai-service.

Shows each skill's identifier, title, location, and resolved version. Results are
paginated (default 20 per page). In an interactive terminal, use n/p/q to move
between pages after each page is shown.

This is a read-only command — it does not sync or modify any local files.
Use 'port skills sync' to download skill files to your machine.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			flags := GetGlobalFlags(ctx)

			mod, _, err := newSkillsModuleWithFlags(ctx, flags, skillsOrgName(cmd))
			if err != nil {
				return err
			}

			query := aiservice.GetSkillsSummaryQuery{
				Page:               page,
				PageSize:           pageSize,
				IncludeUnpublished: includeUnpublished,
			}
			if query.Page <= 0 {
				query.Page = 1
			}
			if query.PageSize <= 0 {
				query.PageSize = 20
			}

			interactivePaging := IsInteractive() && !cmd.Flags().Changed("page") && !jsonOut
			for {
				resp, err := mod.ListSkills(ctx, query)
				if err != nil {
					return fmt.Errorf("failed to list skills: %w", err)
				}
				if len(resp.Skills) == 0 && resp.Pagination.Total == 0 {
					fmt.Println("No skills found.")
					return nil
				}
				if jsonOut {
					return printSkillsCatalogJSON(resp)
				}

				printSkillsCatalog(resp.Skills)
				printSkillsListPagination(resp.Pagination, len(resp.Skills))

				if !interactivePaging {
					return nil
				}
				if !resp.Pagination.HasNextPage && !resp.Pagination.HasPreviousPage {
					return nil
				}

				action, err := promptSkillsListPageNav(resp.Pagination.HasPreviousPage, resp.Pagination.HasNextPage)
				if err != nil {
					return err
				}
				switch action {
				case "next":
					query.Page++
				case "prev":
					if query.Page > 1 {
						query.Page--
					}
				default:
					return nil
				}
			}
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print catalog page as JSON (includes pagination metadata)")
	cmd.Flags().IntVar(&page, "page", 0, "Page number to fetch (1-based; default 1)")
	cmd.Flags().IntVar(&pageSize, "page-size", 0, "Skills per page (default 20, max 100)")
	cmd.Flags().BoolVar(&includeUnpublished, "include-unpublished", false, "Include skills without an active published version")
	return cmd
}

func registerSkillsSearch() *cobra.Command {
	var (
		jsonOut bool
		limit   int
	)

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search Port skills by identifier or title",
		Long: `Search skills in your Port organization via Port ai-service.

Matches the query as a case-insensitive substring against each skill's
identifier and title. Use quotes in the shell for multi-word queries, or pass
words as separate arguments (they are joined with spaces).

Examples:
  port skills search api
  port skills search demo onboard
  port skills search "api guide" --json`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			flags := GetGlobalFlags(ctx)
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return fmt.Errorf("search query is required")
			}

			mod, _, err := newSkillsModuleWithFlags(ctx, flags, skillsOrgName(cmd))
			if err != nil {
				return err
			}

			entries, err := mod.SearchSkills(ctx, aiservice.SearchSkillsQuery{
				Query: query,
				Limit: limit,
			})
			if err != nil {
				return fmt.Errorf("failed to search skills: %w", err)
			}

			if len(entries) == 0 {
				fmt.Printf("No skills matching %q.\n", query)
				return nil
			}
			if jsonOut {
				return printSkillsCatalogJSON(&aiservice.SkillsSummaryResponse{
					OK:     true,
					Skills: entries,
				})
			}
			printSkillsSearchResults(entries, query)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print matches as JSON")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of skills to return (0 = no limit)")
	return cmd
}

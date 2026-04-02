package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/config"
	"github.com/port-experimental/port-cli/internal/output"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// RegisterClear registers the clear command.
func RegisterClear(rootCmd *cobra.Command) {
	var (
		org             string
		clearPages      bool
		deleteProtected bool
		force           bool
	)

	clearCmd := &cobra.Command{
		Use:   "clear",
		Short: "Delete selected resources from a Port organization",
		Long: `Delete selected resources from a Port organization.

Deletion is opt-in by resource type. For example:
  port clear --pages

For now, only pages are supported. Clearing pages deletes root pages and root
folders. The UI deletes descendants recursively, but the API still differs by
resource type, so pages and folders use different endpoints. Items whose
identifiers contain underscores are treated as protected by default and are
skipped unless --delete-protected is provided.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !clearPages {
				return fmt.Errorf("no resource types selected. Use at least one flag such as --pages")
			}

			if !force {
				selection := []string{}
				if clearPages {
					selection = append(selection, "pages")
				}
				cmd.Printf("Delete %s from organization %q? [y/N]: ", strings.Join(selection, ", "), org)
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					cmd.Println("Operation cancelled")
					return nil
				}
			}

			flags := GetGlobalFlags(cmd.Context())
			configManager := config.NewConfigManager(flags.ConfigFile)

			cfg, err := configManager.LoadWithOverrides(
				flags.ClientID,
				flags.ClientSecret,
				flags.APIURL,
				org,
			)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			orgConfig, err := cfg.GetOrgConfig(org)
			if err != nil {
				return err
			}

			client := api.NewClient(api.ClientOpts{
				ClientID:     orgConfig.ClientID,
				ClientSecret: orgConfig.ClientSecret,
				APIURL:       orgConfig.APIURL,
			})
			defer client.Close()

			if clearPages {
				if err := clearPagesAndFolders(cmd, client, deleteProtected); err != nil {
					return err
				}
			}

			return nil
		},
	}

	clearCmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	clearCmd.Flags().BoolVar(&clearPages, "pages", false, "Delete root pages and root folders")
	clearCmd.Flags().BoolVar(&deleteProtected, "delete-protected", false, "Also delete protected pages and folders whose identifiers contain underscores, after non-protected items")
	clearCmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")

	rootCmd.AddCommand(clearCmd)
}

func clearPagesAndFolders(cmd *cobra.Command, client *api.Client, deleteProtected bool) error {
	ctx := cmd.Context()

	pages, err := client.GetPages(ctx)
	if err != nil {
		return fmt.Errorf("failed to list pages: %w", err)
	}

	folders, err := client.GetFolders(ctx)
	if err != nil {
		return fmt.Errorf("failed to list folders: %w", err)
	}

	rootFolders := rootFoldersForDeletion(folders, deleteProtected)
	rootPages := rootPagesForDeletion(pages, deleteProtected)
	rootFolders, protectedRootFolders := partitionProtectedFolders(rootFolders)
	rootPages, protectedRootPages := partitionProtectedPages(rootPages)

	deletedRootFolders, deletedRootPages, err := deleteRootSidebarItems(ctx, client, rootFolders, rootPages)
	if err != nil {
		return err
	}
	if deleteProtected {
		deletedProtectedFolders, deletedProtectedPages, err := deleteRootSidebarItems(ctx, client, protectedRootFolders, protectedRootPages)
		if err != nil {
			return err
		}
		deletedRootFolders += deletedProtectedFolders
		deletedRootPages += deletedProtectedPages
	}

	output.SuccessPrint("Deleted %d root pages and %d root folders\n", deletedRootPages, deletedRootFolders)
	return nil
}

func deleteRootSidebarItems(ctx context.Context, client *api.Client, folders []api.Folder, pages []api.Page) (int, int, error) {
	var (
		deletedRootFolders int
		deletedRootPages   int
	)

	g, groupCtx := errgroup.WithContext(ctx)
	g.SetLimit(8)

	for _, folder := range folders {
		folderID, _ := folder["identifier"].(string)
		if folderID == "" {
			continue
		}
		g.Go(func() error {
			if err := client.DeleteFolder(groupCtx, folderID); err != nil {
				return fmt.Errorf("failed to delete folder %q: %w", folderID, err)
			}
			output.Printf("Deleted root folder: %s\n", folderID)
			return nil
		})
		deletedRootFolders++
	}

	for _, page := range pages {
		pageID, _ := page["identifier"].(string)
		if pageID == "" {
			continue
		}
		g.Go(func() error {
			if err := client.DeletePage(groupCtx, pageID); err != nil {
				return fmt.Errorf("failed to delete page %q: %w", pageID, err)
			}
			output.Printf("Deleted root page: %s\n", pageID)
			return nil
		})
		deletedRootPages++
	}

	if err := g.Wait(); err != nil {
		return 0, 0, err
	}

	return deletedRootFolders, deletedRootPages, nil
}

func rootPagesForDeletion(pages []api.Page, deleteProtected bool) []api.Page {
	roots := make([]api.Page, 0, len(pages))
	protectedRoots := make([]api.Page, 0, len(pages))
	for _, page := range pages {
		if !isDeletablePage(page) || !isRootSidebarItem(map[string]interface{}(page)) {
			continue
		}
		if isProtectedSidebarItemIdentifier(page["identifier"]) {
			if deleteProtected {
				protectedRoots = append(protectedRoots, page)
			}
			continue
		}
		roots = append(roots, page)
	}
	return append(roots, protectedRoots...)
}

func rootFoldersForDeletion(folders []api.Folder, deleteProtected bool) []api.Folder {
	roots := make([]api.Folder, 0, len(folders))
	protectedRoots := make([]api.Folder, 0, len(folders))
	for _, folder := range folders {
		if !isRootSidebarItem(map[string]interface{}(folder)) {
			continue
		}
		if isProtectedSidebarItemIdentifier(folder["identifier"]) {
			if deleteProtected {
				protectedRoots = append(protectedRoots, folder)
			}
			continue
		}
		roots = append(roots, folder)
	}
	return append(roots, protectedRoots...)
}

func partitionProtectedPages(pages []api.Page) ([]api.Page, []api.Page) {
	regular := make([]api.Page, 0, len(pages))
	protected := make([]api.Page, 0, len(pages))
	for _, page := range pages {
		if isProtectedSidebarItemIdentifier(page["identifier"]) {
			protected = append(protected, page)
			continue
		}
		regular = append(regular, page)
	}
	return regular, protected
}

func partitionProtectedFolders(folders []api.Folder) ([]api.Folder, []api.Folder) {
	regular := make([]api.Folder, 0, len(folders))
	protected := make([]api.Folder, 0, len(folders))
	for _, folder := range folders {
		if isProtectedSidebarItemIdentifier(folder["identifier"]) {
			protected = append(protected, folder)
			continue
		}
		regular = append(regular, folder)
	}
	return regular, protected
}

func isRootSidebarItem(item map[string]interface{}) bool {
	parent, exists := item["parent"]
	if !exists || parent == nil {
		return true
	}
	parentID, ok := parent.(string)
	return !ok || parentID == ""
}

func isDeletablePage(page api.Page) bool {
	showInSidebar, ok := page["showInSidebar"].(bool)
	return ok && showInSidebar
}

func isProtectedSidebarItemIdentifier(identifier interface{}) bool {
	id, ok := identifier.(string)
	if !ok || id == "" {
		return false
	}
	return strings.Contains(id, "_")
}

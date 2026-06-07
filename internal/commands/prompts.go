package commands

import (
	"errors"
	"fmt"
	"os"

	"charm.land/huh/v2"
	"github.com/charmbracelet/x/term"
	"github.com/port-experimental/port-cli/internal/styles"
	"github.com/spf13/cobra"
)

// ErrNonInteractiveRequired is returned when a command would prompt but stdin is not a TTY
// and no non-interactive bypass flags were provided.
var ErrNonInteractiveRequired = errors.New("non-interactive mode required: provide flags to skip prompts or run in a terminal")

// IsInteractive returns true when huh prompts can be shown.
func IsInteractive() bool {
	return term.IsTerminal(os.Stdin.Fd())
}

// RequireInteractive fails when not a TTY (for CI/scripts).
func RequireInteractive() error {
	if !IsInteractive() {
		return ErrNonInteractiveRequired
	}
	return nil
}

// ShouldSkipConfirm returns true when the user passed --force, --yes, or global --yes.
func ShouldSkipConfirm(cmd *cobra.Command, force bool) bool {
	if force {
		return true
	}
	if yes, _ := cmd.Flags().GetBool("yes"); yes {
		return true
	}
	if yes, _ := cmd.Root().PersistentFlags().GetBool("yes"); yes {
		return true
	}
	flags := GetGlobalFlags(cmd.Context())
	return flags.Yes
}

// promptSkillsListPageNav asks how to move through a paginated skills list.
// Returns "next", "prev", or "quit".
func promptSkillsListPageNav(hasPrev, hasNext bool) (string, error) {
	if err := RequireInteractive(); err != nil {
		return "", err
	}

	var options []huh.Option[string]
	if hasPrev {
		options = append(options, huh.NewOption("Previous page", "prev"))
	}
	if hasNext {
		options = append(options, huh.NewOption("Next page", "next"))
	}
	options = append(options, huh.NewOption("Quit", "quit"))

	var choice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Show another page?").
				Options(options...).
				Value(&choice),
		),
	).WithTheme(&styles.FormTheme{})
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("prompt error: %w", err)
	}
	if choice == "" {
		return "quit", nil
	}
	return choice, nil
}

// confirmPrompt shows a yes/no confirmation and returns whether the user accepted.
func confirmPrompt(title, description string) (bool, error) {
	if err := RequireInteractive(); err != nil {
		return false, err
	}
	var confirmed bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(title).
				Description(description).
				Value(&confirmed),
		),
	).WithTheme(&styles.FormTheme{})
	if err := form.Run(); err != nil {
		return false, fmt.Errorf("prompt error: %w", err)
	}
	return confirmed, nil
}

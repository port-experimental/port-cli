package styles

import (
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

var (
	Cross           = lipgloss.NewStyle().Foreground(lipgloss.Red).Render("✖︎")
	CheckMark       = lipgloss.NewStyle().Foreground(lipgloss.Green).Render("✔︎")
	QuestionMark    = lipgloss.NewStyle().Foreground(lipgloss.Yellow).Render("?")
	ExclamationMark = lipgloss.NewStyle().Foreground(lipgloss.Yellow).Render("!")
	Bold            = lipgloss.NewStyle().Bold(true)
)

// FormTheme implements huh.Theme using the base theme.
type FormTheme struct{}

func (t *FormTheme) Theme(isDark bool) *huh.Styles {
	return huh.ThemeBase(isDark)
}

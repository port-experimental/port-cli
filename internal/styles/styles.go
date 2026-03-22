package styles

import "charm.land/lipgloss/v2"

var (
	CheckMark    = lipgloss.NewStyle().Foreground(lipgloss.Green).Render("✔︎")
	QuestionMark = lipgloss.NewStyle().Foreground(lipgloss.Yellow).Render("?")
)

package styles

import "charm.land/lipgloss/v2"

var (
	Cross        = lipgloss.NewStyle().Foreground(lipgloss.Red).Render("✖︎")
	CheckMark    = lipgloss.NewStyle().Foreground(lipgloss.Green).Render("✔︎")
	QuestionMark = lipgloss.NewStyle().Foreground(lipgloss.Yellow).Render("?")
	Bold         = lipgloss.NewStyle().Bold(true)
)

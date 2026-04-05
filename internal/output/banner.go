package output

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

const asciiLogo = ` ██████╗   ██████╗  ██████╗  ████████╗
 ██╔══██╗ ██╔═══██╗ ██╔══██╗ ╚══██╔══╝
 ██████╔╝ ██║   ██║ ██████╔╝    ██║
 ██╔═══╝  ██║   ██║ ██╔══██╗    ██║
 ██║      ╚██████╔╝ ██║  ██║    ██║
 ╚═╝       ╚═════╝  ╚═╝  ╚═╝    ╚═╝   `

// VersionInfo holds the fields displayed below the banner.
type VersionInfo struct {
	Version   string
	BuildDate string
	Commit    string
	GoVersion string
	Platform  string
}

// Banner renders a branded ASCII art banner with version information.
// It uses Lipgloss for styling and respects the existing NO_COLOR / TTY
// detection handled by the output package.
func Banner(info VersionInfo) string {
	// -- styles --
	logoStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.White)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.White)
	subtitleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("33")) // blue
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))     // gray / dim

	// -- logo --
	logo := logoStyle.Render(asciiLogo)

	// -- tagline --
	tagline := fmt.Sprintf("%s  %s  %s",
		titleStyle.Render("Port CLI"),
		dimStyle.Render("\u00b7"),
		subtitleStyle.Render("Agentic Engineering Platform"),
	)

	// -- separator (thin line matching logo width) --
	logoWidth := lipgloss.Width(logo)
	separator := dimStyle.Render(strings.Repeat("─", logoWidth))

	// -- version details --
	versionLines := []string{
		fmt.Sprintf("Version:    %s", info.Version),
		fmt.Sprintf("Build date: %s", info.BuildDate),
		fmt.Sprintf("Git commit: %s", info.Commit),
		fmt.Sprintf("Go version: %s", info.GoVersion),
		fmt.Sprintf("Platform:   %s", info.Platform),
	}
	versionBlock := dimStyle.Render(strings.Join(versionLines, "\n"))

	// -- assemble & center --
	inner := lipgloss.JoinVertical(lipgloss.Center,
		logo,
		"",
		tagline,
		separator,
		versionBlock,
	)

	centered := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(logoWidth).
		Render(inner)

	return centered
}

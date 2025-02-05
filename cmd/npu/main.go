package main

import (
	"fmt"
	"os"

	"github.com/GNURub/node-package-updater/cmd"
	"github.com/GNURub/node-package-updater/internal/constants"
	"github.com/GNURub/node-package-updater/pkg/upgrade"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	newVersion := upgrade.GetNewVersion(constants.RepoOwner, constants.RepoName)
	if newVersion != "" {
		titleStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Bold(true).
			Padding(1, 2).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF00FF"))

		versionStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFF00")).
			Bold(true).
			Italic(true)

		commandStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FFFF")).
			Bold(true)

		message := fmt.Sprintf(
			"%s %s\n🚀 %s",
			titleStyle.Render("New version available:"),
			versionStyle.Render(newVersion),
			commandStyle.Render("Run `npu upgrade` to update!"),
		)

		fmt.Println("\n" + message + "\n")
	}

	if err := cmd.Exec(); err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)
		fmt.Println(errorStyle.Render("Error: " + err.Error()))
		os.Exit(1)
	}
}

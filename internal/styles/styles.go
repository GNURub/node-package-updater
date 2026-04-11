package styles

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	ErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#E88388"))
	SuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#A9DC76"))

	MutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#7A7A8C"))
	LabelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#8FA3B6")).Bold(true)
	AccentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF75B7")).Bold(true)
	TitleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#63B0B8")).Bold(true)

	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#63B0B8")).
			Padding(0, 1)

	CardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#3F4757")).
			Padding(0, 1).
			MarginTop(1)
)

var severityPalette = map[string]lipgloss.Color{
	"critical": lipgloss.Color("#FF1744"),
	"high":     lipgloss.Color("#FF6B35"),
	"moderate": lipgloss.Color("#FFB300"),
	"low":      lipgloss.Color("#4FC3F7"),
	"unknown":  lipgloss.Color("#9E9E9E"),
}

// NormalizeSeverity collapses provider-specific severity strings into the
// canonical buckets the UI styles know about.
func NormalizeSeverity(severity string) string {
	s := strings.ToLower(strings.TrimSpace(severity))
	switch s {
	case "":
		return "unknown"
	case "medium":
		return "moderate"
	}
	if _, ok := severityPalette[s]; !ok {
		return "unknown"
	}
	return s
}

// SeverityBadge renders a colored, padded badge for a severity level.
func SeverityBadge(severity string) string {
	key := NormalizeSeverity(severity)
	color := severityPalette[key]
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(color).
		Bold(true).
		Padding(0, 1).
		Render(strings.ToUpper(key))
}

// SeverityForeground returns just the colored severity label, without a
// background — useful inside dense lines where a full badge would be noisy.
func SeverityForeground(severity string) string {
	key := NormalizeSeverity(severity)
	color := severityPalette[key]
	return lipgloss.NewStyle().Foreground(color).Bold(true).Render(strings.ToUpper(key))
}

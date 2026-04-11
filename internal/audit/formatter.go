package audit

import (
	"fmt"
	"sort"
	"strings"

	"github.com/GNURub/node-package-updater/internal/styles"
	"github.com/charmbracelet/lipgloss"
)

const (
	cardWidth  = 80
	labelWidth = 9
)

func formatText(result *Result) string {
	if result == nil {
		return styles.SuccessStyle.Render("✅ No audit result available") + "\n"
	}

	if result.Summary.TotalFindings == 0 {
		return renderCleanReport(result) + "\n"
	}

	return renderVulnReport(result) + "\n"
}

func renderCleanReport(result *Result) string {
	body := lipgloss.JoinVertical(lipgloss.Left,
		styles.TitleStyle.Render("npu audit"),
		styles.SuccessStyle.Bold(true).Render("✅ No known vulnerabilities found"),
		"",
		fieldLine("Project", result.Summary.RootDir),
		fieldLine("Scanned", fmt.Sprintf("%d lockfile(s) · %d directory(ies)",
			result.Summary.LockfilesScanned, result.Summary.DirectoriesScanned)),
	)
	return styles.PanelStyle.Width(cardWidth).Render(body)
}

func renderVulnReport(result *Result) string {
	var b strings.Builder

	b.WriteString(renderHeader(result))
	b.WriteString("\n")

	if bar := renderSeverityBar(result.Findings); bar != "" {
		b.WriteString("\n")
		b.WriteString(bar)
		b.WriteString("\n")
	}

	for _, f := range sortFindingsBySeverity(result.Findings) {
		b.WriteString(renderFindingCard(f))
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

func renderHeader(result *Result) string {
	heroLine := styles.ErrorStyle.Bold(true).Render(
		fmt.Sprintf("🚨 %d vulnerability finding(s) · %d package(s) affected",
			result.Summary.TotalFindings, result.Summary.AffectedPackages),
	)

	body := lipgloss.JoinVertical(lipgloss.Left,
		styles.TitleStyle.Render("npu audit"),
		heroLine,
		"",
		fieldLine("Project", result.Summary.RootDir),
		fieldLine("Scanned", fmt.Sprintf("%d lockfile(s) · %d directory(ies)",
			result.Summary.LockfilesScanned, result.Summary.DirectoriesScanned)),
	)
	return styles.PanelStyle.Width(cardWidth).Render(body)
}

func renderSeverityBar(findings []Finding) string {
	counts := map[string]int{}
	for _, f := range findings {
		counts[styles.NormalizeSeverity(f.Severity)]++
	}

	order := []string{"critical", "high", "moderate", "low", "unknown"}
	var parts []string
	for _, sev := range order {
		if counts[sev] == 0 {
			continue
		}
		badge := styles.SeverityBadge(sev)
		count := styles.MutedStyle.Render(fmt.Sprintf("× %d", counts[sev]))
		parts = append(parts, badge+" "+count)
	}
	if len(parts) == 0 {
		return ""
	}
	return lipgloss.NewStyle().PaddingLeft(1).Render(strings.Join(parts, "   "))
}

func renderFindingCard(f Finding) string {
	pkg := lipgloss.JoinHorizontal(lipgloss.Left,
		styles.AccentStyle.Render(f.PackageName),
		styles.MutedStyle.Render("@"+f.Version),
	)
	header := lipgloss.JoinHorizontal(lipgloss.Left, pkg, "  ", styles.SeverityBadge(f.Severity))

	rows := []string{header, ""}
	rows = append(rows, fieldLine("ID", f.ID))
	rows = append(rows, fieldLine("Source", fmt.Sprintf("%s (%s)", f.SourcePath, f.SourceType)))
	if len(f.Aliases) > 0 {
		rows = append(rows, fieldLine("Aliases", strings.Join(f.Aliases, ", ")))
	}
	if f.Summary != "" {
		rows = append(rows, "", styles.LabelStyle.Render("Summary"), wrap(f.Summary, cardWidth-4))
	}
	if f.Details != "" {
		rows = append(rows, "", styles.LabelStyle.Render("Details"), wrap(truncate(f.Details, 480), cardWidth-4))
	}
	rows = append(rows,
		"",
		styles.SuccessStyle.Bold(true).Render("▸ Suggested fix"),
		wrap("Suggestion: "+f.Recommendation, cardWidth-4),
	)

	return styles.CardStyle.Width(cardWidth).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func fieldLine(label, value string) string {
	return styles.LabelStyle.Render(padLabel(label)) + value
}

func padLabel(label string) string {
	if len(label) >= labelWidth {
		return label + " "
	}
	return label + strings.Repeat(" ", labelWidth-len(label))
}

func wrap(text string, width int) string {
	if width <= 0 {
		return text
	}
	return lipgloss.NewStyle().Width(width).Render(text)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func sortFindingsBySeverity(in []Finding) []Finding {
	out := make([]Finding, len(in))
	copy(out, in)

	rank := map[string]int{
		"critical": 0,
		"high":     1,
		"moderate": 2,
		"low":      3,
		"unknown":  4,
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri := rank[styles.NormalizeSeverity(out[i].Severity)]
		rj := rank[styles.NormalizeSeverity(out[j].Severity)]
		if ri != rj {
			return ri < rj
		}
		if out[i].PackageName != out[j].PackageName {
			return out[i].PackageName < out[j].PackageName
		}
		return out[i].ID < out[j].ID
	})
	return out
}

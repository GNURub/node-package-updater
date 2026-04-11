package audit

import (
	"fmt"
	"strings"

	"github.com/GNURub/node-package-updater/internal/styles"
)

func formatText(result *Result) string {
	if result == nil {
		return styles.SuccessStyle.Render("✅ No audit result available") + "\n"
	}

	if result.Summary.TotalFindings == 0 {
		return fmt.Sprintf(
			"%s\nScanned %d lockfiles and %d directories in %s.\n",
			styles.SuccessStyle.Render("✅ No known vulnerabilities found"),
			result.Summary.LockfilesScanned,
			result.Summary.DirectoriesScanned,
			result.Summary.RootDir,
		)
	}

	var b strings.Builder
	b.WriteString(styles.ErrorStyle.Render(
		fmt.Sprintf("🚨 %d vulnerabilities found across %d packages", result.Summary.TotalFindings, result.Summary.AffectedPackages),
	))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf(
		"Scanned %d lockfiles and %d directories in %s.\n\n",
		result.Summary.LockfilesScanned,
		result.Summary.DirectoriesScanned,
		result.Summary.RootDir,
	))

	for i, finding := range result.Findings {
		if i > 0 {
			b.WriteString("\n")
		}

		header := fmt.Sprintf("%s@%s", finding.PackageName, finding.Version)
		if finding.Severity != "" {
			header = fmt.Sprintf("%s [%s]", header, strings.ToUpper(finding.Severity))
		}

		b.WriteString(header + "\n")
		b.WriteString(fmt.Sprintf("  ID: %s\n", finding.ID))
		b.WriteString(fmt.Sprintf("  Source: %s (%s)\n", finding.SourcePath, finding.SourceType))
		if finding.Summary != "" {
			b.WriteString(fmt.Sprintf("  Summary: %s\n", finding.Summary))
		}
		if finding.Details != "" {
			b.WriteString(fmt.Sprintf("  Details: %s\n", finding.Details))
		}
		if len(finding.Aliases) > 0 {
			b.WriteString(fmt.Sprintf("  Aliases: %s\n", strings.Join(finding.Aliases, ", ")))
		}
		b.WriteString(fmt.Sprintf("  Suggestion: %s\n", finding.Recommendation))
	}

	return b.String()
}

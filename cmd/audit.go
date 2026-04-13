package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/GNURub/node-package-updater/internal/audit"
	"github.com/GNURub/node-package-updater/internal/styles"
	"github.com/spf13/cobra"
)

var (
	auditDir            string
	auditFormat         string
	auditOutputFile     string
	auditOffline        bool
	auditDownloadDB     bool
	auditDBPath         string
	auditPackageManager string
	auditNoTUI          bool
)

var auditCmd = &cobra.Command{
	Use:   "audit [path]",
	Short: "Audit project dependencies for known vulnerabilities",
	Long:  "Audit project dependencies for known vulnerabilities using the OSV API.",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		baseDir := auditDir
		if len(args) > 0 {
			baseDir = args[0]
		}

		format := audit.OutputFormat(auditFormat)
		if format != audit.FormatText && format != audit.FormatJSON {
			fmt.Println(styles.ErrorStyle.Render("invalid format: must be one of text, json"))
			os.Exit(1)
		}

		service := audit.NewService()
		result, err := service.Audit(context.Background(), audit.Options{
			Path:                   baseDir,
			PackageManagerOverride: auditPackageManager,
			Offline:                auditOffline,
			DownloadDB:             auditDownloadDB,
			DBPath:                 auditDBPath,
		})
		if err != nil {
			fmt.Println(styles.ErrorStyle.Render(err.Error()))
			os.Exit(1)
		}

		output, err := audit.Format(result, format)
		if err != nil {
			fmt.Println(styles.ErrorStyle.Render(err.Error()))
			os.Exit(1)
		}

		switch {
		case auditOutputFile != "":
			if err := os.MkdirAll(filepath.Dir(auditOutputFile), 0o755); err != nil {
				fmt.Println(styles.ErrorStyle.Render(err.Error()))
				os.Exit(1)
			}
			if err := os.WriteFile(auditOutputFile, output, 0o644); err != nil {
				fmt.Println(styles.ErrorStyle.Render(err.Error()))
				os.Exit(1)
			}
			fmt.Printf("📝 Audit report written to %s\n", auditOutputFile)

		case format == audit.FormatText && !auditNoTUI && result.Summary.TotalFindings > 0 && isStdoutTTY():
			if err := audit.RunInteractive(result); err != nil {
				fmt.Println(styles.ErrorStyle.Render(err.Error()))
				os.Exit(1)
			}
			fmt.Println(styles.ErrorStyle.Bold(true).Render(
				fmt.Sprintf("🚨 %d vulnerability finding(s) across %d package(s)",
					result.Summary.TotalFindings, result.Summary.AffectedPackages),
			))

		default:
			fmt.Println(string(output))
		}

		if result.Summary.TotalFindings > 0 {
			os.Exit(1)
		}
	},
}

func isStdoutTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func init() {
	auditCmd.Flags().StringVarP(&auditDir, "dir", "d", "", "Project directory to audit")
	auditCmd.Flags().StringVar(&auditFormat, "format", "text", "Output format (text|json)")
	auditCmd.Flags().StringVar(&auditOutputFile, "output-file", "", "Write the report to a file")
	auditCmd.Flags().BoolVar(&auditOffline, "offline", false, "Use the local OSV database instead of the network")
	auditCmd.Flags().BoolVar(&auditDownloadDB, "download-db", false, "Download/update the offline OSV database")
	auditCmd.Flags().StringVar(&auditDBPath, "db-path", "", "Path to the local OSV database")
	auditCmd.Flags().StringVarP(&auditPackageManager, "packageManager", "M", "", "Package manager override for target discovery")
	auditCmd.Flags().BoolVar(&auditNoTUI, "no-tui", false, "Disable the interactive TUI and print the styled report instead")

	rootCmd.AddCommand(auditCmd)
}

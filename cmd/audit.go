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
)

var auditCmd = &cobra.Command{
	Use:   "audit [path]",
	Short: "Audit project dependencies for known vulnerabilities",
	Long:  "Audit project dependencies for known vulnerabilities using an embedded osv-scanner integration.",
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

		if auditOutputFile != "" {
			if err := os.MkdirAll(filepath.Dir(auditOutputFile), 0o755); err != nil {
				fmt.Println(styles.ErrorStyle.Render(err.Error()))
				os.Exit(1)
			}
			if err := os.WriteFile(auditOutputFile, output, 0o644); err != nil {
				fmt.Println(styles.ErrorStyle.Render(err.Error()))
				os.Exit(1)
			}
			fmt.Printf("📝 Audit report written to %s\n", auditOutputFile)
		} else {
			fmt.Println(string(output))
		}

		if result.Summary.TotalFindings > 0 {
			os.Exit(1)
		}
	},
}

func init() {
	auditCmd.Flags().StringVarP(&auditDir, "dir", "d", "", "Project directory to audit")
	auditCmd.Flags().StringVar(&auditFormat, "format", "text", "Output format (text|json)")
	auditCmd.Flags().StringVar(&auditOutputFile, "output-file", "", "Write the report to a file")
	auditCmd.Flags().BoolVar(&auditOffline, "offline", false, "Use the local OSV database instead of the network")
	auditCmd.Flags().BoolVar(&auditDownloadDB, "download-db", false, "Download/update the offline OSV database")
	auditCmd.Flags().StringVar(&auditDBPath, "db-path", "", "Path to the local OSV database")
	auditCmd.Flags().StringVarP(&auditPackageManager, "packageManager", "M", "", "Package manager override for target discovery")

	rootCmd.AddCommand(auditCmd)
}

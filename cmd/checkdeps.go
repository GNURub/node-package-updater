package cmd

import (
	"fmt"
	"os"

	"github.com/GNURub/node-package-updater/internal/packagejson"
	"github.com/GNURub/node-package-updater/internal/ui"
	"github.com/GNURub/node-package-updater/pkg/checkdeps"
	"github.com/spf13/cobra"
)

var checkdepsCmd = &cobra.Command{
	Use:     "checkdeps",
	Aliases: []string{"cd", "check"},
	Short:   "Check unused dependencies in the project",
	Long:    "Analyze your project to find dependencies that are not being used in your code",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			flags.BaseDir = args[0]
		}

		done := make(chan struct{})
		errChan := make(chan error, 1)
		var results []*checkdeps.CheckResults
		go func() {
			var err error
			results, err = checkdeps.CheckUnusedDependencies(flags)
			errChan <- err
			close(done)
		}()

		ui.RunSpinner("Analizando dependencias...", done)

		err := <-errChan
		if err != nil {
			fmt.Printf("❌ Error checking dependencies: %v\n", err)
			os.Exit(1)
		}

		// Mostrar resultados después del spinner
		if results != nil {
			// Cargar el package.json principal para el fix
			baseDir := flags.BaseDir
			if baseDir == "" {
				baseDir = "."
			}
			pkg, err := packagejson.LoadPackageJSON(baseDir)
			if err != nil {
				fmt.Printf("❌ Error loading package.json: %v\n", err)
				os.Exit(1)
			}
			checkdeps.ProcessCheckDepsResults(pkg, results, flags)
		}
	},
}

func init() {
	checkdepsCmd.Flags().StringVarP(&flags.BaseDir, "dir", "d", "", "Root directory for dependency search")
	checkdepsCmd.Flags().BoolVarP(&flags.Fix, "fix", "F", false, "Automatically remove unused dependencies")
	checkdepsCmd.Flags().BoolVarP(&flags.NoInstall, "noInstall", "n", false, "Don't reinstall dependencies after removing unused ones")
	checkdepsCmd.Flags().BoolVarP(&flags.DryRun, "dryRun", "D", false, "Show what would be removed without making changes")
	checkdepsCmd.Flags().BoolVarP(&flags.Verbose, "verbose", "V", false, "Show detailed output")

	rootCmd.AddCommand(checkdepsCmd)
}

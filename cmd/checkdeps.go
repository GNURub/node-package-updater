package cmd

import (
	"fmt"
	"os"

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

		if err := checkdeps.CheckUnusedDependencies(flags); err != nil {
			fmt.Printf("❌ Error checking dependencies: %v\n", err)
			os.Exit(1)
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

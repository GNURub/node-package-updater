package cmd

import (
	"fmt"

	"github.com/GNURub/node-package-updater/internal/packagejson"
	"github.com/GNURub/node-package-updater/internal/styles"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "unused",
	Short: "Check for unused and missing dependencies",
	Long:  `Check for unused and missing dependencies in your project.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		baseDir := flags.BaseDir

		if len(args) > 0 {
			baseDir = args[0]
		}

		_, err := packagejson.LoadPackageJSON(
			baseDir,
		)

		if err != nil {
			fmt.Println(styles.ErrorStyle.Render(err.Error()))
			return
		}
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
}

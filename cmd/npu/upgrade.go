package npu

import (
	"github.com/GNURub/node-package-updater/pkg/upgrade"
	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "A CLI application to manage dependencies",
	Run: func(cmd *cobra.Command, args []string) {
		upgrade.Upgrade()
	},
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
}

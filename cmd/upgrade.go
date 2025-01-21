package cmd

import (
	"fmt"
	"os"

	"github.com/GNURub/node-package-updater/pkg/upgrade"
	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade to the latest version of the CLI",
	Run: func(cmd *cobra.Command, args []string) {
		if err := upgrade.Upgrade(); err != nil {
			fmt.Printf("❌ Upgrade failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
}

package cmd

import (
	"fmt"

	"github.com/GNURub/node-package-updater/internal/version"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of NPU",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("NPU %s\n", version.Version)
	},
}

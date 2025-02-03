package cmd

import (
	"fmt"
	"os"

	"github.com/GNURub/node-package-updater/pkg/global"
	"github.com/spf13/cobra"
)

var globalCmd = &cobra.Command{
	Use:     "global",
	Aliases: []string{"g"},
	Short:   "Global deps",
	Run: func(cmd *cobra.Command, args []string) {
		if err := global.UpdateGlobalDependencies(flags); err != nil {
			fmt.Printf("❌ Update gobal dependencies failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {

	globalCmd.Flags().StringVarP(&flags.Registry, "registry", "r", "https://registry.npmjs.org/", "NPM registry URL")
	globalCmd.Flags().StringVarP(&flags.ConfigFile, "config", "c", "", "Path to config file (default: .npmrc)")

	globalCmd.Flags().BoolVar(&flags.Pre, "pre", false, "Update to latest versions")
	globalCmd.Flags().BoolVarP(&flags.Minor, "minor", "m", false, "Update to latest minor versions")
	globalCmd.Flags().BoolVarP(&flags.Patch, "patch", "p", false, "Updatncue to latest patch versions")
	globalCmd.Flags().BoolVarP(&flags.MaintainSemver, "semanticVersion", "s", false, "Maintain semver satisfaction")

	globalCmd.Flags().BoolVarP(&flags.NoInteractive, "nonInteractive", "x", false, "Non-interactive mode")
	globalCmd.Flags().BoolVarP(&flags.DryRun, "dryRun", "D", false, "Show what would be updated without making changes")
	globalCmd.Flags().BoolVarP(&flags.Verbose, "verbose", "V", false, "Show detailed output")
	globalCmd.Flags().StringVarP(&flags.PackageManager, "packageManager", "M", "", "Package manager to use (npm, yarn, pnpm, bun, deno)")
	globalCmd.Flags().StringVar(&flags.LogLevel, "log", "info", "Log level (debug, info, warn, error)")

	globalCmd.Flags().IntVarP(&flags.Timeout, "timeout", "t", 30, "Timeout in seconds for each package update")

	rootCmd.AddCommand(globalCmd)
}

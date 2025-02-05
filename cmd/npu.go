package cmd

import (
	"fmt"

	"github.com/GNURub/node-package-updater/internal/cache"
	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/packagejson"
	"github.com/GNURub/node-package-updater/internal/styles"
	"github.com/spf13/cobra"
)

var flags = &cli.Flags{}
var rootCmd = &cobra.Command{
	Use:   "npu",
	Short: "A CLI application to manage dependencies",
	Long:  "A CLI application to manage dependencies",
	Args:  cobra.MaximumNArgs(1),
}

func init() {
	rootCmd.Flags().BoolVarP(&flags.CleanCache, "cleanCache", "C", false, "Clean cache")

	rootCmd.Flags().StringVarP(&flags.BaseDir, "dir", "d", "", "Root directory for package search")
	rootCmd.Flags().StringVarP(&flags.Registry, "registry", "r", "https://registry.npmjs.org/", "NPM registry URL")
	rootCmd.Flags().StringVarP(&flags.ConfigFile, "config", "c", "", "Path to config file (default: .npmrc)")

	rootCmd.Flags().BoolVar(&flags.Pre, "pre", false, "Update to latest versions")
	rootCmd.Flags().BoolVarP(&flags.Minor, "minor", "m", false, "Update to latest minor versions")
	rootCmd.Flags().BoolVarP(&flags.Patch, "patch", "p", false, "Update to latest patch versions")
	rootCmd.Flags().BoolVarP(&flags.MaintainSemver, "semanticVersion", "s", false, "Maintain semver satisfaction")
	rootCmd.Flags().BoolVarP(&flags.KeepRangeOperator, "keepRange", "k", true, "Keep range operator on version")

	rootCmd.Flags().BoolVarP(&flags.Production, "production", "P", false, "Update only production dependencies")
	rootCmd.Flags().BoolVarP(&flags.PeerDependencies, "includePeer", "i", false, "Include peer dependencies")

	rootCmd.Flags().BoolVarP(&flags.WithWorkspaces, "workspaces", "w", false, "Include workspace repositories")
	rootCmd.Flags().BoolVar(&flags.SkipDeprecated, "skipDeprecated", true, "Skip deprecated packages")

	rootCmd.Flags().BoolVarP(&flags.NoInstall, "noInstall", "n", false, "Do not install packages after updating")
	rootCmd.Flags().BoolVarP(&flags.NoInteractive, "nonInteractive", "x", false, "Non-interactive mode")
	rootCmd.Flags().BoolVarP(&flags.DryRun, "dryRun", "D", false, "Show what would be updated without making changes")
	rootCmd.Flags().BoolVarP(&flags.Verbose, "verbose", "V", false, "Show detailed output")
	rootCmd.Flags().StringVarP(&flags.PackageManager, "packageManager", "M", "", "Package manager to use (npm, yarn, pnpm, bun)")
	rootCmd.Flags().StringVar(&flags.LogLevel, "log", "info", "Log level (debug, info, warn, error)")

	rootCmd.Flags().IntVarP(&flags.Timeout, "timeout", "t", 30, "Timeout in seconds for each package update")

	rootCmd.Flags().StringSliceVarP(&flags.Include, "include", "I", []string{}, "Packages to include (can be specified multiple times)")
	rootCmd.Flags().StringSliceVarP(&flags.Exclude, "exclude", "e", []string{}, "Packages to exclude (can be specified multiple times)")
}

func Exec() error {
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		options := []packagejson.Option{}

		baseDir := flags.BaseDir

		if len(args) > 0 {
			baseDir = args[0]
		}

		if flags.PackageManager != "" {
			options = append(options, packagejson.WithPackageManager(flags.PackageManager))
		}

		if flags.WithWorkspaces {
			options = append(options, packagejson.EnableWorkspaces())
		}

		cache, err := cache.NewCache()
		if err != nil {
			fmt.Println(styles.ErrorStyle.Render(err.Error()))
			return
		}
		defer cache.Close()

		if flags.CleanCache {
			cache.Clean()
		}

		options = append(options, packagejson.WithCache(cache))

		pkg, err := packagejson.LoadPackageJSON(
			baseDir,
			options...,
		)

		if err != nil {
			fmt.Println(styles.ErrorStyle.Render(err.Error()))
			return
		}

		if err := pkg.ProcessDependencies(flags); err != nil {
			fmt.Println(styles.ErrorStyle.Render(err.Error()))
			return
		}
	}

	return rootCmd.Execute()
}

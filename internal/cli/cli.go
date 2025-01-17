package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type Flags struct {
	BaseDir           string
	ConfigFile        string
	DryRun            bool
	Exclude           []string
	Include           []string
	KeepRangeOperator bool
	LogLevel          string
	MaintainSemver    bool
	Minor             bool
	NoInstall         bool
	NoInteractive     bool
	PackageManager    string
	Patch             bool
	PeerDependencies  bool
	Production        bool
	Registry          string
	ShowVersion       bool
	Timeout           int
	Verbose           bool
	WithWorkspaces    bool
}

func NewRootCommand() (*cobra.Command, *Flags) {
	flags := &Flags{}

	rootCmd := &cobra.Command{
		Use:   "npu",
		Short: "A CLI application to manage dependencies",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	helpFunc := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(c *cobra.Command, s []string) {
		helpFunc(c, s)
		os.Exit(0)
	})

	rootCmd.Flags().StringVarP(&flags.BaseDir, "directory", "d", "", "Root directory for package search")
	rootCmd.Flags().StringVarP(&flags.Registry, "registry", "r", "https://registry.npmjs.org/", "NPM registry URL")
	rootCmd.Flags().StringVarP(&flags.ConfigFile, "config", "c", "", "Path to config file (default: .npmrc)")

	rootCmd.Flags().BoolVarP(&flags.Minor, "minor", "m", false, "Update to latest minor versions")
	rootCmd.Flags().BoolVarP(&flags.Patch, "patch", "p", false, "Update to latest patch versions")
	rootCmd.Flags().BoolVarP(&flags.MaintainSemver, "semanticVersion", "s", false, "Maintain semver satisfaction")
	rootCmd.Flags().BoolVarP(&flags.KeepRangeOperator, "keepRange", "k", true, "Keep range operator on version")

	rootCmd.Flags().BoolVarP(&flags.Production, "production", "P", false, "Update only production dependencies")
	rootCmd.Flags().BoolVarP(&flags.PeerDependencies, "includePeer", "i", false, "Include peer dependencies")

	rootCmd.Flags().BoolVarP(&flags.WithWorkspaces, "workspaces", "w", false, "Include workspace repositories")

	rootCmd.Flags().BoolVarP(&flags.NoInstall, "noInstall", "n", false, "Do not install packages after updating")
	rootCmd.Flags().BoolVarP(&flags.NoInteractive, "nonInteractive", "x", false, "Non-interactive mode")
	rootCmd.Flags().BoolVarP(&flags.DryRun, "dryRun", "D", false, "Show what would be updated without making changes")
	rootCmd.Flags().BoolVarP(&flags.ShowVersion, "version", "v", false, "Show version")
	rootCmd.Flags().BoolVarP(&flags.Verbose, "verbose", "V", false, "Show detailed output")
	rootCmd.Flags().StringVarP(&flags.PackageManager, "packageManager", "M", "", "Package manager to use (npm, yarn, pnpm, bun)")
	rootCmd.Flags().StringVarP(&flags.LogLevel, "log", "l", "info", "Log level (debug, info, warn, error)")

	rootCmd.Flags().IntVarP(&flags.Timeout, "timeout", "t", 30, "Timeout in seconds for each package update")

	rootCmd.Flags().StringSliceVarP(&flags.Include, "include", "I", []string{}, "Packages to include (can be specified multiple times)")
	rootCmd.Flags().StringSliceVarP(&flags.Exclude, "exclude", "e", []string{}, "Packages to exclude (can be specified multiple times)")

	return rootCmd, flags
}

func (f *Flags) ValidateFlags() error {
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[f.LogLevel] {
		return fmt.Errorf("invalid log level: must be one of debug, info, warn, error")
	}

	validPackageManagers := map[string]bool{"npm": true, "yarn": true, "pnpm": true, "bun": true, "deno": true}
	if f.PackageManager != "" && !validPackageManagers[f.PackageManager] {
		return fmt.Errorf("invalid package manager: must be one of npm, yarn, pnpm, bun, deno")
	}

	if f.Timeout < 1 {
		return fmt.Errorf("timeout must be greater than 0")
	}

	return nil
}

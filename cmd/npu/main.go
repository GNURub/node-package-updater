package main

import (
	"fmt"

	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/packagejson"
	"github.com/GNURub/node-package-updater/internal/version"
)

func main() {
	rootCmd, flags := cli.NewRootCommand()

	if err := rootCmd.Execute(); err != nil {
		fmt.Print(err)
		return
	}

	if flags.ShowVersion {
		fmt.Printf("node-package-updater version %s\n", version.Version)
		return
	}

	options := []packagejson.Option{}

	if flags.PackageManager != "" {
		options = append(options, packagejson.WithPackageManager(flags.PackageManager))
	}

	if flags.WithWorkspaces {
		options = append(options, packagejson.EnableWorkspaces())
	}

	pkg, err := packagejson.LoadPackageJSON(
		flags.BaseDir,
		options...,
	)

	if err != nil {
		fmt.Printf("Error loading package.json: %v", err)
		return
	}

	if err := pkg.ProcessDependencies(flags); err != nil {
		fmt.Printf("Warning: Error processing pkg: %v", err)
	}
}

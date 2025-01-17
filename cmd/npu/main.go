package main

import (
	"fmt"
	"log"

	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/packagejson"
	"github.com/GNURub/node-package-updater/internal/version"
)

func main() {
	// Obtener el comando raíz y las flags
	rootCmd, flags := cli.NewRootCommand()

	// Ejecutar el comando
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Error executing command: %v", err)
	}

	if flags.ShowVersion {
		fmt.Printf("node-package-updater version %s\n", version.Version)
		return
	}

	options := []packagejson.Option{
		packagejson.WithBaseDir(flags.BaseDir),
	}

	if flags.PackageManager != "" {
		options = append(options, packagejson.WithPackageManager(flags.PackageManager))
	}

	if flags.WithWorkspaces {
		options = append(options, packagejson.WithWorkspaces())
	}

	pkg, err := packagejson.LoadPackageJSON(
		options...,
	)

	if err != nil {
		log.Fatalf("Error reading package.json: %v", err)
	}

	if err := pkg.ProcessDependencies(flags); err != nil {
		log.Printf("Warning: Error processing pkg: %v", err)
	}
}

package main

import (
	"fmt"
	"log"
	"path"

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

	pkg, err := packagejson.LoadPackageJSON(
		options...,
	)

	if err != nil {
		log.Fatalf("Error reading package.json: %v", err)
	}

	spaces := []string{
		"",
	}

	if flags.Workspaces {
		workspaces, err := pkg.GetWorkspaces()
		if err != nil {
			log.Fatalf("Error getting workspaces: %v", err)
		}

		spaces = append(spaces, workspaces...)
	}

	for _, workspace := range spaces {
		workspacePkg, err := packagejson.LoadPackageJSON(
			packagejson.WithBaseDir(path.Join(flags.BaseDir, workspace)),
			packagejson.WithPackageManager(pkg.PackageManager.Name),
		)
		if err != nil {
			log.Printf("Warning: Error reading workspace %s: %v", workspace, err)
			continue
		}

		if err := workspacePkg.ProcessDependencies(flags); err != nil {
			log.Printf("Warning: Error processing workspace %s: %v", workspace, err)
		}
	}
}

package main

import (
	"log"
	"path"

	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/packagejson"
)

func main() {
	flags := cli.ParseFlags()

	pkg, err := packagejson.LoadPackageJSON(flags.BaseDir)

	if err != nil {
		log.Fatalf("Error reading package.json: %v", err)
	}

	if err := pkg.ProcessDependencies(flags); err != nil {
		log.Fatalf("Error processing dependencies: %v", err)
	}

	if flags.Workspaces {
		workspaces, err := pkg.GetWorkspaces()
		if err != nil {
			log.Printf("Warning: Error reading workspaces: %v", err)
		}

		for _, workspace := range workspaces {
			workspacePkg, err := packagejson.LoadPackageJSON(path.Join(flags.BaseDir, workspace))
			if err != nil {
				log.Printf("Warning: Error reading workspace %s: %v", workspace, err)
				continue
			}

			if err := workspacePkg.ProcessDependencies(flags); err != nil {
				log.Printf("Warning: Error processing workspace %s: %v", workspace, err)
			}
		}
	}
}

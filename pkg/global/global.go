package global

import (
	"fmt"

	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/packagejson"
	"github.com/GNURub/node-package-updater/internal/packagemanager"
)

func UpdateGlobalDependencies(flags *cli.Flags) error {
	pm := packagemanager.Detect("", flags.PackageManager)

	globalDeps, err := pm.GetGlobalDeps()

	if err != nil {
		return fmt.Errorf("failed to get global dependencies: %w", err)
	}

	// Actualizamos las dependencias
	dependencies, _ := packagejson.UpdateDependencies(globalDeps, flags, nil)

	if len(dependencies["global"]) == 0 {
		fmt.Println("🎉! All global dependencies updated successfully!")
		return nil
	}

	for _, dep := range dependencies["global"] {
		if err := pm.Install("-g", dep.PackageName+"@"+dep.NextVersion.String()); err != nil {
			return fmt.Errorf("failed to install package %s: %w", dep.PackageName, err)
		}
	}

	fmt.Printf("📦 global dependencies updated\n")

	return nil
}

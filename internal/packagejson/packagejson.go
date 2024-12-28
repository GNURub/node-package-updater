package packagejson

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/dependency"
	"github.com/GNURub/node-package-updater/internal/packagemanager"
	"github.com/GNURub/node-package-updater/internal/ui"
)

type PackageJSON struct {
	PackageManager   packagemanager.PackageManager
	Path             string
	Dependencies     map[string]string `json:"dependencies"`
	DevDependencies  map[string]string `json:"devDependencies"`
	PeerDependencies map[string]string `json:"peerDependencies"`
	Workspaces       []string          `json:"workspaces"`
}

func LoadPackageJSON(dir string) (*PackageJSON, error) {
	packagePath := path.Join(dir, "package.json")
	data, err := os.ReadFile(packagePath)
	if err != nil {
		return nil, err
	}

	var pkg PackageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}

	pkg.PackageManager = packagemanager.Detect(dir)

	pkg.Path = packagePath
	return &pkg, nil
}

func (p *PackageJSON) GetWorkspaces() ([]string, error) {
	var workspacePaths []string
	for _, workspace := range p.Workspaces {
		matches, err := filepath.Glob(workspace)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			workspacePaths = append(workspacePaths, filepath.Join(match, "package.json"))
		}
	}
	return workspacePaths, nil
}

func (p *PackageJSON) ProcessDependencies(flags *cli.Flags) error {
	allDeps := make(map[string]dependency.Dependencies)
	for name, version := range p.Dependencies {
		d, err := dependency.NewDependency(name, version)

		if err == nil {
			allDeps["prod"] = append(allDeps["prod"], d)
		} else {
			fmt.Print(err, "\n", name, "\n", version)
		}
	}

	if !flags.Production {
		for name, version := range p.DevDependencies {
			d, err := dependency.NewDependency(name, version)

			if err == nil {
				allDeps["dev"] = append(allDeps["dev"], d)
			}
		}

		if flags.PeerDependencies {
			for name, version := range p.PeerDependencies {
				d, err := dependency.NewDependency(name, version)

				if err == nil {
					allDeps["peer"] = append(allDeps["peer"], d)
				}
			}
		}
	}

	for _, deps := range allDeps {
		deps.FetchNewVersions(flags)
	}

	toUpdate := allDeps
	if flags.Interactive {
		toUpdate, _ = ui.SelectDependencies(allDeps)
	}

	// selectedDeps := allDeps
	// if flags.Interactive {
	// 	var items []string
	// 	for name := range allDeps {
	// 		items = append(items, name)
	// 	}

	// 	prompt := promptui.Select{
	// 		Label: "Select dependencies to update (space to select, enter to confirm)",
	// 		Items: items,
	// 		Size:  20,
	// 	}

	// 	selectedDeps = make(map[string]string)
	// 	_, result, err := prompt.Run()
	// 	if err != nil {
	// 		return err
	// 	}
	// 	selectedDeps[result] = allDeps[result]
	// }

	// updatedDeps := make(map[string]string)
	// for name, version := range selectedDeps {
	// 	newVersion, err := dependency.GetNewVersion(name, version, flags)
	// 	if err != nil {
	// 		return fmt.Errorf("error updating %s: %v", name, err)
	// 	}
	// 	updatedDeps[name] = newVersion
	// }

	return p.updatePackageJSON(toUpdate)
}

func (p *PackageJSON) updatePackageJSON(updatedDeps map[string]dependency.Dependencies) error {
	for _, dep := range updatedDeps["prod"] {
		if _, ok := p.Dependencies[dep.PackageName]; ok && dep.HaveToUpdate {
			p.Dependencies[dep.PackageName] = dep.NextVersion
		}
	}

	for _, dep := range updatedDeps["dev"] {
		if _, ok := p.DevDependencies[dep.PackageName]; ok && dep.HaveToUpdate {
			p.DevDependencies[dep.PackageName] = dep.NextVersion
		}
	}

	for _, dep := range updatedDeps["peer"] {
		if _, ok := p.PeerDependencies[dep.PackageName]; ok && dep.HaveToUpdate {
			p.PeerDependencies[dep.PackageName] = dep.NextVersion
		}
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(p.Path, data, 0644)
}

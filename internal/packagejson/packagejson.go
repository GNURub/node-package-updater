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
	"github.com/GNURub/node-package-updater/internal/updater"
)

type PackageJSON struct {
	PackageManager   packagemanager.PackageManager
	Path             string
	Dependencies     map[string]string `json:"dependencies,omitempty"`
	DevDependencies  map[string]string `json:"devDependencies,omitempty"`
	PeerDependencies map[string]string `json:"peerDependencies,omitempty"`
	Workspaces       []string          `json:"workspaces,omitempty"`
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
		d, err := dependency.NewDependency(name, version, "prod")

		if err == nil {
			allDeps["prod"] = append(allDeps["prod"], d)
		} else {
			fmt.Print(err, "\n", name, "\n", version)
		}
	}

	if !flags.Production {
		for name, version := range p.DevDependencies {
			d, err := dependency.NewDependency(name, version, "dev")

			if err == nil {
				allDeps["dev"] = append(allDeps["dev"], d)
			}
		}

		if flags.PeerDependencies {
			for name, version := range p.PeerDependencies {
				d, err := dependency.NewDependency(name, version, "peer")

				if err == nil {
					allDeps["peer"] = append(allDeps["peer"], d)
				}
			}
		}
	}

	totalDeps := len(allDeps["prod"]) + len(allDeps["dev"]) + len(allDeps["peer"])
	currentPackage := make(chan string, totalDeps)
	processed := make(chan bool, totalDeps)
	packageUpdateNotifier := make(chan bool)
	done := make(chan bool)
	someNewVersion := false
	bar := ui.ShowProgressBar(
		totalDeps,
	)

	for _, envDeps := range allDeps {
		go func(deps dependency.Dependencies) {
			updater.FetchNewVersions(deps, flags, processed, currentPackage, packageUpdateNotifier)
		}(envDeps)
	}

	currentProcessed := 0
	go func() {
		for {
			select {
			case packageName := <-currentPackage:
				bar.Send(packageName)
			case <-processed:
				currentProcessed++
				bar.Send(currentProcessed)

				percentage := float64(currentProcessed) / float64(totalDeps) * 100
				bar.Send(ui.ProgressMsg(percentage))

				if currentProcessed == totalDeps {
					done <- true
				}
			case <-packageUpdateNotifier:
				someNewVersion = true
			}
		}
	}()

	<-done
	bar.ReleaseTerminal()
	bar.Kill()

	if !someNewVersion {
		fmt.Println("All dependencies are up to date")
		return nil
	}

	toUpdate := allDeps
	if !flags.NoInteractive {
		toUpdate, _ = ui.SelectDependencies(allDeps)
	} else {
		for _, envDeps := range allDeps {
			for _, dep := range envDeps {
				dep.HaveToUpdate = dep.NextVersion != ""
			}
		}
	}

	err := p.updatePackageJSON(toUpdate)

	if err != nil {
		return fmt.Errorf("error updating package.json: %v", err)
	}

	fmt.Println("All dependencies processed")

	return nil
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
		return fmt.Errorf("error serializing package.json: %v", err)
	}

	return os.WriteFile(p.Path, data, 0644)
}

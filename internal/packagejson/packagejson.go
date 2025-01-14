package packagejson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"

	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/dependency"
	"github.com/GNURub/node-package-updater/internal/packagemanager"
	"github.com/GNURub/node-package-updater/internal/ui"
	"github.com/GNURub/node-package-updater/internal/updater"
	"github.com/iancoleman/orderedmap"
)

type Option func(*PackageJSON)

type PackageJSON struct {
	packageFilePath string
	basedir         string
	PackageManager  *packagemanager.PackageManager
	JSON            struct {
		Manager          string            `json:"packageManager,omitempty"`
		Dependencies     map[string]string `json:"dependencies,omitempty"`
		DevDependencies  map[string]string `json:"devDependencies,omitempty"`
		PeerDependencies map[string]string `json:"peerDependencies,omitempty"`
		Workspaces       []string          `json:"workspaces,omitempty"`
	}
}

func WithPackageManager(manager string) Option {
	return func(s *PackageJSON) {
		s.JSON.Manager = manager
	}
}

func WithBaseDir(basedir string) Option {
	return func(s *PackageJSON) {
		s.basedir = basedir
	}
}

func LoadPackageJSON(options ...Option) (*PackageJSON, error) {
	pkg := &PackageJSON{}

	for _, o := range options {
		o(pkg)
	}

	fullPackageJSONPath := path.Join(pkg.basedir, "package.json")
	data, err := os.ReadFile(fullPackageJSONPath)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &pkg.JSON); err != nil {
		return nil, err
	}

	pkg.PackageManager = packagemanager.Detect(path.Base(fullPackageJSONPath), pkg.JSON.Manager)

	pkg.packageFilePath = fullPackageJSONPath
	return pkg, nil
}

func (p *PackageJSON) GetWorkspaces() ([]string, error) {
	var workspacePaths []string
	for _, workspace := range p.JSON.Workspaces {
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
	for name, version := range p.JSON.Dependencies {
		d, err := dependency.NewDependency(name, version, "prod")

		if err == nil {
			allDeps["prod"] = append(allDeps["prod"], d)
		} else {
			fmt.Print(err, "\n", name, "\n", version)
		}
	}

	if !flags.Production {
		for name, version := range p.JSON.DevDependencies {
			d, err := dependency.NewDependency(name, version, "dev")

			if err == nil {
				allDeps["dev"] = append(allDeps["dev"], d)
			}
		}

		if flags.PeerDependencies {
			for name, version := range p.JSON.PeerDependencies {
				d, err := dependency.NewDependency(name, version, "peer")

				if err == nil {
					allDeps["peer"] = append(allDeps["peer"], d)
				}
			}
		}
	}

	totalDeps := len(allDeps["prod"]) + len(allDeps["dev"]) + len(allDeps["peer"])

	if totalDeps == 0 {
		return fmt.Errorf("no dependencies found")
	}

	currentPackage := make(chan string, totalDeps)
	processed := make(chan bool, totalDeps)
	packageUpdateNotifier := make(chan bool)
	done := make(chan bool)
	someNewVersion := false
	bar, err := ui.ShowProgressBar(
		totalDeps,
	)

	if err != nil {
		return fmt.Errorf("error showing progress bar: %v", err)
	}

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
				bar.Send(ui.PackageName(packageName))
			case <-processed:
				currentProcessed++

				percentage := float64(currentProcessed) / float64(totalDeps)

				bar.Send(ui.ProgressMsg{
					Percentage:          percentage,
					CurrentPackageIndex: currentProcessed,
				})
				if currentProcessed == totalDeps {
					done <- true
				}
			case <-packageUpdateNotifier:
				someNewVersion = true
			}
		}
	}()

	bar.Run()

	<-done

	bar.ReleaseTerminal()
	bar.Kill()

	if !someNewVersion {
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

	// Comprobamos si hay que actualizar las dependencias
	if !updater.NeedToUpdate(toUpdate) {
		fmt.Println("No dependencies to update")
		return nil
	}

	err = p.updatePackageJSON(toUpdate)

	if err != nil {
		return fmt.Errorf("error updating package.json: %v", err)
	}

	fmt.Println("All dependencies processed")

	if !flags.NoInstall {
		if err := p.PackageManager.Install(); err != nil {
			log.Printf("Warning: Error installing workspace %s: %v", p.basedir, err)
		}
	}

	return nil
}

func (p *PackageJSON) updatePackageJSON(updatedDeps map[string]dependency.Dependencies) error {
	// Leer el archivo original
	originalData, err := os.ReadFile(p.packageFilePath)
	if err != nil {
		return fmt.Errorf("error reading package.json: %v", err)
	}

	orderedJSON := orderedmap.New()
	if err := json.Unmarshal(originalData, &orderedJSON); err != nil {
		return fmt.Errorf("error unmarshalling package.json: %v", err)
	}

	depSections := map[string]string{
		"prod": "dependencies",
		"dev":  "devDependencies",
		"peer": "peerDependencies",
	}

	for envType, section := range depSections {
		if depsValue, ok := orderedJSON.Get(section); ok {
			if depsMap, ok := depsValue.(orderedmap.OrderedMap); ok {
				// Luego actualizar las que han cambiado
				for _, dep := range updatedDeps[envType] {
					if dep.HaveToUpdate {
						depsMap.Set(dep.PackageName, dep.NextVersion)
					}
				}
			}
		}
	}

	// Convertir a JSON manteniendo el formato
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(orderedJSON); err != nil {
		return fmt.Errorf("error serializing updated package.json: %v", err)
	}

	jsonBytes := bytes.TrimRight(buf.Bytes(), "\n")

	if err := os.WriteFile(p.packageFilePath, jsonBytes, 0644); err != nil {
		return fmt.Errorf("error writing updated package.json: %v", err)
	}

	return nil
}

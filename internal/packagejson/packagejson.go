package packagejson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/GNURub/node-package-updater/internal/cache"
	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/dependency"
	"github.com/GNURub/node-package-updater/internal/packagemanager"
	"github.com/GNURub/node-package-updater/internal/ui"
	"github.com/GNURub/node-package-updater/internal/updater"
	"github.com/GNURub/node-package-updater/internal/utils"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/iancoleman/orderedmap"
)

type Option func(*PackageJSON)

type PackageJSON struct {
	packageFilePath string
	basedir         string
	PackageManager  *packagemanager.PackageManager
	packageJson     struct {
		Manager          string            `json:"packageManager,omitempty"`
		Dependencies     map[string]string `json:"dependencies,omitempty"`
		DevDependencies  map[string]string `json:"devDependencies,omitempty"`
		PeerDependencies map[string]string `json:"peerDependencies,omitempty"`
		Workspaces       []string          `json:"workspaces,omitempty"`
	}
}

func WithPackageManager(manager string) Option {
	return func(s *PackageJSON) {
		s.packageJson.Manager = manager
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

	pkg.packageFilePath = fullPackageJSONPath

	if err := json.Unmarshal(data, &pkg.packageJson); err != nil {
		return nil, err
	}

	pkg.PackageManager = packagemanager.Detect(path.Dir(pkg.packageFilePath), pkg.packageJson.Manager)
	return pkg, nil
}

func (p *PackageJSON) GetWorkspaces() ([]string, error) {
	var workspacePaths []string
	for _, workspace := range p.packageJson.Workspaces {
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
	cache, err := cache.NewCache()
	if err != nil {
		return fmt.Errorf("error creating cache: %v", err)
	}
	defer cache.Close()

	var allDeps dependency.Dependencies

	for name, version := range p.packageJson.Dependencies {
		d, err := dependency.NewDependency(name, version, "prod")
		if err != nil {
			fmt.Printf("Error creating dependency %s: %v\n", name, err)
			continue
		}
		allDeps = append(allDeps, d)
	}

	if !flags.Production {
		for name, version := range p.packageJson.DevDependencies {
			d, err := dependency.NewDependency(name, version, "dev")
			if err != nil {
				fmt.Printf("Error creating dependency %s: %v\n", name, err)
				continue
			}
			allDeps = append(allDeps, d)
		}

		if flags.PeerDependencies {
			for name, version := range p.packageJson.PeerDependencies {
				d, err := dependency.NewDependency(name, version, "peer")
				if err != nil {
					fmt.Printf("Error creating dependency %s: %v\n", name, err)
					continue
				}
				allDeps = append(allDeps, d)
			}
		}
	}

	totalDeps := len(allDeps)
	if totalDeps == 0 {
		return fmt.Errorf("no dependencies found")
	}

	currentPackage := make(chan string, totalDeps)
	processed := make(chan bool, totalDeps)

	// Solo mostrar la barra de progreso si NoInteractive es falso
	var bar *tea.Program
	if !flags.NoInteractive {
		bar, err = ui.ShowProgressBar(totalDeps)
		if err != nil {
			return fmt.Errorf("error showing progress bar: %v", err)
		}
	}

	var wg sync.WaitGroup
	wg.Add(totalDeps)

	go func() {
		updater.FetchNewVersions(allDeps, flags, processed, currentPackage, cache)
	}()

	go func() {
		currentProcessed := 0
		for {
			select {
			case packageName := <-currentPackage:
				if bar != nil {
					bar.Send(ui.PackageName(packageName))
				}
			case <-processed:
				wg.Done()
				currentProcessed++
				if bar != nil {
					percentage := float64(currentProcessed) / float64(totalDeps)
					bar.Send(ui.ProgressMsg{Percentage: percentage, CurrentPackageIndex: currentProcessed})
				}
			}
		}
	}()

	if bar != nil {
		bar.Run()
	}

	wg.Wait()

	if bar != nil {
		bar.ReleaseTerminal()
		bar.Kill()
	}

	depsWithNewVersion := allDeps.FilterWithNewVersion()
	if len(depsWithNewVersion) == 0 {
		return nil
	}

	// Solo permitir la selección interactiva si NoInteractive es falso
	if !flags.NoInteractive {
		depsWithNewVersion, _ = ui.SelectDependencies(depsWithNewVersion)
	} else {
		for _, dep := range depsWithNewVersion {
			dep.HaveToUpdate = true
		}
	}

	depsToUpdate := depsWithNewVersion.FilterForUpdate()
	if len(depsToUpdate) == 0 {
		fmt.Println("No dependencies to update")
		return nil
	}

	err = p.updatePackageJSON(flags, depsToUpdate)
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

func (p *PackageJSON) updatePackageJSON(flags *cli.Flags, updatedDeps dependency.Dependencies) error {
	// Leer el archivo original
	originalData, err := os.ReadFile(p.packageFilePath)
	if err != nil {
		return fmt.Errorf("error reading package.json: %v", err)
	}

	orderedJSON := orderedmap.New()
	orderedJSON.SetEscapeHTML(false)

	if err := json.Unmarshal(originalData, &orderedJSON); err != nil {
		return fmt.Errorf("error unmarshalling package.json: %v", err)
	}

	depSections := map[string]string{
		"prod": "dependencies",
		"dev":  "devDependencies",
		"peer": "peerDependencies",
	}

	for _, dep := range updatedDeps {
		if depsValue, ok := orderedJSON.Get(depSections[dep.Env]); ok {
			if depsMap, ok := depsValue.(orderedmap.OrderedMap); ok {
				currentVersion := dep.CurrentVersionStr
				updatedVersion := dep.NextVersion

				if flags.KeepRangeOperator {
					updatedVersion = fmt.Sprintf("%s%s", utils.GetPrefix(currentVersion), updatedVersion)
				}

				depsMap.Set(dep.PackageName, updatedVersion)
			}
		}
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)

	// Configurar el encoder para que no escape caracteres HTML
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)

	if err := enc.Encode(orderedJSON); err != nil {
		return fmt.Errorf("error serializing updated package.json: %v", err)
	}

	jsonBytes := bytes.TrimRight(buf.Bytes(), "\n")

	// Escribir el archivo actualizado
	if err := os.WriteFile(p.packageFilePath, jsonBytes, 0644); err != nil {
		return fmt.Errorf("error writing updated package.json: %v", err)
	}

	return nil
}

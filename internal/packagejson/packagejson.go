package packagejson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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
	tea "github.com/charmbracelet/bubbletea"
	"github.com/iancoleman/orderedmap"
)

type Option func(*PackageJSON) error

type PackageJSON struct {
	packageFilePath   string
	Dir               string
	PackageManager    *packagemanager.PackageManager
	workspacesPkgs    map[string]*PackageJSON
	processWorkspaces bool
	cache             *cache.Cache
	packageJson       struct {
		Manager          string            `json:"packageManager,omitempty"`
		Dependencies     map[string]string `json:"dependencies,omitempty"`
		DevDependencies  map[string]string `json:"devDependencies,omitempty"`
		PeerDependencies map[string]string `json:"peerDependencies,omitempty"`
		Workspaces       []string          `json:"workspaces,omitempty"`
	}
}

func WithPackageManager(manager string) Option {
	return func(s *PackageJSON) error {
		s.packageJson.Manager = manager

		return nil
	}
}

func WithCache(cache *cache.Cache) Option {
	return func(s *PackageJSON) error {
		s.cache = cache

		return nil
	}
}

func WithBaseDir(basedir string) Option {
	return func(s *PackageJSON) error {
		s.Dir = basedir

		return nil
	}
}

func EnableWorkspaces() Option {
	return func(p *PackageJSON) error {
		p.processWorkspaces = true
		return nil
	}
}

func LoadPackageJSON(dir string, opts ...Option) (*PackageJSON, error) {
	pkg := &PackageJSON{
		Dir: dir,
	}

	fullPackageJSONPath := path.Join(pkg.Dir, "package.json")
	data, err := os.ReadFile(fullPackageJSONPath)
	if err != nil {
		return nil, fmt.Errorf("no package.json found on directory: %s", filepath.Dir(fullPackageJSONPath))
	}

	pkg.packageFilePath = fullPackageJSONPath

	if err := json.Unmarshal(data, &pkg.packageJson); err != nil {
		return nil, err
	}

	pkg.workspacesPkgs = make(map[string]*PackageJSON)
	pkg.PackageManager = packagemanager.Detect(pkg.Dir, pkg.packageJson.Manager)

	for _, opt := range opts {
		if err := opt(pkg); err != nil {
			return nil, fmt.Errorf("error applying option: %w", err)
		}
	}

	if pkg.processWorkspaces {
		workspacesPaths := pkg.PackageManager.GetWorkspacesPaths(pkg.Dir, pkg.packageJson.Workspaces)
		for _, workspacePath := range workspacesPaths {
			if _, ok := pkg.workspacesPkgs[workspacePath]; ok {
				continue
			}

			workspacePkg, err := LoadPackageJSON(
				workspacePath,
				WithPackageManager(pkg.packageJson.Manager),
				WithCache(pkg.cache),
			)

			if err != nil {
				continue
			}

			pkg.workspacesPkgs[workspacePath] = workspacePkg
		}
	}

	pkg.workspacesPkgs[pkg.Dir] = pkg

	return pkg, nil
}

func (p *PackageJSON) ProcessDependencies(flags *cli.Flags) error {
	var allDeps dependency.Dependencies

	for workspace, pkg := range p.workspacesPkgs {
		for name, version := range pkg.packageJson.Dependencies {
			d, err := dependency.NewDependency(name, version, "prod", workspace)
			if err != nil {
				continue
			}
			allDeps = append(allDeps, d)
		}

		if !flags.Production {
			for name, version := range p.packageJson.DevDependencies {
				d, err := dependency.NewDependency(name, version, "dev", workspace)
				if err != nil {
					continue
				}
				allDeps = append(allDeps, d)
			}

			if flags.PeerDependencies {
				for name, version := range p.packageJson.PeerDependencies {
					d, err := dependency.NewDependency(name, version, "peer", workspace)
					if err != nil {
						continue
					}
					allDeps = append(allDeps, d)
				}
			}
		}
	}

	depsByWorkspace, err := UpdateDependencies(allDeps, flags, p.cache)
	if err != nil {
		return err
	}

	for workspace, pkg := range p.workspacesPkgs {
		if deps, ok := depsByWorkspace[workspace]; ok {
			err := pkg.updatePackageJSON(flags, deps)
			if err != nil {
				return fmt.Errorf("error updating package.json: %v", err)
			}
		}
	}

	fmt.Println("🎉! All dependencies updated successfully!")

	return nil
}

func UpdateDependencies(allDeps dependency.Dependencies, flags *cli.Flags, cache *cache.Cache) (map[string]dependency.Dependencies, error) {
	totalDeps := len(allDeps)
	if totalDeps == 0 {
		return nil, errors.New("no dependencies to update")
	}

	currentPackageName := make(chan string, totalDeps)
	dependencyProcessed := make(chan bool, totalDeps)

	var bar *tea.Program
	if !flags.NoInteractive {
		bar, _ = ui.ShowProgressBar(totalDeps)
	}

	var wg sync.WaitGroup
	wg.Add(totalDeps)

	go func() {
		updater.FetchNewVersions(allDeps, flags, dependencyProcessed, currentPackageName, cache)
	}()

	go func() {
		currentProcessed := 0
		for {
			select {
			case packageName := <-currentPackageName:
				if bar != nil {
					bar.Send(ui.PackageName(packageName))
				}
			case <-dependencyProcessed:
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
		return nil, errors.New("no dependencies to update")
	}

	if !flags.NoInteractive {
		depsWithNewVersion, _ = ui.SelectDependencies(depsWithNewVersion)
	} else {
		for _, dep := range depsWithNewVersion {
			dep.HaveToUpdate = true
		}
	}

	depsToUpdate := depsWithNewVersion.FilterForUpdate()
	if len(depsToUpdate) == 0 {
		return nil, errors.New("no dependencies to update")
	}

	var depsByWorkspace = make(map[string]dependency.Dependencies)
	for _, dep := range depsToUpdate {
		if _, ok := depsByWorkspace[dep.Workspace]; !ok {
			depsByWorkspace[dep.Workspace] = make(dependency.Dependencies, 0)
			depsByWorkspace[dep.Workspace] = append(depsByWorkspace[dep.Workspace], dep)
		}
		depsByWorkspace[dep.Workspace] = append(depsByWorkspace[dep.Workspace], dep)
	}
	return depsByWorkspace, nil
}

func (p *PackageJSON) updatePackageJSON(flags *cli.Flags, updatedDeps dependency.Dependencies) error {
	var orderedJSON orderedmap.OrderedMap
	orderedJSON.SetEscapeHTML(false)
	originalData, err := os.ReadFile(p.packageFilePath)
	if err != nil {
		return fmt.Errorf("[ERROR] Unable to read package.json: %w", err)
	}

	if err := json.Unmarshal(originalData, &orderedJSON); err != nil {
		return fmt.Errorf("[ERROR] Failed to parse package.json: %w", err)
	}

	depSections := map[string]string{
		"prod": "dependencies",
		"dev":  "devDependencies",
		"peer": "peerDependencies",
	}

	for _, dep := range updatedDeps {
		section, ok := depSections[dep.Env]
		if !ok {
			continue
		}

		depsMap, ok := orderedJSON.Get(section)
		if !ok {
			continue
		}

		if depsMap, ok := depsMap.(orderedmap.OrderedMap); ok {
			updatedVersion := dep.NextVersion.String()
			depsMap.Set(dep.PackageName, updatedVersion)
		}
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(orderedJSON); err != nil {
		return fmt.Errorf("[ERROR] Failed to serialize updated package.json: %w", err)
	}

	jsonBytes := bytes.TrimRight(buf.Bytes(), "\n")

	if flags.DryRun {
		fmt.Println(string(jsonBytes))
		return nil
	}

	if err := os.WriteFile(p.packageFilePath, jsonBytes, 0644); err != nil {
		return fmt.Errorf("[ERROR] Failed to write updated package.json: %w", err)
	}

	if !flags.NoInstall {
		return p.PackageManager.Install()
	}

	return nil
}

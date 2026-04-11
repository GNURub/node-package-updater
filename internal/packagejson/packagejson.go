package packagejson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/GNURub/node-package-updater/internal/cache"
	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/constants"
	"github.com/GNURub/node-package-updater/internal/dependency"
	"github.com/GNURub/node-package-updater/internal/gitignore"
	"github.com/GNURub/node-package-updater/internal/packagemanager"
	"github.com/GNURub/node-package-updater/internal/ui"
	"github.com/GNURub/node-package-updater/internal/updater"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/iancoleman/orderedmap"
)

// Agregamos un patrón regex para parsear el campo packageManager
var packageManagerRegex = regexp.MustCompile(`^([~^]?)([a-zA-Z0-9-]+)@(.+)$`)

type Option func(*PackageJSON) error

type Workspaces []string

func (w *Workspaces) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		*w = nil
		return nil
	}

	var patterns []string
	if err := json.Unmarshal(data, &patterns); err == nil {
		*w = Workspaces(patterns)
		return nil
	}

	var workspaceObject struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(data, &workspaceObject); err != nil {
		return err
	}

	*w = Workspaces(workspaceObject.Packages)
	return nil
}

func (w Workspaces) Patterns() []string {
	return []string(w)
}

type PackageJSON struct {
	packageFilePath   string
	Dir               string
	PackageManager    *packagemanager.PackageManager
	WorkspacesPkgs    map[string]*PackageJSON
	processWorkspaces bool
	depth             uint8
	cache             *cache.Cache
	PackageJson       struct {
		Manager              string            `json:"packageManager,omitempty"`
		Dependencies         map[string]string `json:"dependencies,omitempty"`
		DevDependencies      map[string]string `json:"devDependencies,omitempty"`
		PeerDependencies     map[string]string `json:"peerDependencies,omitempty"`
		OptionalDependencies map[string]string `json:"optionalDependencies,omitempty"`
		Workspaces           Workspaces        `json:"workspaces,omitempty"`
	}
}

func WithPackageManager(packagemanager *packagemanager.PackageManager) Option {
	return func(s *PackageJSON) error {
		s.PackageManager = packagemanager

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
		p.depth = 0
		return nil
	}
}

func WithDepth(depth uint8) Option {
	return func(p *PackageJSON) error {
		p.depth = depth
		if depth > 0 {
			p.processWorkspaces = false
		}
		return nil
	}
}

func LoadPackageJSON(dir string, opts ...Option) (*PackageJSON, error) {
	if dir == "" {
		dir = "."
	}

	if !strings.HasSuffix(dir, string(os.PathSeparator)) {
		dir += string(os.PathSeparator)
	}

	pkg := &PackageJSON{
		Dir: dir,
	}

	fullPackageJSONPath := path.Join(pkg.Dir, "package.json")
	data, err := os.ReadFile(fullPackageJSONPath)
	if err == nil {
		pkg.packageFilePath = fullPackageJSONPath
		if err := json.Unmarshal(data, &pkg.PackageJson); err != nil {
			return nil, err
		}
	}

	pkg.WorkspacesPkgs = make(map[string]*PackageJSON)

	if pkg.PackageManager == nil {
		pkg.PackageManager = packagemanager.Detect(pkg.Dir, pkg.PackageJson.Manager)
	}

	for _, opt := range opts {
		if err := opt(pkg); err != nil {
			return nil, fmt.Errorf("error applying option: %w", err)
		}
	}

	if pkg.processWorkspaces {
		workspacesPaths := pkg.PackageManager.GetWorkspacesPaths(pkg.Dir, pkg.PackageJson.Workspaces.Patterns())
		for _, workspacePath := range workspacesPaths {
			if _, ok := pkg.WorkspacesPkgs[workspacePath]; ok {
				continue
			}

			options := []Option{
				WithPackageManager(pkg.PackageManager),
				WithCache(pkg.cache),
			}

			if pkg.processWorkspaces {
				options = append(options, EnableWorkspaces())
			}

			workspacePkg, err := LoadPackageJSON(
				workspacePath,
				options...,
			)

			if err != nil {
				continue
			}

			pkg.WorkspacesPkgs[workspacePath] = workspacePkg
		}
	} else if pkg.depth > 0 {
		// Cargar .gitignore si existe
		gitignoreMatcher, err := gitignore.NewMatcher(pkg.Dir)
		if err != nil {
			// Continuar sin gitignore si hay un error al cargarlo
			fmt.Printf("Warning: Error loading .gitignore: %v\n", err)
		}

		filepath.WalkDir(pkg.Dir, func(path string, file fs.DirEntry, err error) error {
			if err != nil && !errors.Is(err, filepath.SkipDir) {
				return err
			}

			if !file.IsDir() || pkg.Dir == path {
				return nil
			}

			// comprobamos si ya existe el workspace
			if _, ok := pkg.WorkspacesPkgs[path]; ok {
				return nil
			}

			// Comprobamos si el directorio tiene un package.json
			_, err = os.Stat(filepath.Join(path, "package.json"))
			if err != nil {
				return nil
			}

			// Ignorar archivos/directorios según gitignore
			if gitignoreMatcher != nil && gitignoreMatcher.ShouldIgnore(path) {
				return nil
			}

			// check depth of the directory
			depth := len(filepath.SplitList(path))
			if depth > int(pkg.depth) {
				return filepath.SkipDir
			}

			workspacePkg, err := LoadPackageJSON(
				path,
				WithPackageManager(pkg.PackageManager),
				WithCache(pkg.cache),
			)

			if err != nil {
				return nil
			}

			pkg.WorkspacesPkgs[workspacePkg.Dir] = workspacePkg

			return nil
		})
	}

	pkg.WorkspacesPkgs[pkg.Dir] = pkg

	return pkg, nil
}

func ResolveProjectRoot(startDir, packageManagerOverride string) (string, *packagemanager.PackageManager, error) {
	if startDir == "" {
		startDir = "."
	}

	absPath, err := filepath.Abs(startDir)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve project path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err == nil && !info.IsDir() {
		absPath = filepath.Dir(absPath)
	}

	var nearestPackageDir string
	var workspaceRoot string

	for current := absPath; ; current = filepath.Dir(current) {
		manifestPath := filepath.Join(current, "package.json")
		data, err := os.ReadFile(manifestPath)
		if err == nil {
			var manifest struct {
				Manager    string     `json:"packageManager,omitempty"`
				Workspaces Workspaces `json:"workspaces,omitempty"`
			}

			if err := json.Unmarshal(data, &manifest); err == nil {
				if nearestPackageDir == "" {
					nearestPackageDir = current
				}

				pm := packagemanager.Detect(current, firstNonEmpty(packageManagerOverride, manifest.Manager))
				for _, workspacePath := range pm.GetWorkspacesPaths(current, manifest.Workspaces.Patterns()) {
					if isPathWithin(absPath, workspacePath) {
						workspaceRoot = current
					}
				}
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}

	rootDir := workspaceRoot
	if rootDir == "" {
		rootDir = nearestPackageDir
	}
	if rootDir == "" {
		return "", nil, errors.New("no package.json found in this project")
	}

	rootPkg, err := LoadPackageJSON(rootDir)
	if err != nil {
		return "", nil, err
	}

	rootPkg.PackageManager = packagemanager.Detect(rootDir, firstNonEmpty(packageManagerOverride, rootPkg.PackageJson.Manager))
	return rootDir, rootPkg.PackageManager, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}

	return ""
}

func isPathWithin(targetPath, basePath string) bool {
	rel, err := filepath.Rel(basePath, targetPath)
	if err != nil {
		return false
	}

	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func (p *PackageJSON) ProcessDependencies(flags *cli.Flags) error {
	var allDeps dependency.Dependencies

	for workspace, pkg := range p.WorkspacesPkgs {
		// Añadimos el packageManager como dependencia especial si existe
		if pkg.PackageJson.Manager != "" {
			matches := packageManagerRegex.FindStringSubmatch(pkg.PackageJson.Manager)
			if len(matches) == 4 {
				prefix := matches[1]
				name := matches[2]
				version := matches[3]

				// Formateamos la versión de acuerdo al prefijo para que se procese correctamente
				formattedVersion := version
				if prefix != "" {
					formattedVersion = prefix + version
				}

				d, err := dependency.NewDependency(name, formattedVersion, constants.PackageManager, workspace)
				if err == nil {
					// Guardamos el prefijo original como metadato para restaurarlo después
					d.PackageNamePrefix = prefix
					allDeps = append(allDeps, d)
				}
			}
		}

		for name, version := range pkg.PackageJson.Dependencies {
			d, err := dependency.NewDependency(name, version, constants.Dependencies, workspace)
			if err != nil {
				continue
			}
			allDeps = append(allDeps, d)
		}

		if !flags.Production {
			for name, version := range pkg.PackageJson.DevDependencies {
				d, err := dependency.NewDependency(name, version, constants.DevDependencies, workspace)
				if err != nil {
					continue
				}
				allDeps = append(allDeps, d)
			}

			if flags.PeerDependencies {
				for name, version := range pkg.PackageJson.PeerDependencies {
					d, err := dependency.NewDependency(name, version, constants.PeerDependencies, workspace)
					if err != nil {
						continue
					}
					allDeps = append(allDeps, d)
				}
			}
		}
	}

	if flags.Filter != "" {
		allDeps = allDeps.FilterByRegex(flags.Filter)
	}

	// sort dependencies
	sort.Sort(allDeps)

	depsByWorkspace, err := UpdateDependencies(allDeps, flags, p.cache)
	if err != nil {
		return err
	}

	// Release the exclusive bitcask lock before running the package manager install.
	// The install phase doesn't need the cache, and holding the lock prevents concurrent
	// npu invocations in other terminals from even starting.
	if p.cache != nil {
		_ = p.cache.Close()
	}

	allError := true
	for workspace, pkg := range p.WorkspacesPkgs {
		if deps, ok := depsByWorkspace[workspace]; ok {
			err := pkg.UpdatePackageJSON(flags, deps)
			if err == nil && allError {
				allError = false
			}
		}
	}

	if allError {
		return errors.New("failed to update package.json")
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

	// Lanzar la actualización en un goroutine
	go func() {
		updater.FetchNewVersions(allDeps, flags, dependencyProcessed, currentPackageName, cache)
	}()

	currentProcessed := 0
	progressDone := make(chan struct{})
	// Progreso y sincronización solo por canales
	go func() {
		for currentProcessed < totalDeps {
			select {
			case packageName, ok := <-currentPackageName:
				if !ok {
					currentPackageName = nil
					continue
				}
				if bar != nil {
					bar.Send(ui.PackageName(packageName))
				}
			case _, ok := <-dependencyProcessed:
				if !ok {
					dependencyProcessed = nil
					continue
				}
				currentProcessed++
				if bar != nil {
					percentage := float64(currentProcessed) / float64(totalDeps)
					bar.Send(ui.ProgressMsg{Percentage: percentage, CurrentPackageIndex: currentProcessed})
				}
			}
			if currentPackageName == nil && dependencyProcessed == nil {
				break
			}
		}
		close(progressDone)
	}()

	if bar != nil {
		bar.Run()
	}

	<-progressDone

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

	return groupDependenciesByWorkspace(depsToUpdate), nil
}

func groupDependenciesByWorkspace(deps dependency.Dependencies) map[string]dependency.Dependencies {
	depsByWorkspace := make(map[string]dependency.Dependencies)
	for _, dep := range deps {
		depsByWorkspace[dep.Workspace] = append(depsByWorkspace[dep.Workspace], dep)
	}

	return depsByWorkspace
}

func (p *PackageJSON) UpdatePackageJSON(flags *cli.Flags, updatedDeps dependency.Dependencies) error {
	var orderedJSON orderedmap.OrderedMap
	orderedJSON.SetEscapeHTML(false)
	originalData, err := os.ReadFile(p.packageFilePath)
	if err != nil {
		return fmt.Errorf("[ERROR] Unable to read package.json: %w", err)
	}

	if err := json.Unmarshal(originalData, &orderedJSON); err != nil {
		return fmt.Errorf("[ERROR] Failed to parse package.json: %w", err)
	}

	for _, dep := range updatedDeps {
		if dep.Env == constants.PackageManager && p.PackageJson.Manager != "" {
			// Manejo especial para el campo packageManager
			prefix := ""
			if dep.PackageNamePrefix != "" {
				prefix = dep.PackageNamePrefix
			}

			// Formateamos correctamente el valor del packageManager
			packageManagerValue := fmt.Sprintf("%s%s@%s", prefix, dep.PackageName, strings.TrimPrefix(dep.NextVersion.String(), dep.NextVersion.Prefix()))
			orderedJSON.Set(constants.PackageManager.String(), packageManagerValue)
			continue
		}

		depsMap, ok := orderedJSON.Get(dep.Env.String())
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
		return p.PackageManager.InstallInDir(p.Dir)
	}

	return nil
}

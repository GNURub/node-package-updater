package checkdeps

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/constants"
	"github.com/GNURub/node-package-updater/internal/dependency"
	"github.com/GNURub/node-package-updater/internal/gitignore"
	"github.com/GNURub/node-package-updater/internal/packagejson"
	"github.com/GNURub/node-package-updater/internal/styles"
)

// File extensions to analyze
var validExtensions = map[string]bool{
	".js":     true,
	".jsx":    true,
	".ts":     true,
	".tsx":    true,
	".vue":    true,
	".svelte": true,
	".mjs":    true,
	".cjs":    true,
	".json":   true,
}

// JavaScript/TypeScript file extensions (excluding JSON)
var validJSExtensions = map[string]bool{
	".js":     true,
	".jsx":    true,
	".ts":     true,
	".tsx":    true,
	".vue":    true,
	".svelte": true,
	".mjs":    true,
	".cjs":    true,
}

// Patterns to detect imports
var (
	// ES6 imports: import xxx from 'package'
	importRegex = regexp.MustCompile(`import\s+(?:(?:\{[^}]+\}|\*\s+as\s+[^,]+|[^,{}\s*]+)(?:\s*,\s*(?:\{[^}]+\}|\*\s+as\s+[^,]+|[^,{}\s*]+))*\s+from\s+)?['"]([^'"]+)['"]`)

	// Dynamic imports: import('package')
	dynamicImportRegex = regexp.MustCompile(`import\s*\(\s*['"]([^'"]+)['"]\s*\)`)

	// Requires: require('package')
	requireRegex = regexp.MustCompile(`require\s*\(\s*['"]([^'"]+)['"]\s*\)`)
)

// CheckResults stores the analysis results
type CheckResults struct {
	WorkspacePath         string
	UnusedDependencies    []string
	UnusedDevDependencies []string
	UsedDependencies      map[string][]string
}

// CheckUnusedDependencies looks for unused dependencies in an npm project
// Ahora retorna los resultados y no imprime nada directamente
func CheckUnusedDependencies(flags *cli.Flags) ([]*CheckResults, error) {
	baseDir := flags.BaseDir
	if baseDir == "" {
		baseDir = "."
	}

	options := []packagejson.Option{}
	if flags.WithWorkspaces {
		options = append(options, packagejson.EnableWorkspaces())
	}
	if flags.Depth > 0 {
		options = append(options, packagejson.WithDepth(flags.Depth))
	}

	pkg, err := packagejson.LoadPackageJSON(baseDir, options...)
	if err != nil {
		return nil, fmt.Errorf("error loading package.json: %w", err)
	}

	var allResults []*CheckResults
	workspacesMap := make(map[string]*packagejson.PackageJSON)
	workspacesMap[pkg.Dir] = pkg
	for path, workspacePkg := range pkg.WorkspacesPkgs {
		workspacesMap[path] = workspacePkg
	}

	for workspacePath, workspacePkg := range workspacesMap {
		// Find all files to scan in this workspace
		files, err := findFilesToScan(workspacePath)
		if err != nil {
			if flags.Verbose {
				// Guardar advertencia en los resultados, no imprimir
			}
			continue
		}

		// Analyze files and find used dependencies
		usedPackages, err := findUsedDependencies(files, false) // nunca verbose aquí
		if err != nil {
			continue
		}

		// Get dependencies from package.json structure
		deps := make(map[string]string)
		devDeps := make(map[string]string)
		peerDeps := make(map[string]string)
		optDeps := make(map[string]string)

		// Determine dependencies based on what's available in the package.json
		if len(workspacePkg.PackageJson.Dependencies) > 0 {
			deps = workspacePkg.PackageJson.Dependencies
		}

		if len(workspacePkg.PackageJson.DevDependencies) > 0 {
			devDeps = workspacePkg.PackageJson.DevDependencies
		}

		if len(workspacePkg.PackageJson.PeerDependencies) > 0 {
			peerDeps = workspacePkg.PackageJson.PeerDependencies
		}

		if len(workspacePkg.PackageJson.OptionalDependencies) > 0 {
			optDeps = workspacePkg.PackageJson.OptionalDependencies
		}

		// Determine which dependencies are not being used
		results := analyzePackageDependencies(deps, devDeps, peerDeps, optDeps, usedPackages)
		results.WorkspacePath = workspacePath

		allResults = append(allResults, results)
	}

	return allResults, nil
}

// Nueva función para aplicar fix y mostrar resultados
func ProcessCheckDepsResults(pkg *packagejson.PackageJSON, results []*CheckResults, flags *cli.Flags) error {
	for _, r := range results {
		printWorkspaceResults(r, r.WorkspacePath, flags.Verbose)
	}
	if flags.Fix {
		return fixUnusedDependencies(pkg, results, flags)
	}
	return nil
}

// FindFilesToScan looks for files to analyze in the directory
// It uses gitignore patterns to skip files that should be ignored
func FindFilesToScan(rootDir string) ([]string, error) {
	var files []string

	// Create a gitignore matcher
	matcher, err := gitignore.NewMatcher(rootDir)
	if err != nil {
		return nil, fmt.Errorf("error creating gitignore matcher: %w", err)
	}

	err = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip node_modules folder (as an extra precaution since gitignore should handle this)
		if matcher.ShouldIgnore(path) || d.IsDir() && (strings.Contains(d.Name(), "node_modules") || strings.HasPrefix(d.Name(), ".")) {
			return nil
		}

		// Check if it's a file to analyze
		if !d.IsDir() {
			ext := filepath.Ext(path)
			if validExtensions[ext] {
				files = append(files, path)
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error scanning directories: %w", err)
	}

	return files, nil
}

// findFilesToScan is an alias for FindFilesToScan, kept for backwards compatibility
func findFilesToScan(rootDir string) ([]string, error) {
	return FindFilesToScan(rootDir)
}

// findUsedDependencies looks for dependencies used in files
func findUsedDependencies(files []string, verbose bool) (map[string][]string, error) {
	usedPackages := make(map[string][]string)

	// Mejorar rendimiento: procesar archivos en paralelo con un pool de workers
	type result struct {
		dep  string
		file string
		err  error
		isJS bool
	}

	workerCount := 8 // Número de workers, ajustable según CPU
	jobs := make(chan string, len(files))
	results := make(chan result, len(files)*3) // Puede haber varios deps por archivo
	var wg sync.WaitGroup

	// Worker para procesar archivos
	worker := func() {
		defer wg.Done()
		for file := range jobs {
			ext := filepath.Ext(file)
			if ext == ".json" {
				content, err := os.ReadFile(file)
				if err != nil {
					results <- result{"", file, err, false}
					continue
				}
				jsonImports, err := findDependenciesInJSON(content, verbose)
				if err != nil && verbose {
					fmt.Printf("⚠️ Error analyzing JSON in %s: %v\n", file, err)
					continue
				}
				for _, importName := range jsonImports {
					packageName := normalizePackageName(importName)
					if packageName != "" {
						results <- result{packageName, file, nil, false}
					}
				}
			} else if validJSExtensions[ext] {
				deps, err := ExtractDependencies(file, verbose)
				if err != nil && verbose {
					fmt.Printf("⚠️ Error analyzing JS/TS file %s: %v\n", file, err)
					continue
				}
				for _, dep := range deps {
					results <- result{dep, file, nil, true}
				}
			}
		}
	}

	// Lanzar workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go worker()
	}

	// Enviar trabajos
	for _, file := range files {
		jobs <- file
	}
	close(jobs)

	// Esperar a que terminen los workers en un goroutine
	go func() {
		wg.Wait()
		close(results)
	}()

	// Recoger resultados
	for res := range results {
		if res.err != nil {
			continue
		}
		relPath, err := filepath.Rel(".", res.file)
		if err != nil {
			relPath = res.file
		}
		usedPackages[res.dep] = append(usedPackages[res.dep], relPath)
		if verbose {
			if res.isJS {
				fmt.Printf("📦 Dependency '%s' found in %s\n", res.dep, relPath)
			} else {
				fmt.Printf("📦 Dependency '%s' found in JSON file %s\n", res.dep, relPath)
			}
		}
	}

	return usedPackages, nil
}

// findDependenciesInJSON looks for references to dependencies in JSON files
func findDependenciesInJSON(content []byte, verbose bool) ([]string, error) {
	var jsonData interface{}
	var foundDependencies []string

	// Try to parse the JSON
	if err := json.Unmarshal(content, &jsonData); err != nil {
		return nil, err
	}

	// Process JSON data recursively
	processDependenciesInJSON(jsonData, &foundDependencies, verbose)

	return foundDependencies, nil
}

// processDependenciesInJSON recursively analyzes a JSON object looking for package references
func processDependenciesInJSON(data interface{}, foundDependencies *[]string, verbose bool) {
	switch v := data.(type) {
	case map[string]interface{}:
		// Analyze specific fields that usually contain dependencies
		processJSONObject(v, foundDependencies, verbose)

		// Recursively analyze all values
		for _, val := range v {
			processDependenciesInJSON(val, foundDependencies, verbose)
		}

	case []interface{}:
		// Analyze arrays
		for _, item := range v {
			processDependenciesInJSON(item, foundDependencies, verbose)
		}

	case string:
		// Process strings that could be package names
		processJSONString(v, foundDependencies)
	}
}

// processJSONObject processes JSON objects looking for specific keys that usually contain dependencies
func processJSONObject(obj map[string]interface{}, foundDependencies *[]string, verbose bool) {
	// List of keys that may contain package references
	dependencyFields := []string{
		"extends", "plugins", "presets", "parser", "module",
		"loader", "use", "require", "import", "dependency",
		"plugin", "preset", "eslintConfig", "babel", "transform",
		"scripts",
	}

	for _, field := range dependencyFields {
		if value, exists := obj[field]; exists {
			switch v := value.(type) {
			case string:
				// If the value is a string, add it directly
				*foundDependencies = append(*foundDependencies, v)

			case []interface{}:
				// If it's an array, process each element
				for _, item := range v {
					if str, ok := item.(string); ok {
						*foundDependencies = append(*foundDependencies, str)
					} else {
						processDependenciesInJSON(item, foundDependencies, verbose)
					}
				}

			default:
				// For other types, process recursively
				processDependenciesInJSON(value, foundDependencies, verbose)
			}
		}
	}
}

// processJSONString processes a string to determine if it could be a package
func processJSONString(str string, foundDependencies *[]string) {
	// Detect if a string looks like a package name
	// (doesn't start with ., /, http://, https://, doesn't contain spaces)
	if !strings.HasPrefix(str, ".") &&
		!strings.HasPrefix(str, "/") &&
		!strings.HasPrefix(str, "http://") &&
		!strings.HasPrefix(str, "https://") &&
		strings.Contains(str, "-") && // Most packages contain hyphens
		!strings.Contains(str, " ") && // Should not contain spaces
		!strings.Contains(str, "\\") { // Should not contain backslashes
		*foundDependencies = append(*foundDependencies, str)
	}
}

// findAllImports finds all imports in a file
func findAllImports(content string) []string {
	var imports []string

	// Look for ES6 imports
	matches := importRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			imports = append(imports, match[1])
		}
	}

	// Look for dynamic imports
	dynamicMatches := dynamicImportRegex.FindAllStringSubmatch(content, -1)
	for _, match := range dynamicMatches {
		if len(match) > 1 {
			imports = append(imports, match[1])
		}
	}

	// Look for requires
	requireMatches := requireRegex.FindAllStringSubmatch(content, -1)
	for _, match := range requireMatches {
		if len(match) > 1 {
			imports = append(imports, match[1])
		}
	}

	return imports
}

// normalizePackageName extracts the base name of the package
func normalizePackageName(importPath string) string {
	// Ignorar imports relativos y absolutos
	if strings.HasPrefix(importPath, ".") || strings.HasPrefix(importPath, "/") {
		return ""
	}
	// Ignorar imports tipo URL
	if strings.HasPrefix(importPath, "http://") || strings.HasPrefix(importPath, "https://") {
		return ""
	}
	// Handle imports with aliases (@organization/package)
	parts := strings.Split(importPath, "/")
	if strings.HasPrefix(parts[0], "@") && len(parts) > 1 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

// analyzePackageDependencies analyzes dependencies to determine which ones are unused
func analyzePackageDependencies(deps, devDeps, peerDeps, optDeps map[string]string, usedPackages map[string][]string) *CheckResults {
	results := &CheckResults{
		UsedDependencies: usedPackages,
	}

	// Check production dependencies
	for dep := range deps {
		if _, ok := usedPackages[dep]; !ok {
			// Check if it's an optional or peer dependency
			if _, isOpt := optDeps[dep]; !isOpt {
				if _, isPeer := peerDeps[dep]; !isPeer {
					results.UnusedDependencies = append(results.UnusedDependencies, dep)
				}
			}
		}
	}

	// Check development dependencies
	for dep := range devDeps {
		if _, ok := usedPackages[dep]; !ok {
			results.UnusedDevDependencies = append(results.UnusedDevDependencies, dep)
		}
	}

	return results
}

// printWorkspaceResults shows the analysis results for a workspace
func printWorkspaceResults(results *CheckResults, workspacePath string, verbose bool) {
	if workspacePath != "." {
		fmt.Printf("\n📊 Dependency Analysis for workspace: %s\n", workspacePath)
	} else {
		fmt.Println("\n📊 Dependency Analysis:")
	}

	if len(results.UnusedDependencies) > 0 {
		fmt.Printf("\n🔍 Unused dependencies (%d):\n", len(results.UnusedDependencies))
		for _, dep := range results.UnusedDependencies {
			fmt.Printf("  • %s\n", dep)
		}
	} else {
		fmt.Println("\n✅ No unused production dependencies found")
	}

	if len(results.UnusedDevDependencies) > 0 {
		fmt.Printf("\n🔍 Unused development dependencies (%d):\n", len(results.UnusedDevDependencies))
		for _, dep := range results.UnusedDevDependencies {
			fmt.Printf("  • %s\n", dep)
		}
	} else {
		fmt.Println("\n✅ No unused development dependencies found")
	}

	// If in verbose mode, show used dependencies
	if verbose {
		fmt.Printf("\n📦 Used dependencies (%d):\n", len(results.UsedDependencies))
		for dep, files := range results.UsedDependencies {
			fmt.Printf("  • %s (used in %d files)\n", dep, len(files))
			if len(files) > 0 && verbose {
				fmt.Printf("    Example: %s\n", files[0])
			}
		}
	}
}

// fixUnusedDependencies removes unused dependencies using packagejson methods
func fixUnusedDependencies(pkg *packagejson.PackageJSON, results []*CheckResults, flags *cli.Flags) error {
	if len(results) == 0 {
		fmt.Println("\n✅ No workspaces to process")
		return nil
	}

	modified := false

	// In dry run mode, just show what would be removed
	if flags.DryRun {
		for _, result := range results {
			if len(result.UnusedDependencies) > 0 || len(result.UnusedDevDependencies) > 0 {
				if result.WorkspacePath != "." {
					fmt.Printf("\n🔧 Changes for workspace: %s (dry-run)\n", result.WorkspacePath)
				} else {
					fmt.Println("\n🔧 Changes (dry-run):")
				}

				for _, dep := range result.UnusedDependencies {
					fmt.Printf("  • Would remove dependency: %s\n", dep)
				}

				for _, dep := range result.UnusedDevDependencies {
					fmt.Printf("  • Would remove dev dependency: %s\n", dep)
				}

				modified = true
			}
		}

		if modified {
			fmt.Println(styles.SuccessStyle.Render("\n⚠️ Dry-run mode: No changes made to package.json"))
		} else {
			fmt.Println("\n✅ No unused dependencies to remove")
		}

		return nil
	}

	// Create a map of dependencies to update for each workspace
	depsByWorkspace := make(map[string]dependency.Dependencies)

	for _, result := range results {
		if len(result.UnusedDependencies) > 0 || len(result.UnusedDevDependencies) > 0 {
			var deps dependency.Dependencies

			// Mark production dependencies for removal (setting next version to empty string)
			for _, depName := range result.UnusedDependencies {
				d, err := dependency.NewDependency(depName, "", constants.Dependencies, result.WorkspacePath)
				if err != nil {
					continue
				}
				d.NextVersion = nil // This will signal removal
				d.HaveToUpdate = true
				deps = append(deps, d)
				fmt.Printf("🗑️ Marking for removal: %s (dependency)\n", depName)
			}

			// Mark dev dependencies for removal
			for _, depName := range result.UnusedDevDependencies {
				d, err := dependency.NewDependency(depName, "", constants.DevDependencies, result.WorkspacePath)
				if err != nil {
					continue
				}
				d.NextVersion = nil // This will signal removal
				d.HaveToUpdate = true
				deps = append(deps, d)
				fmt.Printf("🗑️ Marking for removal: %s (dev dependency)\n", depName)
			}

			if len(deps) > 0 {
				depsByWorkspace[result.WorkspacePath] = deps
				modified = true
			}
		}
	}

	if !modified {
		fmt.Println("\n✅ No unused dependencies to remove")
		return nil
	}

	// Update package.json files for each workspace
	allError := true
	for workspace, workspacePkg := range pkg.WorkspacesPkgs {
		if deps, ok := depsByWorkspace[workspace]; ok {
			// Use the PackageJSON's UpdatePackageJSON method
			if err := workspacePkg.UpdatePackageJSON(flags, deps); err != nil {
				fmt.Printf("⚠️ Error updating package.json in %s: %v\n", workspace, err)
			} else {
				fmt.Printf("✅ Successfully removed unused dependencies in %s\n", workspace)
				if allError {
					allError = false
				}
			}
		}
	}

	if allError && modified {
		return errors.New("failed to update package.json files")
	}

	fmt.Println(styles.SuccessStyle.Render("\n✅ Unused dependencies removed successfully"))

	return nil
}

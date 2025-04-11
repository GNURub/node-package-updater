// filepath: /home/gnurub/code/node-package-updater/pkg/checkdeps/checkdeps.go
package checkdeps

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/styles"
)

// Dependency types
const (
	DependencyType     = "dependencies"
	DevDependencyType  = "devDependencies"
	PeerDependencyType = "peerDependencies"
	OptDependencyType  = "optionalDependencies"
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

// Patterns to detect imports
var (
	// ES6 imports: import xxx from 'package'
	importRegex = regexp.MustCompile(`import\s+(?:(?:\{[^}]+\}|\*\s+as\s+[^,]+|[^,{}\s*]+)(?:\s*,\s*(?:\{[^}]+\}|\*\s+as\s+[^,]+|[^,{}\s*]+))*\s+from\s+)?['"]([^'"]+)['"]`)

	// Dynamic imports: import('package')
	dynamicImportRegex = regexp.MustCompile(`import\s*\(\s*['"]([^'"]+)['"]\s*\)`)

	// Requires: require('package')
	requireRegex = regexp.MustCompile(`require\s*\(\s*['"]([^'"]+)['"]\s*\)`)
)

// PackageJSON represents the structure of the package.json file
type PackageJSON struct {
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
}

// CheckResults stores the analysis results
type CheckResults struct {
	UnusedDependencies    []string
	UnusedDevDependencies []string
	UsedDependencies      map[string][]string
}

// CheckUnusedDependencies looks for unused dependencies in an npm project
func CheckUnusedDependencies(flags *cli.Flags) error {
	baseDir := flags.BaseDir
	if baseDir == "" {
		baseDir = "."
	}

	// Read package.json
	packageJSONPath := filepath.Join(baseDir, "package.json")
	packageData, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return fmt.Errorf("error reading package.json: %w", err)
	}

	var packageJSON PackageJSON
	if err := json.Unmarshal(packageData, &packageJSON); err != nil {
		return fmt.Errorf("error parsing package.json: %w", err)
	}

	// Check if there are dependencies to analyze
	if len(packageJSON.Dependencies) == 0 && len(packageJSON.DevDependencies) == 0 {
		fmt.Println("⚠️ No dependencies found in package.json")
		return nil
	}

	// Find all files to scan
	files, err := findFilesToScan(baseDir)
	if err != nil {
		return err
	}

	if flags.Verbose {
		fmt.Printf("📂 Analyzing %d files for dependencies\n", len(files))
	}

	// Analyze files and find used dependencies
	usedPackages, err := findUsedDependencies(files, flags.Verbose)
	if err != nil {
		return err
	}

	// Determine which dependencies are not being used
	results := analyzeResults(packageJSON, usedPackages)

	// Show results
	printResults(results, flags.Verbose)

	// If fix option is specified, remove unused dependencies
	if flags.Fix {
		return removeDependencies(packageJSONPath, results, flags)
	}

	return nil
}

// findFilesToScan looks for files to analyze in the directory
func findFilesToScan(rootDir string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip node_modules folder
		if d.IsDir() && (d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".")) {
			return filepath.SkipDir
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

// findUsedDependencies looks for dependencies used in files
func findUsedDependencies(files []string, verbose bool) (map[string][]string, error) {
	usedPackages := make(map[string][]string)

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("error reading file %s: %w", file, err)
		}

		contentStr := string(content)
		ext := filepath.Ext(file)

		var imports []string

		// Process the file according to its extension
		if ext == ".json" {
			// Analyze JSON file to find package references
			jsonImports, err := findDependenciesInJSON(content, verbose)
			if err != nil && verbose {
				fmt.Printf("⚠️ Error analyzing JSON in %s: %v\n", file, err)
			}
			imports = append(imports, jsonImports...)
		} else {
			// Look for all imports in JS/TS files
			imports = findAllImports(contentStr)
		}

		for _, importName := range imports {
			// Normalize package name
			packageName := normalizePackageName(importName)

			if packageName != "" {
				relPath, err := filepath.Rel(".", file)
				if err != nil {
					relPath = file
				}

				usedPackages[packageName] = append(usedPackages[packageName], relPath)

				if verbose {
					fmt.Printf("📦 Dependency '%s' found in %s\n", packageName, relPath)
				}
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
	// Ignore relative imports
	if strings.HasPrefix(importPath, ".") || strings.HasPrefix(importPath, "/") {
		return ""
	}

	// Handle imports with aliases (@organization/package)
	parts := strings.Split(importPath, "/")

	// If it starts with @ it's an import with a scoped package (@org/package)
	if strings.HasPrefix(parts[0], "@") && len(parts) > 1 {
		return parts[0] + "/" + parts[1]
	}

	// If it's a subpath of a package (package/subpath)
	return parts[0]
}

// analyzeResults analyzes the results to determine unused dependencies
func analyzeResults(packageJSON PackageJSON, usedPackages map[string][]string) *CheckResults {
	results := &CheckResults{
		UsedDependencies: usedPackages,
	}

	// Check production dependencies
	for dep := range packageJSON.Dependencies {
		if _, ok := usedPackages[dep]; !ok {
			// Check if it's an optional or peer dependency
			if _, isOpt := packageJSON.OptionalDependencies[dep]; !isOpt {
				if _, isPeer := packageJSON.PeerDependencies[dep]; !isPeer {
					results.UnusedDependencies = append(results.UnusedDependencies, dep)
				}
			}
		}
	}

	// Check development dependencies
	for dep := range packageJSON.DevDependencies {
		if _, ok := usedPackages[dep]; !ok {
			results.UnusedDevDependencies = append(results.UnusedDevDependencies, dep)
		}
	}

	return results
}

// printResults shows the analysis results
func printResults(results *CheckResults, verbose bool) {
	fmt.Println("\n📊 Dependency Analysis:")

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

// removeDependencies removes unused dependencies from package.json
func removeDependencies(packageJSONPath string, results *CheckResults, flags *cli.Flags) error {
	if len(results.UnusedDependencies) == 0 && len(results.UnusedDevDependencies) == 0 {
		fmt.Println("\n✅ No unused dependencies to remove")
		return nil
	}

	// Read package.json
	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return fmt.Errorf("could not read package.json: %w", err)
	}

	var packageJSON map[string]interface{}
	if err := json.Unmarshal(data, &packageJSON); err != nil {
		return fmt.Errorf("error parsing package.json: %w", err)
	}

	// Remove unused dependencies
	modified := false

	if deps, ok := packageJSON["dependencies"].(map[string]interface{}); ok && len(results.UnusedDependencies) > 0 {
		for _, dep := range results.UnusedDependencies {
			if _, exists := deps[dep]; exists {
				delete(deps, dep)
				modified = true
				fmt.Printf("🗑️ Removing unused dependency: %s\n", dep)
			}
		}
		packageJSON["dependencies"] = deps
	}

	if devDeps, ok := packageJSON["devDependencies"].(map[string]interface{}); ok && len(results.UnusedDevDependencies) > 0 {
		for _, dep := range results.UnusedDevDependencies {
			if _, exists := devDeps[dep]; exists {
				delete(devDeps, dep)
				modified = true
				fmt.Printf("🗑️ Removing unused development dependency: %s\n", dep)
			}
		}
		packageJSON["devDependencies"] = devDeps
	}

	if !modified {
		return nil
	}

	// If in "dry run" mode, don't modify the file
	if flags.DryRun {
		fmt.Println(styles.SuccessStyle.Render("\n⚠️ Dry-run mode: No changes made to package.json"))
		return nil
	}

	// Save the modified package.json
	updatedData, err := json.MarshalIndent(packageJSON, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializing package.json: %w", err)
	}

	if err := os.WriteFile(packageJSONPath, updatedData, 0644); err != nil {
		return fmt.Errorf("error writing package.json: %w", err)
	}

	fmt.Println(styles.SuccessStyle.Render("\n✅ Unused dependencies removed from package.json"))

	// Reinstall dependencies if --noInstall wasn't specified
	if !flags.NoInstall {
		return reinstallDependencies(filepath.Dir(packageJSONPath), flags)
	}

	return nil
}

// reinstallDependencies reinstalls project dependencies
func reinstallDependencies(baseDir string, flags *cli.Flags) error {
	fmt.Println("\n📦 Reinstalling dependencies...")

	// Detect package manager
	packageManager := detectPackageManager(baseDir)

	var cmd string
	switch packageManager {
	case "yarn":
		cmd = "yarn install"
	case "pnpm":
		cmd = "pnpm install"
	case "bun":
		cmd = "bun install"
	default:
		cmd = "npm install"
	}

	// Run installation command
	if flags.Verbose {
		fmt.Printf("🔄 Running: %s\n", cmd)
	}

	fmt.Println(styles.SuccessStyle.Render("\n✅ Manually run '" + cmd + "' to update your node_modules"))

	return nil
}

// detectPackageManager detects the package manager used
func detectPackageManager(baseDir string) string {
	// Check lock files to determine the package manager
	if _, err := os.Stat(filepath.Join(baseDir, "yarn.lock")); err == nil {
		return "yarn"
	}

	if _, err := os.Stat(filepath.Join(baseDir, "pnpm-lock.yaml")); err == nil {
		return "pnpm"
	}

	if _, err := os.Stat(filepath.Join(baseDir, "bun.lock")); err == nil {
		return "bun"
	}

	return "npm"
}

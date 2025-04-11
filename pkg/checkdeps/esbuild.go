// filepath: /home/gnurub/code/node-package-updater/pkg/checkdeps/esbuild.go
package checkdeps

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

// EsbuildMetafile represents the structure of the esbuild metafile output
type EsbuildMetafile struct {
	Inputs map[string]struct {
		Bytes   int `json:"bytes"`
		Imports []struct {
			Path     string `json:"path"`
			Kind     string `json:"kind"`               // e.g., "import-statement", "require-call", "dynamic-import"
			Original string `json:"original,omitempty"` // Path as it appeared in the source
		} `json:"imports"`
		Format string `json:"format,omitempty"` // e.g., "esm", "cjs"
	} `json:"inputs"`
	Outputs map[string]struct {
		Imports []struct { // Imports needed by the *output* chunk
			Path string `json:"path"`
			Kind string `json:"kind"`
		} `json:"imports"`
		// ... other output fields
	} `json:"outputs"`
}

// ExtractDependenciesWithEsbuild uses esbuild API to parse a JS/TS file and extract its dependencies.
func ExtractDependenciesWithEsbuild(filePath string, verbose bool) ([]string, error) {
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for %s: %w", filePath, err)
	}

	// Configure esbuild build options for analysis
	buildOptions := api.BuildOptions{
		EntryPoints: []string{absFilePath},
		Bundle:      true,                // Need bundle true to resolve dependencies
		Metafile:    true,                // Generate the metafile containing dependency info
		Write:       false,               // Don't actually write output files
		Platform:    api.PlatformNeutral, // Or PlatformNode, PlatformBrowser
		Format:      api.FormatESModule,  // Format doesn't matter much for metafile only
		LogLevel:    api.LogLevelSilent,  // Suppress build logs unless errors occur
	}

	// Run esbuild
	result := api.Build(buildOptions)

	// Check for errors reported by esbuild
	if len(result.Errors) > 0 {
		// Format errors nicely
		errorMessages := api.FormatMessages(result.Errors, api.FormatMessagesOptions{
			Color: true, // Use terminal colors
			Kind:  api.ErrorMessage,
		})
		return nil, fmt.Errorf("esbuild failed:\n%s", string(errorMessages[0]))
	}

	if verbose && len(result.Warnings) > 0 {
		// Log warnings if in verbose mode
		warningMessages := api.FormatMessages(result.Warnings, api.FormatMessagesOptions{
			Color: true,
			Kind:  api.WarningMessage,
		})
		log.Printf("esbuild warnings:\n%s", string(warningMessages[0]))
	}

	// Check if metafile was generated
	if result.Metafile == "" {
		return nil, fmt.Errorf("esbuild did not generate a metafile for %s", filePath)
	}

	// Parse the metafile JSON
	var metafile EsbuildMetafile
	err = json.Unmarshal([]byte(result.Metafile), &metafile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse esbuild metafile JSON: %w", err)
	}

	// Extract unique dependency paths
	dependencies := make(map[string]struct{})
	foundInput := false

	for inputFileRelativePath, inputData := range metafile.Inputs {
		// For the main input file
		if filepath.Base(inputFileRelativePath) == filepath.Base(absFilePath) {
			foundInput = true
			if verbose {
				log.Printf("Analyzing dependencies for input: %s (format: %s)", inputFileRelativePath, inputData.Format)
			}

			for _, imp := range inputData.Imports {
				depPath := imp.Path
				if imp.Original != "" {
					depPath = imp.Original
				}

				// Skip relative imports
				if !strings.HasPrefix(depPath, ".") && !strings.HasPrefix(depPath, "/") {
					dependencies[depPath] = struct{}{}

					if verbose {
						log.Printf("  Found import: Path='%s', Kind='%s', Original='%s'", imp.Path, imp.Kind, imp.Original)
					}
				}
			}
		}
	}

	if !foundInput && verbose {
		log.Printf("Warning: Could not definitively match entry point '%s' in metafile inputs", absFilePath)
	}

	// Convert map keys to a slice
	depList := make([]string, 0, len(dependencies))
	for dep := range dependencies {
		// Normalize package names (extract base package name)
		packageName := normalizePackageName(dep)
		if packageName != "" {
			depList = append(depList, packageName)
		}
	}

	return depList, nil
}

// fallbackExtractDependencies extracts dependencies using regex when esbuild isn't available
func fallbackExtractDependencies(filePath string) ([]string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	contentStr := string(content)

	// Use the regex-based method
	imports := findAllImports(contentStr)

	// Normalize and deduplicate the imports
	dependencies := make(map[string]struct{})
	for _, importPath := range imports {
		packageName := normalizePackageName(importPath)
		if packageName != "" {
			dependencies[packageName] = struct{}{}
		}
	}

	// Convert map to slice
	depList := make([]string, 0, len(dependencies))
	for dep := range dependencies {
		depList = append(depList, dep)
	}

	return depList, nil
}

// ExtractDependencies extracts dependencies from a file, using esbuild API
// or falling back to regex-based extraction
func ExtractDependencies(filePath string, verbose bool) ([]string, error) {
	// Try using esbuild first
	deps, err := ExtractDependenciesWithEsbuild(filePath, verbose)
	if err == nil {
		return deps, nil
	}

	// Log the error but don't fail - fall back to regex method
	if verbose {
		log.Printf("Warning: esbuild extraction failed for %s: %v. Falling back to regex method.", filePath, err)
	}

	// Fall back to regex-based method
	return fallbackExtractDependencies(filePath)
}

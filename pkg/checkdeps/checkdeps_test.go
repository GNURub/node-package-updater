package checkdeps

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/GNURub/node-package-updater/internal/cli"
)

func setupTestFiles(t *testing.T) (string, func()) {
	// Create a temporary directory for the test files
	tempDir, err := os.MkdirTemp("", "checkdeps-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create test js file
	jsFile := filepath.Join(tempDir, "test.js")
	jsContent := `
		const react = require('react');
		import { useState } from 'react';
		import lodash from 'lodash';
		import * as moment from 'moment';
		
		// Dynamic import
		import('axios').then(axios => {
			// do something
		});
	`
	err = os.WriteFile(jsFile, []byte(jsContent), 0644)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to write js test file: %v", err)
	}

	// Create test json file
	jsonFile := filepath.Join(tempDir, "config.json")
	jsonContent := `
	{
		"extends": "eslint:recommended",
		"plugins": ["jest", "react"],
		"settings": {
			"import/resolver": {
				"node": {
					"extensions": [".js", ".jsx", ".ts", ".tsx"]
				}
			}
		}
	}
	`
	err = os.WriteFile(jsonFile, []byte(jsonContent), 0644)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to write json test file: %v", err)
	}

	// Create package.json
	packageJson := filepath.Join(tempDir, "package.json")
	packageContent := `
	{
		"name": "test-package",
		"version": "1.0.0",
		"dependencies": {
			"react": "^17.0.2",
			"lodash": "^4.17.21",
			"moment": "^2.29.1",
			"axios": "^0.21.1",
			"express": "^4.17.1",
			"unused-dep": "^1.0.0"
		},
		"devDependencies": {
			"jest": "^27.0.6",
			"eslint": "^7.32.0",
			"typescript": "^4.4.2",
			"prettier": "^2.3.2",
			"unused-dev-dep": "^1.0.0"
		}
	}
	`
	err = os.WriteFile(packageJson, []byte(packageContent), 0644)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to write package.json test file: %v", err)
	}

	// Return the temp directory and a cleanup function
	return tempDir, func() {
		os.RemoveAll(tempDir)
	}
}

func TestFindFilesToScan(t *testing.T) {
	tempDir, cleanup := setupTestFiles(t)
	defer cleanup()

	// Call the findFilesToScan function which is not exported,
	// so we're using reflection to access it
	findFilesToScan := reflect.ValueOf(findFilesToScan)
	results := findFilesToScan.Call([]reflect.Value{reflect.ValueOf(tempDir)})

	// Check for errors
	if !results[1].IsNil() {
		t.Fatalf("findFilesToScan returned an error: %v", results[1].Interface())
	}

	// Get the files
	files := results[0].Interface().([]string)

	// We expect 3 files: test.js, config.json, and package.json
	if len(files) != 3 {
		t.Errorf("Expected 3 files, got %d", len(files))
	}

	// Check if all expected files are found
	foundJS := false
	foundJSON := false
	foundPackageJSON := false

	for _, file := range files {
		basename := filepath.Base(file)
		switch basename {
		case "test.js":
			foundJS = true
		case "config.json":
			foundJSON = true
		case "package.json":
			foundPackageJSON = true
		}
	}

	if !foundJS || !foundJSON || !foundPackageJSON {
		t.Errorf("Not all expected files were found. JS: %v, JSON: %v, Package: %v",
			foundJS, foundJSON, foundPackageJSON)
	}
}

func TestExtractDependencies(t *testing.T) {
	tempDir, cleanup := setupTestFiles(t)
	defer cleanup()

	jsFile := filepath.Join(tempDir, "test.js")

	// Extract dependencies from the JS file
	deps, err := ExtractDependencies(jsFile, true)
	if err != nil {
		t.Fatalf("ExtractDependencies returned an error: %v", err)
	}

	// Sort the dependencies for deterministic comparison
	sort.Strings(deps)

	// We expect to find these dependencies
	expected := []string{"axios", "lodash", "moment", "react"}
	sort.Strings(expected)

	// Compare the results
	if !reflect.DeepEqual(deps, expected) {
		t.Errorf("Expected dependencies %v, got %v", expected, deps)
	}
}

func TestFindDependenciesInJSON(t *testing.T) {
	tempDir, cleanup := setupTestFiles(t)
	defer cleanup()

	jsonFile := filepath.Join(tempDir, "config.json")

	// Read the JSON file
	content, err := os.ReadFile(jsonFile)
	if err != nil {
		t.Fatalf("Failed to read JSON file: %v", err)
	}

	// Call the findDependenciesInJSON function which is not exported,
	// so we're using reflection to access it
	findDependenciesInJSON := reflect.ValueOf(findDependenciesInJSON)
	results := findDependenciesInJSON.Call([]reflect.Value{
		reflect.ValueOf(content),
		reflect.ValueOf(true),
	})

	// Check for errors
	if !results[1].IsNil() {
		t.Fatalf("findDependenciesInJSON returned an error: %v", results[1].Interface())
	}

	// Get the dependencies
	jsonDeps := results[0].Interface().([]string)

	// Convert to a map for easier checking
	depsMap := make(map[string]bool)
	for _, dep := range jsonDeps {
		depsMap[dep] = true
	}

	// Check if the expected dependencies are found
	if !depsMap["eslint:recommended"] || !depsMap["jest"] || !depsMap["react"] {
		t.Errorf("Not all expected dependencies were found in the JSON: %v", depsMap)
	}
}

func TestAnalyzePackageDependencies(t *testing.T) {
	// Test data
	deps := map[string]string{
		"react":      "^17.0.2",
		"lodash":     "^4.17.21",
		"moment":     "^2.29.1",
		"axios":      "^0.21.1",
		"express":    "^4.17.1",
		"unused-dep": "^1.0.0",
	}

	devDeps := map[string]string{
		"jest":           "^27.0.6",
		"eslint":         "^7.32.0",
		"typescript":     "^4.4.2",
		"prettier":       "^2.3.2",
		"unused-dev-dep": "^1.0.0",
	}

	peerDeps := map[string]string{}
	optDeps := map[string]string{}

	// Used packages
	usedPackages := map[string][]string{
		"react":      {"file1.js"},
		"lodash":     {"file1.js", "file2.js"},
		"moment":     {"file1.js"},
		"axios":      {"file1.js"},
		"jest":       {"test1.js"},
		"eslint":     {"config.json"},
		"typescript": {"tsconfig.json"},
		"prettier":   {"config.json"},
	}

	// Call the analyzePackageDependencies function which is not exported,
	// so we're using reflection to access it
	analyzePackageDependencies := reflect.ValueOf(analyzePackageDependencies)
	results := analyzePackageDependencies.Call([]reflect.Value{
		reflect.ValueOf(deps),
		reflect.ValueOf(devDeps),
		reflect.ValueOf(peerDeps),
		reflect.ValueOf(optDeps),
		reflect.ValueOf(usedPackages),
	})

	// Get the analysis results
	checkResults := results[0].Interface().(*CheckResults)

	// Sort the results for deterministic comparison
	sort.Strings(checkResults.UnusedDependencies)
	sort.Strings(checkResults.UnusedDevDependencies)

	// We expect these unused dependencies
	expectedUnused := []string{"express", "unused-dep"}
	expectedUnusedDev := []string{"unused-dev-dep"}

	sort.Strings(expectedUnused)
	sort.Strings(expectedUnusedDev)

	// Compare the results
	if !reflect.DeepEqual(checkResults.UnusedDependencies, expectedUnused) {
		t.Errorf("Expected unused dependencies %v, got %v", expectedUnused, checkResults.UnusedDependencies)
	}

	if !reflect.DeepEqual(checkResults.UnusedDevDependencies, expectedUnusedDev) {
		t.Errorf("Expected unused dev dependencies %v, got %v", expectedUnusedDev, checkResults.UnusedDevDependencies)
	}
}

func TestNormalizePackageName(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"Simple package", "lodash", "lodash"},
		{"Scoped package", "@angular/core", "@angular/core"},
		{"Package with path", "lodash/fp", "lodash"},
		{"Scoped package with path", "@angular/core/testing", "@angular/core"},
		{"Relative import", "./local", ""},
		{"Absolute import", "/absolute/path", ""},
		{"URL import", "https://example.com/script.js", ""},
	}

	normalizePackageName := reflect.ValueOf(normalizePackageName)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizePackageName.Call([]reflect.Value{reflect.ValueOf(tc.input)})

			if result[0].String() != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result[0].String())
			}
		})
	}
}

func TestCheckUnusedDependencies_DryRun(t *testing.T) {
	tempDir, cleanup := setupTestFiles(t)
	defer cleanup()

	// Save current directory and change to temp directory for the test
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(currentDir)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Setup flags for dry run
	flags := &cli.Flags{
		BaseDir: tempDir,
		DryRun:  true,
		Fix:     true,
		Verbose: true,
	}

	// Call CheckUnusedDependencies with dry run
	err = CheckUnusedDependencies(flags)
	if err != nil {
		t.Fatalf("CheckUnusedDependencies returned an error: %v", err)
	}

	// Verify package.json was not modified (should still contain unused dependencies)
	packageContent, err := os.ReadFile(filepath.Join(tempDir, "package.json"))
	if err != nil {
		t.Fatalf("Failed to read package.json: %v", err)
	}

	packageStr := string(packageContent)

	// Check if unused dependencies are still there
	if !strings.Contains(packageStr, "unused-dep") || !strings.Contains(packageStr, "unused-dev-dep") {
		t.Errorf("package.json should not be modified in dry run mode")
	}
}

package packagemanager

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/GNURub/node-package-updater/internal/dependency"
	"gopkg.in/yaml.v3"
)

type npmGoblaDeps struct {
	Deps map[string]struct {
		Version string `json:"version"`
	} `json:"dependencies"`
}

type pnpmGoblaDeps struct {
	Dependencies map[string]struct {
		Version string `json:"version"`
	} `json:"dependencies"`
}

type PackageManager struct {
	Name      string
	LockFiles []string
}

type pnpmWorkspace struct {
	Packages []string `yaml:"packages"`
}

var (
	Bun = &PackageManager{
		Name: "bun",
		LockFiles: []string{
			"bun.lockb",
			"bun.lock",
		},
	}

	Yarn = &PackageManager{
		Name:      "yarn",
		LockFiles: []string{"yarn.lock"},
	}

	Pnpm = &PackageManager{
		Name:      "pnpm",
		LockFiles: []string{"pnpm-lock.yaml"},
	}

	Deno = &PackageManager{
		Name:      "deno",
		LockFiles: []string{"deno.jsonc", "deno.json"},
	}

	Npm = &PackageManager{
		Name:      "npm",
		LockFiles: []string{"package-lock.json"},
	}
)

var SupportedPackageManagers = []*PackageManager{Bun, Yarn, Pnpm, Npm, Deno}

func (p *PackageManager) GetWorkspacesPaths(dir string, pkgJsonWorkspaces []string) []string {

	if p == Pnpm {
		ws, err := getWorkspacesFromPnpm(dir)

		if err != nil {
			pkgJsonWorkspaces = append(pkgJsonWorkspaces, ws...)
		}
	}

	var workspacePaths []string
	for _, workspace := range pkgJsonWorkspaces {
		matches, err := filepath.Glob(filepath.Join(dir, workspace))
		if err != nil {
			continue
		}

		for _, match := range matches {
			packageJSONPath := filepath.Join(match, "package.json")

			if fileInfo, err := os.Stat(packageJSONPath); err == nil && !fileInfo.IsDir() {
				workspacePaths = append(workspacePaths, filepath.Dir(packageJSONPath))
			}
		}
	}

	return workspacePaths
}

func Detect(projectPath, manager string) *PackageManager {
	if manager != "" {
		for _, pm := range SupportedPackageManagers {
			if strings.Contains(manager, pm.Name) {
				return pm
			}
		}
	} else {
		userAgent := os.Getenv("npm_config_user_agent")
		execpath := os.Getenv("npm_execpath")

		// Check for Bun first (less reliable in Go compared to JS, might need refinement)
		// Simple check based on user agent or execpath for now
		if strings.HasPrefix(userAgent, "bun/") || strings.Contains(execpath, "bun") {
			// Verify bun executable exists as a fallback check
			if _, err := exec.LookPath("bun"); err == nil {
				return Bun
			}
		}
		if strings.HasPrefix(userAgent, "yarn/") || strings.Contains(execpath, "yarn") {
			// Verify yarn executable exists
			if _, err := exec.LookPath("yarn"); err == nil {
				return Yarn
			}
		}
		if strings.HasPrefix(userAgent, "pnpm/") || strings.Contains(execpath, "pnpm") {
			// Verify pnpm executable exists
			if _, err := exec.LookPath("pnpm"); err == nil {
				return Pnpm
			}
		}
	}

	// Fallback to lockfile detection
	for _, pm := range SupportedPackageManagers {
		// Check if the command exists first, regardless of lockfile,
		// as the user might be in a directory without a lockfile but still wants to use a specific manager.
		_, cmdExists := exec.LookPath(pm.Name)
		if cmdExists != nil {
			continue
		}

		if projectPath != "" {
			for _, lockFile := range pm.LockFiles {
				if _, err := os.Stat(filepath.Join(projectPath, lockFile)); err == nil {
					// Ensure the command for the detected lockfile actually exists
					if _, err := exec.LookPath(pm.Name); err == nil {
						return pm
					}
					// If command doesn't exist, continue searching (e.g., yarn.lock exists but yarn isn't installed)
					break // Move to the next package manager check
				}
			}
		}
	}

	return Npm
}

func (pm *PackageManager) npmGlobalDeps() (dependency.Dependencies, error) {
	cmd := exec.Command("npm", "list", "-g", "--depth=0", "--json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list global dependencies: %w", err)
	}

	var deps npmGoblaDeps
	if err := json.Unmarshal(output, &deps); err != nil {
		return nil, fmt.Errorf("failed to parse global dependencies: %w", err)
	}

	var allDeps dependency.Dependencies
	for depName, dep := range deps.Deps {
		d, err := dependency.NewDependency(depName, dep.Version, "", "global")
		if err != nil {
			continue
		}
		allDeps = append(allDeps, d)
	}

	return allDeps, nil
}

func (pm *PackageManager) pnpmGlobalDeps() (dependency.Dependencies, error) {
	cmd := exec.Command("pnpm", "list", "-g", "--depth=0", "--json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list global dependencies: %w", err)
	}

	var deps []pnpmGoblaDeps
	if err := json.Unmarshal(output, &deps); err != nil {
		return nil, fmt.Errorf("failed to parse global dependencies: %w", err)
	}

	var allDeps dependency.Dependencies
	for _, dep := range deps {
		for depName, depInfo := range dep.Dependencies {
			d, err := dependency.NewDependency(depName, depInfo.Version, "", "global")
			if err != nil {
				continue
			}
			allDeps = append(allDeps, d)
		}

	}

	return allDeps, nil
}

func (pm *PackageManager) GetGlobalDeps() (dependency.Dependencies, error) {
	switch pm {
	case Npm:
		return pm.npmGlobalDeps()
	case Pnpm:
		return pm.pnpmGlobalDeps()
	case Bun:
		return nil, fmt.Errorf("bun does not support global dependencies")
	case Yarn:
		return nil, fmt.Errorf("yarn does not support global dependencies")
	case Deno:
		return nil, fmt.Errorf("deno does not support global dependencies")
	default:
		return nil, fmt.Errorf("npm does not support global dependencies")
	}
}

func (pm *PackageManager) installCommand(args ...string) []string {
	cmd := []string{}
	switch pm {
	case Bun:
		cmd = append(cmd, "bun", "install")
	case Yarn:
		cmd = append(cmd, "yarn", "install")
	case Pnpm:
		cmd = append(cmd, "pnpm", "install")
	case Deno:
		cmd = append(cmd, "deno", "install")
	default:
		cmd = append(cmd, "npm", "install")
	}

	if len(args) > 0 {
		cmd = append(cmd, args...)
	}

	return cmd
}

func (pm *PackageManager) Install(args ...string) error {
	command := pm.installCommand(args...)
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/C", strings.Join(command, " "))
	default:
		cmd = exec.Command("sh", "-c", strings.Join(command, " "))
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func getWorkspacesFromPnpm(dir string) ([]string, error) {
	var workspacePaths []string

	fileContent, err := os.ReadFile(filepath.Join(dir, "pnpm-workspace.yaml"))
	if err != nil {
		return nil, fmt.Errorf("error reading pnpm-workspace.yaml: %w", err)
	}

	var workspaceConfig pnpmWorkspace
	err = yaml.Unmarshal(fileContent, &workspaceConfig)
	if err != nil {
		return nil, fmt.Errorf("error parsing YAML: %w", err)
	}

	for _, pattern := range workspaceConfig.Packages {
		isExclusion := strings.HasPrefix(pattern, "!")

		if isExclusion {
			continue
		}

		workspacePaths = append(workspacePaths, pattern)
	}

	return workspacePaths, nil
}

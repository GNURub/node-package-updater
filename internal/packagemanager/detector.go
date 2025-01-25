package packagemanager

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

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

func (p *PackageManager) GetWorkspacesPaths(dir string, pkgJsonWorkspaces []string) []string {

	if p == Pnpm {
		ws, err := getWorkspacesFromPnpm(dir)

		if err != nil {
			pkgJsonWorkspaces = append(pkgJsonWorkspaces, ws...)
		}
	}

	var workspacePaths []string
	for _, workspace := range pkgJsonWorkspaces {
		matches, err := filepath.Glob(workspace)
		if err != nil {
			continue
		}

		for _, match := range matches {
			packageJSONPath := filepath.Join(dir, match, "package.json")

			if fileInfo, err := os.Stat(packageJSONPath); err == nil && !fileInfo.IsDir() {
				workspacePaths = append(workspacePaths, filepath.Dir(packageJSONPath))
			}
		}
	}

	return workspacePaths
}

func Detect(projectPath, manager string) *PackageManager {
	supportedPackageManagers := []*PackageManager{Bun, Yarn, Pnpm, Npm}

	if manager != "" {
		for _, pm := range supportedPackageManagers {
			if strings.Contains(manager, pm.Name) {
				return pm
			}
		}
	}
	for _, pm := range supportedPackageManagers {
		_, cmdExists := exec.LookPath(pm.Name)
		if cmdExists != nil {
			continue
		}

		for _, lockFile := range pm.LockFiles {
			if _, err := os.Stat(filepath.Join(projectPath, lockFile)); err == nil {
				return pm
			}
		}
	}

	return Npm
}

func (pm *PackageManager) installCommand() []string {
	switch pm {
	case Bun:
		return []string{"bun", "install"}
	case Yarn:
		return []string{"yarn", "install"}
	case Pnpm:
		return []string{"pnpm", "install"}
	case Npm:
		return []string{"npm", "install"}
	default:
		return []string{"npm", "install"}
	}
}

func (pm *PackageManager) Install() error {
	command := pm.installCommand()
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

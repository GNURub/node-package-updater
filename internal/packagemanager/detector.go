// internal/packagemanager/detector.go
package packagemanager

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type PackageManager struct {
	Name     string
	LockFile string
}

var (
	Bun = &PackageManager{
		Name:     "bun",
		LockFile: "bun.lockb",
	}

	Yarn = &PackageManager{
		Name:     "yarn",
		LockFile: "yarn.lock",
	}

	Pnpm = &PackageManager{
		Name:     "pnpm",
		LockFile: "pnpm-lock.yaml",
	}

	Npm = &PackageManager{
		Name:     "npm",
		LockFile: "package-lock.json",
	}
)

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
		lockPath := filepath.Join(projectPath, pm.LockFile)
		if _, err := os.Stat(lockPath); err == nil {
			if _, err := exec.LookPath(pm.Name); err == nil {
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

	// Detecta la plataforma y selecciona el shell adecuado
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

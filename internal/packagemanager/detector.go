// internal/packagemanager/detector.go
package packagemanager

import (
	"os"
	"path/filepath"
)

type PackageManager struct {
	Name     string
	LockFile string
}

var (
	Bun = PackageManager{
		Name:     "bun",
		LockFile: "bun.lockb",
	}

	Yarn = PackageManager{
		Name:     "yarn",
		LockFile: "yarn.lock",
	}

	Pnpm = PackageManager{
		Name:     "pnpm",
		LockFile: "pnpm-lock.yaml",
	}

	Npm = PackageManager{
		Name:     "npm",
		LockFile: "package-lock.json",
	}
)

func Detect(projectPath string) PackageManager {
	// Buscar archivos de lock en orden de prioridad
	lockFiles := []PackageManager{Bun, Yarn, Pnpm, Npm}

	for _, pm := range lockFiles {
		lockPath := filepath.Join(projectPath, pm.LockFile)
		if _, err := os.Stat(lockPath); err == nil {
			return pm
		}
	}

	// Por defecto retorna npm si no se encuentra ningún archivo de lock
	return Npm
}

func (pm PackageManager) InstallCommand() string {
	switch pm {
	case Bun:
		return "bun install"
	case Yarn:
		return "yarn install"
	case Pnpm:
		return "pnpm install"
	case Npm:
		return "npm install"
	default:
		return "npm install"
	}
}

package dependency

import (
	"errors"
	"fmt"

	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/semver"
)

var (
	// ErrInvalidCurrentVersion indica que la versión actual no es válida
	ErrInvalidCurrentVersion = errors.New("invalid current version")
	// ErrNilVersions indica que no se proporcionaron versiones
	ErrNilVersions = errors.New("versions cannot be nil")
	// ErrNilFlags indica que no se proporcionaron flags
	ErrNilFlags = errors.New("flags cannot be nil")
)

// VersionType representa el tipo de actualización de versión
type VersionType int

const (
	// Major indica actualización de versión mayor
	Major VersionType = iota
	// Minor indica actualización de versión menor
	Minor
	// Patch indica actualización de parche
	Patch
)

// VersionManager gestiona la lógica de actualización de versiones
type VersionManager struct {
	isLatest       bool            // indica si la versión actual es 'latest', '*' o vacía
	currentVersion *semver.Version // versión actual del paquete
	versions       []*Version      // lista de versiones disponibles
}

// NewVersionManager crea una nueva instancia de VersionManager con validación
func NewVersionManager(currentVersion *semver.Version, versions *Versions, flags *cli.Flags) (*VersionManager, error) {
	if currentVersion == nil {
		return nil, fmt.Errorf("%w: current version is required", ErrInvalidCurrentVersion)
	}
	if versions == nil {
		return nil, fmt.Errorf("%w: versions list is required", ErrNilVersions)
	}
	if flags == nil {
		return nil, fmt.Errorf("%w: flags are required", ErrNilFlags)
	}

	vs := versions.Values()

	// Si se debe mantener el operador de rango, aplicarlo a todas las versiones
	if flags.KeepRangeOperator {
		for _, v := range vs {
			v.Version.SetPrefix(currentVersion.Prefix())
		}
	}

	return &VersionManager{
		isLatest:       isLatestVersion(currentVersion.String()),
		currentVersion: currentVersion,
		versions:       vs,
	}, nil
}

// isLatestVersion verifica si una versión es considerada "latest"
func isLatestVersion(version string) bool {
	return version == "latest" || version == "*" || version == ""
}

// GetUpdatedVersion determina la siguiente versión a la que actualizar
// basándose en los flags proporcionados y las restricciones de versión
func (vm *VersionManager) GetUpdatedVersion(flags *cli.Flags) (*Version, error) {
	if flags == nil {
		return nil, ErrNilFlags
	}

	var latestVersion *Version

	// Determinar el tipo de actualización basado en los flags
	updateType := vm.determineUpdateType(flags)

	// Iterar sobre todas las versiones disponibles
	for _, v := range vm.versions {
		// Saltar versiones que no cumplen con los criterios básicos
		if !vm.isValidVersionUpdate(v, flags) {
			continue
		}

		// Aplicar restricciones según el tipo de actualización
		if !vm.isValidUpdateType(v, updateType) {
			continue
		}

		// Actualizar la última versión válida si corresponde
		if latestVersion == nil || v.Compare(latestVersion.Version) > 0 {
			latestVersion = v
		}
	}

	// Verificar si se encontró una nueva versión válida
	if latestVersion == nil || vm.currentVersion.Compare(latestVersion.Version) == 0 {
		return nil, nil
	}

	return latestVersion, nil
}

// determineUpdateType determina el tipo de actualización basado en los flags
func (vm *VersionManager) determineUpdateType(flags *cli.Flags) VersionType {
	switch {
	case flags.Patch:
		return Patch
	case flags.Minor:
		return Minor
	default:
		return Major
	}
}

// isValidVersionUpdate verifica si una versión cumple con los criterios básicos de actualización
func (vm *VersionManager) isValidVersionUpdate(v *Version, flags *cli.Flags) bool {
	// No considerar versiones menores o iguales a la actual
	if v.Compare(vm.currentVersion) <= 0 {
		return false
	}

	// Verificar restricciones de semver si está habilitado
	if flags.MaintainSemver && !vm.currentVersion.Check(v.Version) {
		return false
	}

	// Verificar si se permiten versiones preliminares
	if v.Version.Prerelease() != "" && !flags.Pre {
		return false
	}

	return true
}

// isValidUpdateType verifica si una versión cumple con el tipo de actualización especificado
func (vm *VersionManager) isValidUpdateType(v *Version, updateType VersionType) bool {
	switch updateType {
	case Patch:
		return vm.currentVersion.Major() == v.Version.Major() &&
			vm.currentVersion.Minor() == v.Version.Minor()
	case Minor:
		return vm.currentVersion.Major() == v.Version.Major()
	default:
		return true
	}
}

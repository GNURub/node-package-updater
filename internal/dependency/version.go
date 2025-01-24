package dependency

import (
	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/semver"
)

type VersionManager struct {
	latest         bool
	currentVersion *semver.Version
	versions       []*Version
}

func NewVersionManager(currentVersion *semver.Version, versions *Versions, flags *cli.Flags) (*VersionManager, error) {

	vs := versions.Values()
	if flags.KeepRangeOperator {
		for _, v := range vs {
			v.Version.SetPrefix(currentVersion.Prefix())
		}
	}
	return &VersionManager{
		latest:         currentVersion.String() == "latest" || currentVersion.String() == "*" || currentVersion.String() == "",
		currentVersion: currentVersion,
		versions:       vs,
	}, nil
}

func (vm *VersionManager) GetUpdatedVersion(flags *cli.Flags) (*semver.Version, error) {
	var latestVersion *semver.Version

	for _, v := range vm.versions {
		// Si la versión es menor o igual a la actual, la ignoramos
		if v.Compare(vm.currentVersion) <= 0 {
			continue
		}

		// Si MaintainSemver está activado y la versión no cumple con el prefijo, la ignoramos
		if flags.MaintainSemver && !vm.currentVersion.Check(v.Version) {
			continue
		}

		// Si la versión es un prerelease y no estamos buscando prereleases, la ignoramos
		if v.Version.Prerelease() != "" && !flags.Pre {
			continue
		}

		// Si estamos buscando una versión mayor, permitimos actualizaciones de major, minor o patch
		if !flags.Minor && !flags.Patch {
			if latestVersion == nil || v.Compare(latestVersion) > 0 {
				latestVersion = v.Version
			}
			continue
		}

		// Si estamos buscando una versión menor, permitimos actualizaciones de minor o patch
		if flags.Minor {
			if vm.currentVersion.Major() == v.Version.Major() {
				if latestVersion == nil || v.Compare(latestVersion) > 0 {
					latestVersion = v.Version
				}
			}
			continue
		}

		// Si estamos buscando una versión de parche, solo permitimos actualizaciones de patch
		if flags.Patch {
			if vm.currentVersion.Major() == v.Version.Major() && vm.currentVersion.Minor() == v.Version.Minor() {
				if latestVersion == nil || v.Compare(latestVersion) > 0 {
					latestVersion = v.Version
				}
			}
			continue
		}
	}

	if latestVersion == nil {
		return nil, nil
	}

	if vm.currentVersion.Compare(latestVersion) == 0 {
		return nil, nil
	}

	return latestVersion, nil
}

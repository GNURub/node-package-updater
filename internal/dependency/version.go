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
		if v.Compare(vm.currentVersion) <= 0 {
			continue
		}

		if flags.MaintainSemver && !vm.currentVersion.Check(v.Version) {
			continue
		}

		if v.Version.Prerelease() != "" && !flags.Pre {
			continue
		}

		if !flags.Minor && !flags.Patch {
			if latestVersion == nil || v.Compare(latestVersion) > 0 {
				latestVersion = v.Version
			}
			continue
		}

		if flags.Minor {
			if vm.currentVersion.Major() == v.Version.Major() {
				if latestVersion == nil || v.Compare(latestVersion) > 0 {
					latestVersion = v.Version
				}
			}
			continue
		}

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

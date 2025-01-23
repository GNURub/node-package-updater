package dependency

import (
	"fmt"

	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/semver"
)

type VersionManager struct {
	latest         bool
	currentVersion *semver.Version
	versions       []*Version
}

func NewVersionManager(currentVersion *semver.Version, versions *Versions, flags *cli.Flags) (*VersionManager, error) {

	if !flags.MaintainSemver {
		if flags.Minor {
			currentVersion.SetPrefix("^")
		} else if flags.Patch {
			currentVersion.SetPrefix("~")
		} else {
			currentVersion.SetPrefix(">=")
		}
	}

	return &VersionManager{
		latest:         currentVersion.String() == "latest" || currentVersion.String() == "*" || currentVersion.String() == "",
		currentVersion: currentVersion,
		versions:       versions.Values(),
	}, nil
}

func (vm *VersionManager) GetUpdatedVersion(flags *cli.Flags) (*semver.Version, error) {
	var latestVersion *semver.Version

	for _, v := range vm.versions {
		if (vm.latest || vm.currentVersion.Check(v.Version)) &&
			(latestVersion == nil || v.Compare(latestVersion) > 0) {
			latestVersion = v.Version
		}
	}

	if latestVersion == nil {
		return nil, fmt.Errorf("no matching version found")
	}

	if vm.currentVersion.Compare(latestVersion) == 0 {
		return nil, nil
	}

	return latestVersion, nil
}

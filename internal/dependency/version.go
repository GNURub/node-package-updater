package dependency

import (
	"fmt"
	"strings"

	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/Masterminds/semver/v3"
)

type VersionManager struct {
	latest            bool
	currentVersionStr string
	currentVersion    *semver.Version
	currentReq        *semver.Constraints
	versions          []*Version
}

func NewVersionManager(current string, versions *Versions, flags *cli.Flags) (*VersionManager, error) {
	currentOnlyVersion := strings.TrimLeft(current, ">=<^~")
	currentOnlyVersion = strings.TrimSpace(currentOnlyVersion)

	currentVersion, _ := semver.NewVersion(currentOnlyVersion)

	var currentReq *semver.Constraints
	if flags.MaintainSemver {
		currentReq, _ = semver.NewConstraint(current)
	} else if flags.Minor {
		currentReq, _ = semver.NewConstraint(fmt.Sprintf("^%s", currentOnlyVersion))
	} else if flags.Patch {
		currentReq, _ = semver.NewConstraint(fmt.Sprintf("~%s", currentOnlyVersion))
	} else {
		currentReq, _ = semver.NewConstraint(fmt.Sprintf(">=%s", currentOnlyVersion))
	}

	return &VersionManager{
		latest:            current == "latest" || current == "*" || current == "",
		currentVersion:    currentVersion,
		currentReq:        currentReq,
		versions:          versions.Values(),
		currentVersionStr: current,
	}, nil
}

func (vm *VersionManager) GetUpdatedVersion(flags *cli.Flags) (*semver.Version, error) {
	var latestVersion *semver.Version

	for _, v := range vm.versions {
		if (vm.latest || vm.currentReq == nil || vm.currentReq.Check(v.Version)) &&
			(latestVersion == nil || v.GreaterThan(latestVersion)) {
			latestVersion = v.Version
		}
	}

	if latestVersion == nil {
		return nil, fmt.Errorf("no matching version found")
	}

	if vm.currentVersion.Equal(latestVersion) {
		return nil, nil
	}

	return latestVersion, nil
}

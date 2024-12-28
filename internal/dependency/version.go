package dependency

import (
	"fmt"
	"strings"

	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/Masterminds/semver/v3"
)

type VersionManager struct {
	currentVersion *semver.Version
	currentReq     *semver.Constraints
	versions       []*semver.Version
}

func NewVersionManager(current string, versions []string, flags *cli.Flags) (*VersionManager, error) {
	currentOnlyVersion := strings.TrimLeft(current, ">=<^~")
	currentOnlyVersion = strings.TrimSpace(currentOnlyVersion)

	currentVersion, err := semver.NewVersion(currentOnlyVersion)

	if err != nil {
		return nil, err
	}

	var currentReq *semver.Constraints
	if flags.Major {
		currentReq, err = semver.NewConstraint(fmt.Sprintf(">=%s", currentOnlyVersion))
		if err != nil {
			return nil, err
		}
	} else if flags.Minor {
		currentReq, err = semver.NewConstraint(fmt.Sprintf("^%s", currentOnlyVersion))
		if err != nil {
			return nil, err
		}
	} else if flags.Patch {
		currentReq, err = semver.NewConstraint(fmt.Sprintf("~%s", currentOnlyVersion))
		if err != nil {
			return nil, err
		}
	} else {
		currentReq, err = semver.NewConstraint(current)
		if err != nil {
			return nil, err
		}
	}

	var parsedVersions []*semver.Version
	for _, v := range versions {
		version, err := semver.NewVersion(v)
		if err != nil {
			continue
		}
		parsedVersions = append(parsedVersions, version)
	}

	return &VersionManager{
		currentVersion: currentVersion,
		currentReq:     currentReq,
		versions:       parsedVersions,
	}, nil
}

func (vm *VersionManager) GetUpdatedVersion(flags *cli.Flags) (*semver.Version, error) {
	var latestVersion *semver.Version
	for _, v := range vm.versions {
		if vm.currentReq.Check(v) && (latestVersion == nil || v.GreaterThan(latestVersion)) {
			latestVersion = v
		}
	}

	if latestVersion == nil {
		return nil, fmt.Errorf("no matching version found")
	}

	return latestVersion, nil
}

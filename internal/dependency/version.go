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
	versions          []*semver.Version
}

func NewVersionManager(current string, versions []string, flags *cli.Flags) (*VersionManager, error) {
	currentOnlyVersion := strings.TrimLeft(current, ">=<^~")
	currentOnlyVersion = strings.TrimSpace(currentOnlyVersion)

	currentVersion, _ := semver.NewVersion(currentOnlyVersion)

	var currentReq *semver.Constraints
	if flags.Major {
		currentReq, _ = semver.NewConstraint(fmt.Sprintf(">=%s", currentOnlyVersion))
	} else if flags.Minor {
		currentReq, _ = semver.NewConstraint(fmt.Sprintf("^%s", currentOnlyVersion))
	} else if flags.Patch {
		currentReq, _ = semver.NewConstraint(fmt.Sprintf("~%s", currentOnlyVersion))
	} else {
		currentReq, _ = semver.NewConstraint(current)
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
		latest:            current == "latest" || current == "*" || current == "",
		currentVersion:    currentVersion,
		currentReq:        currentReq,
		versions:          parsedVersions,
		currentVersionStr: current,
	}, nil
}

func (vm *VersionManager) GetUpdatedVersion(flags *cli.Flags) (string, error) {
	var latestVersion *semver.Version

	for _, v := range vm.versions {
		if vm.latest || vm.currentReq == nil {
			latestVersion = v
			break
		}

		if vm.currentReq.Check(v) && (latestVersion == nil || v.GreaterThan(latestVersion)) {
			latestVersion = v
		}
	}

	if latestVersion == nil {
		return "", fmt.Errorf("no matching version found")
	}

	if vm.currentVersion.Equal(latestVersion) {
		return "", nil
	}

	if flags.KeepRangeOperator {
		prefixes := []string{">=", ">", "^", "~"}
		for _, prefix := range prefixes {
			if strings.HasPrefix(vm.currentVersionStr, prefix) {
				return fmt.Sprintf("%s%s", prefix, latestVersion), nil
			}
		}
	}

	return latestVersion.String(), nil
}

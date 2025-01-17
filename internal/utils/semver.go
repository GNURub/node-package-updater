package utils

import "strings"

var SEMVER_PREFIXES = []string{">=", ">", "^", "~"}

func GetPrefix(version string) string {
	for _, prefix := range SEMVER_PREFIXES {
		if strings.HasPrefix(version, prefix) {
			return prefix
		}
	}

	return ""
}

func GetVersionWithoutPrefix(version string) string {
	for _, prefix := range SEMVER_PREFIXES {
		if strings.HasPrefix(version, prefix) {
			return strings.TrimLeft(version, prefix)
		}
	}

	return version
}

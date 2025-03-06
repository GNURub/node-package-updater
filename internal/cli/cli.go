package cli

import (
	"fmt"
)

type Flags struct {
	BaseDir           string
	CleanCache        bool
	ConfigFile        string
	Depth             uint8
	DryRun            bool
	Exclude           []string
	Filter            string
	Include           []string
	KeepRangeOperator bool
	LogLevel          string
	MaintainSemver    bool
	Minor             bool
	NoInstall         bool
	NoInteractive     bool
	PackageManager    string
	Patch             bool
	PeerDependencies  bool
	Pre               bool
	Production        bool
	Registry          string
	CPUs              int
	SkipDeprecated    bool
	Timeout           int
	Verbose           bool
	WithWorkspaces    bool
}

func (f *Flags) ValidateFlags() error {
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[f.LogLevel] {
		return fmt.Errorf("invalid log level: must be one of debug, info, warn, error")
	}

	validPackageManagers := map[string]bool{"npm": true, "yarn": true, "pnpm": true, "bun": true, "deno": true}
	if f.PackageManager != "" && !validPackageManagers[f.PackageManager] {
		return fmt.Errorf("invalid package manager: must be one of npm, yarn, pnpm, bun, deno")
	}

	if f.Timeout < 1 {
		return fmt.Errorf("timeout must be greater than 0")
	}

	if f.CPUs < 1 {
		return fmt.Errorf("cpus must be greater than 0")
	}

	return nil
}

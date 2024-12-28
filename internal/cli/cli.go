package cli

import "flag"

type Flags struct {
	BaseDir          string
	UpdateAll        bool
	Major            bool
	Minor            bool
	Patch            bool
	Workspaces       bool
	Interactive      bool
	Production       bool
	PeerDependencies bool
	MaintainSemver   bool
	Registry         string
}

func ParseFlags() *Flags {
	flags := &Flags{}

	flag.BoolVar(&flags.Interactive, "i", false, "Interactive mode")
	flag.BoolVar(&flags.MaintainSemver, "m", true, "Maintain semver satisfaction")
	flag.BoolVar(&flags.Production, "p", false, "Update only production dependencies")
	flag.BoolVar(&flags.Major, "major", false, "Update to latest major versions")
	flag.BoolVar(&flags.Minor, "minor", false, "Update to latest minor versions")
	flag.BoolVar(&flags.Patch, "patch", false, "Update to latest patch versions")
	flag.BoolVar(&flags.PeerDependencies, "peer", false, "Include peer dependencies")
	flag.BoolVar(&flags.UpdateAll, "u", false, "Update all dependencies to latest versions")
	flag.BoolVar(&flags.Workspaces, "ws", false, "Include workspace repositories")
	flag.StringVar(&flags.BaseDir, "d", ".", "Root directory")
	flag.StringVar(&flags.Registry, "r", "https://registry.npmjs.org/", "NPM registry URL")

	flag.Parse()
	return flags
}

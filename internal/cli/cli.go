package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

type Flags struct {
	BaseDir          string
	UpdateAll        bool
	Major            bool
	Minor            bool
	Patch            bool
	Workspaces       bool
	NoInteractive    bool
	Production       bool
	PeerDependencies bool
	MaintainSemver   bool
	Registry         string
	DryRun           bool
	Verbose          bool
	LogLevel         string
	OutputFormat     string
	Include          []string
	Exclude          []string
	ConfigFile       string
	Timeout          int
}

// arrayFlags permite manejar flags que aceptan múltiples valores
type arrayFlags []string

func (i *arrayFlags) String() string {
	return strings.Join(*i, ",")
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func ParseFlags() *Flags {
	flags := &Flags{}
	var includeFlags, excludeFlags arrayFlags

	// Opciones básicas
	flag.StringVar(&flags.BaseDir, "d", ".", "Root directory for package search")
	flag.StringVar(&flags.Registry, "r", "https://registry.npmjs.org/", "NPM registry URL")
	flag.StringVar(&flags.ConfigFile, "config", "", "Path to config file (default: .npmrc)")

	// Opciones de actualización
	flag.BoolVar(&flags.UpdateAll, "u", false, "Update all dependencies to latest versions")
	flag.BoolVar(&flags.Major, "major", false, "Update to latest major versions")
	flag.BoolVar(&flags.Minor, "minor", false, "Update to latest minor versions")
	flag.BoolVar(&flags.Patch, "patch", false, "Update to latest patch versions")
	flag.BoolVar(&flags.MaintainSemver, "m", true, "Maintain semver satisfaction")

	// Tipos de dependencias
	flag.BoolVar(&flags.Production, "prod", false, "Update only production dependencies")
	flag.BoolVar(&flags.PeerDependencies, "peer", false, "Include peer dependencies")

	// Opciones de workspace
	flag.BoolVar(&flags.Workspaces, "ws", false, "Include workspace repositories")

	// Opciones de comportamiento
	flag.BoolVar(&flags.NoInteractive, "ni", false, "Non-interactive mode")
	flag.BoolVar(&flags.DryRun, "dry-run", false, "Show what would be updated without making changes")
	flag.BoolVar(&flags.Verbose, "verbose", false, "Show detailed output")
	flag.StringVar(&flags.LogLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.StringVar(&flags.OutputFormat, "output", "text", "Output format (text, json, yaml)")

	// Opciones de rendimiento
	flag.IntVar(&flags.Timeout, "timeout", 30, "Timeout in seconds for each package update")

	// Filtros
	flag.Var(&includeFlags, "include", "Packages to include (can be specified multiple times)")
	flag.Var(&excludeFlags, "exclude", "Packages to exclude (can be specified multiple times)")

	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nUpdate Options:\n")
		fmt.Fprintf(os.Stderr, "  -u\t\tUpdate all dependencies to latest versions\n")
		fmt.Fprintf(os.Stderr, "  -major\tUpdate to latest major versions\n")
		fmt.Fprintf(os.Stderr, "  -minor\tUpdate to latest minor versions\n")
		fmt.Fprintf(os.Stderr, "  -patch\tUpdate to latest patch versions\n")
		fmt.Fprintf(os.Stderr, "  -m\t\tMaintain semver satisfaction (default: true)\n")
		fmt.Fprintf(os.Stderr, "\nDependency Types:\n")
		fmt.Fprintf(os.Stderr, "  -prod\tUpdate only production dependencies\n")
		fmt.Fprintf(os.Stderr, "  -peer\tInclude peer dependencies\n")
		fmt.Fprintf(os.Stderr, "\nBehavior Options:\n")
		fmt.Fprintf(os.Stderr, "  -ni\t\tNon-interactive mode\n")
		fmt.Fprintf(os.Stderr, "  -dry-run\tShow what would be updated without making changes\n")
		fmt.Fprintf(os.Stderr, "  -verbose\tShow detailed output\n")
		fmt.Fprintf(os.Stderr, "\nAdvanced Options:\n")
		fmt.Fprintf(os.Stderr, "  -d string\tRoot directory for package search (default: .)\n")
		fmt.Fprintf(os.Stderr, "  -r string\tNPM registry URL (default: https://registry.npmjs.org/)\n")
		fmt.Fprintf(os.Stderr, "  -config string\tPath to config file\n")
		fmt.Fprintf(os.Stderr, "  -jobs int\tNumber of concurrent update jobs (default: 4)\n")
		fmt.Fprintf(os.Stderr, "  -timeout int\tTimeout in seconds for each package update (default: 30)\n")
		fmt.Fprintf(os.Stderr, "  -log-level string\tLog level: debug, info, warn, error (default: info)\n")
		fmt.Fprintf(os.Stderr, "  -output string\tOutput format: text, json, yaml (default: text)\n")
		fmt.Fprintf(os.Stderr, "\nFilters:\n")
		fmt.Fprintf(os.Stderr, "  -include string\tPackages to include (can be specified multiple times)\n")
		fmt.Fprintf(os.Stderr, "  -exclude string\tPackages to exclude (can be specified multiple times)\n")
	}

	flag.Parse()

	// Convertir los array flags a slices en la estructura Flags
	flags.Include = includeFlags
	flags.Exclude = excludeFlags

	// Validar el nivel de log
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[flags.LogLevel] {
		fmt.Fprintf(os.Stderr, "Error: Invalid log level. Must be one of: debug, info, warn, error\n")
		os.Exit(1)
	}

	// Validar el formato de salida
	validOutputFormats := map[string]bool{"text": true, "json": true, "yaml": true}
	if !validOutputFormats[flags.OutputFormat] {
		fmt.Fprintf(os.Stderr, "Error: Invalid output format. Must be one of: text, json, yaml\n")
		os.Exit(1)
	}

	return flags
}

func (f *Flags) ValidateFlags() error {
	if f.Timeout < 1 {
		return fmt.Errorf("timeout must be greater than 0")
	}

	return nil
}

func (f *Flags) String() string {
	return fmt.Sprintf(
		"BaseDir: %s\nRegistry: %s\nUpdateAll: %v\nMajor: %v\nMinor: %v\nPatch: %v\n"+
			"Production: %v\n"+
			"DryRun: %v\nVerbose: %v\nLogLevel: %s\nOutputFormat: %s\n"+
			"Include: %v\nExclude: %v\nTimeout: %d",
		f.BaseDir, f.Registry, f.UpdateAll, f.Major, f.Minor, f.Patch,
		f.Production,
		f.DryRun, f.Verbose, f.LogLevel, f.OutputFormat,
		f.Include, f.Exclude, f.Timeout,
	)
}

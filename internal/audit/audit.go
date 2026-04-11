package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/GNURub/node-package-updater/internal/packagejson"
	"github.com/GNURub/node-package-updater/internal/packagemanager"
)

type OutputFormat string

const (
	FormatText OutputFormat = "text"
	FormatJSON OutputFormat = "json"
)

type Options struct {
	Path                   string
	PackageManagerOverride string
	Offline                bool
	DownloadDB             bool
	DBPath                 string
}

type ScanRequest struct {
	RootDir     string
	Lockfiles   []string
	Directories []string
	Offline     bool
	DownloadDB  bool
	DBPath      string
}

type Summary struct {
	RootDir            string `json:"root_dir"`
	LockfilesScanned   int    `json:"lockfiles_scanned"`
	DirectoriesScanned int    `json:"directories_scanned"`
	TotalFindings      int    `json:"total_findings"`
	AffectedPackages   int    `json:"affected_packages"`
}

type Finding struct {
	ID             string   `json:"id"`
	PackageName    string   `json:"package_name"`
	Version        string   `json:"version"`
	Ecosystem      string   `json:"ecosystem"`
	Severity       string   `json:"severity,omitempty"`
	SourcePath     string   `json:"source_path"`
	SourceType     string   `json:"source_type"`
	Summary        string   `json:"summary,omitempty"`
	Details        string   `json:"details,omitempty"`
	Aliases        []string `json:"aliases,omitempty"`
	Recommendation string   `json:"recommendation"`
}

type Result struct {
	Summary  Summary   `json:"summary"`
	Findings []Finding `json:"findings"`
}

type scanner interface {
	Scan(context.Context, ScanRequest) (*Result, error)
}

type Service struct {
	scanner scanner
}

func NewService() *Service {
	return &Service{scanner: newOSVScanner()}
}

func NewServiceWithScanner(s scanner) *Service {
	return &Service{scanner: s}
}

func (s *Service) Audit(ctx context.Context, opts Options) (*Result, error) {
	if opts.DownloadDB && !opts.Offline {
		return nil, errors.New("--download-db requires --offline")
	}

	if opts.Offline || opts.DownloadDB {
		if err := os.MkdirAll(defaultDBPath(opts.DBPath), 0o755); err != nil {
			return nil, fmt.Errorf("failed to prepare OSV database path: %w", err)
		}
	}

	rootDir, rootPM, err := packagejson.ResolveProjectRoot(opts.Path, opts.PackageManagerOverride)
	if err != nil {
		return nil, err
	}

	loadOptions := []packagejson.Option{
		packagejson.WithPackageManager(rootPM),
		packagejson.EnableWorkspaces(),
	}

	rootPkg, err := packagejson.LoadPackageJSON(rootDir, loadOptions...)
	if err != nil {
		return nil, err
	}

	targets := buildScanRequest(rootPkg, opts)
	return s.scanner.Scan(ctx, targets)
}

func buildScanRequest(rootPkg *packagejson.PackageJSON, opts Options) ScanRequest {
	rootDir := filepath.Clean(rootPkg.Dir)
	rootPM := packagemanager.Detect(rootDir, rootPkg.PackageJson.Manager)
	rootLockfiles := rootPM.FindLockfiles(rootDir)

	lockfileSet := make(map[string]struct{})
	directorySet := make(map[string]struct{})

	for _, lockfile := range rootLockfiles {
		lockfileSet[filepath.Clean(lockfile)] = struct{}{}
	}

	if len(rootLockfiles) == 0 && packageHasOwnDependencies(rootPkg) {
		directorySet[rootDir] = struct{}{}
	}

	for workspacePath, workspacePkg := range rootPkg.WorkspacesPkgs {
		cleanWorkspace := filepath.Clean(workspacePath)
		if cleanWorkspace == rootDir {
			continue
		}

		workspacePM := packagemanager.Detect(cleanWorkspace, workspacePkg.PackageJson.Manager)
		workspaceLockfiles := workspacePM.FindLockfiles(cleanWorkspace)
		if len(workspaceLockfiles) > 0 {
			for _, lockfile := range workspaceLockfiles {
				lockfileSet[filepath.Clean(lockfile)] = struct{}{}
			}
			continue
		}

		if len(rootLockfiles) == 0 {
			directorySet[cleanWorkspace] = struct{}{}
		}
	}

	lockfiles := setToSortedSlice(lockfileSet)
	directories := setToSortedSlice(directorySet)

	return ScanRequest{
		RootDir:     rootDir,
		Lockfiles:   lockfiles,
		Directories: directories,
		Offline:     opts.Offline,
		DownloadDB:  opts.DownloadDB,
		DBPath:      defaultDBPath(opts.DBPath),
	}
}

func packageHasOwnDependencies(pkg *packagejson.PackageJSON) bool {
	return len(pkg.PackageJson.Dependencies) > 0 ||
		len(pkg.PackageJson.DevDependencies) > 0 ||
		len(pkg.PackageJson.PeerDependencies) > 0 ||
		len(pkg.PackageJson.OptionalDependencies) > 0
}

func setToSortedSlice(values map[string]struct{}) []string {
	items := make([]string, 0, len(values))
	for value := range values {
		items = append(items, value)
	}
	sort.Strings(items)
	return items
}

func defaultDBPath(customPath string) string {
	if strings.TrimSpace(customPath) != "" {
		return customPath
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "npu", "osv-db")
	}

	return filepath.Join(cacheDir, "npu", "osv-db")
}

func Format(result *Result, format OutputFormat) ([]byte, error) {
	switch format {
	case FormatJSON:
		return json.MarshalIndent(result, "", "  ")
	case FormatText:
		return []byte(formatText(result)), nil
	default:
		return nil, fmt.Errorf("unsupported output format: %s", format)
	}
}

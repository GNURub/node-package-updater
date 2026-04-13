package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type osvScanner struct {
	client *OSVClient
}

type scannedPackage struct {
	Name       string
	Version    string
	SourcePath string
	SourceType string
}

type packageLockFile struct {
	Packages     map[string]packageLockPackage    `json:"packages"`
	Dependencies map[string]packageLockDependency `json:"dependencies"`
}

type packageLockPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type packageLockDependency struct {
	Version      string                           `json:"version"`
	Dependencies map[string]packageLockDependency `json:"dependencies"`
}

type pnpmLockFile struct {
	Packages map[string]pnpmLockPackage `yaml:"packages"`
}

type pnpmLockPackage struct {
	Version string `yaml:"version"`
}

type manifestFile struct {
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
}

func newOSVScanner() scanner {
	return osvScanner{client: NewOSVClient()}
}

func (s osvScanner) Scan(ctx context.Context, req ScanRequest) (*Result, error) {
	packages, err := collectScannedPackages(req)
	if err != nil {
		return nil, err
	}

	findings := make([]Finding, 0)
	affectedPackages := make(map[string]struct{})

	if len(packages) > 0 {
		queries := make([]BatchQuery, len(packages))
		for i, pkg := range packages {
			queries[i] = BatchQuery{Name: pkg.Name, Version: pkg.Version}
		}

		batchResults, err := s.client.QueryBatch(ctx, queries)
		if err != nil {
			return nil, err
		}

		detailCache := make(map[string][]Vulnerability)
		for i, pkg := range packages {
			if i >= len(batchResults) || batchResults[i].Count == 0 {
				continue
			}

			packageKey := fmt.Sprintf("%s@%s", pkg.Name, pkg.Version)
			affectedPackages[packageKey] = struct{}{}

			vulns, ok := detailCache[packageKey]
			if !ok {
				vulns, err = s.client.QueryPackage(ctx, pkg.Name, pkg.Version)
				if err != nil {
					return nil, err
				}
				detailCache[packageKey] = vulns
			}

			for _, vuln := range vulns {
				if vuln.ID == "" {
					continue
				}

				findings = append(findings, Finding{
					ID:             vuln.ID,
					PackageName:    pkg.Name,
					Version:        pkg.Version,
					Ecosystem:      "npm",
					Severity:       severityForVulnerability(vuln),
					SourcePath:     pkg.SourcePath,
					SourceType:     pkg.SourceType,
					Summary:        strings.TrimSpace(vuln.Summary),
					Details:        compactDetails(vuln.Details),
					Aliases:        vuln.Aliases,
					Recommendation: buildRecommendation(pkg.Name),
				})
			}
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].PackageName != findings[j].PackageName {
			return findings[i].PackageName < findings[j].PackageName
		}
		if findings[i].Version != findings[j].Version {
			return findings[i].Version < findings[j].Version
		}
		return findings[i].ID < findings[j].ID
	})

	return &Result{
		Summary: Summary{
			RootDir:            req.RootDir,
			LockfilesScanned:   len(req.Lockfiles),
			DirectoriesScanned: len(req.Directories),
			TotalFindings:      len(findings),
			AffectedPackages:   len(affectedPackages),
		},
		Findings: findings,
	}, nil
}

func collectScannedPackages(req ScanRequest) ([]scannedPackage, error) {
	collected := make([]scannedPackage, 0)
	seen := make(map[string]struct{})

	for _, lockfile := range req.Lockfiles {
		packages, err := collectPackagesFromLockfile(lockfile)
		if err != nil {
			return nil, err
		}
		appendUniquePackages(&collected, seen, packages)
	}

	for _, dir := range req.Directories {
		packages, err := collectPackagesFromManifestDir(dir)
		if err != nil {
			return nil, err
		}
		appendUniquePackages(&collected, seen, packages)
	}

	return collected, nil
}

func appendUniquePackages(target *[]scannedPackage, seen map[string]struct{}, packages []scannedPackage) {
	for _, pkg := range packages {
		key := pkg.SourcePath + "|" + pkg.Name + "|" + pkg.Version
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		*target = append(*target, pkg)
	}
}

func collectPackagesFromLockfile(lockfilePath string) ([]scannedPackage, error) {
	switch filepath.Base(lockfilePath) {
	case "package-lock.json", "npm-shrinkwrap.json":
		return parsePackageLock(lockfilePath)
	case "pnpm-lock.yaml":
		return parsePNPMLock(lockfilePath)
	case "yarn.lock":
		return parseYarnLock(lockfilePath)
	default:
		return collectPackagesFromManifestDir(filepath.Dir(lockfilePath))
	}
}

func parsePackageLock(lockfilePath string) ([]scannedPackage, error) {
	data, err := os.ReadFile(lockfilePath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", lockfilePath, err)
	}

	var lockfile packageLockFile
	if err := json.Unmarshal(data, &lockfile); err != nil {
		return nil, fmt.Errorf("parse %s: %w", lockfilePath, err)
	}

	packages := make([]scannedPackage, 0)
	seen := make(map[string]struct{})

	if len(lockfile.Packages) > 0 {
		for packagePath, pkg := range lockfile.Packages {
			if packagePath == "" {
				continue
			}

			name := pkg.Name
			if name == "" {
				name = derivePackageNameFromNodeModulesPath(packagePath)
			}
			version := normalizeResolvedVersion(pkg.Version)
			if name == "" || version == "" {
				continue
			}

			appendPackage(&packages, seen, scannedPackage{
				Name:       name,
				Version:    version,
				SourcePath: lockfilePath,
				SourceType: "lockfile",
			})
		}

		return packages, nil
	}

	var walkDependencies func(map[string]packageLockDependency)
	walkDependencies = func(deps map[string]packageLockDependency) {
		for name, dep := range deps {
			version := normalizeResolvedVersion(dep.Version)
			if name != "" && version != "" {
				appendPackage(&packages, seen, scannedPackage{
					Name:       name,
					Version:    version,
					SourcePath: lockfilePath,
					SourceType: "lockfile",
				})
			}
			walkDependencies(dep.Dependencies)
		}
	}

	walkDependencies(lockfile.Dependencies)
	return packages, nil
}

func parsePNPMLock(lockfilePath string) ([]scannedPackage, error) {
	data, err := os.ReadFile(lockfilePath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", lockfilePath, err)
	}

	var lockfile pnpmLockFile
	if err := yaml.Unmarshal(data, &lockfile); err != nil {
		return nil, fmt.Errorf("parse %s: %w", lockfilePath, err)
	}

	packages := make([]scannedPackage, 0, len(lockfile.Packages))
	seen := make(map[string]struct{})
	for key, pkg := range lockfile.Packages {
		name, version := parsePNPMPackageKey(key)
		if version == "" {
			version = normalizeResolvedVersion(pkg.Version)
		}
		if name == "" || version == "" {
			continue
		}

		appendPackage(&packages, seen, scannedPackage{
			Name:       name,
			Version:    version,
			SourcePath: lockfilePath,
			SourceType: "lockfile",
		})
	}

	return packages, nil
}

func parseYarnLock(lockfilePath string) ([]scannedPackage, error) {
	file, err := os.Open(lockfilePath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", lockfilePath, err)
	}
	defer file.Close()

	packages := make([]scannedPackage, 0)
	seen := make(map[string]struct{})
	var currentNames []string
	var currentVersion string

	flush := func() {
		if currentVersion == "" {
			currentNames = nil
			return
		}
		for _, name := range currentNames {
			appendPackage(&packages, seen, scannedPackage{
				Name:       name,
				Version:    currentVersion,
				SourcePath: lockfilePath,
				SourceType: "lockfile",
			})
		}
		currentNames = nil
		currentVersion = ""
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if line == trimmed && strings.HasSuffix(trimmed, ":") {
			flush()
			currentNames = parseYarnSelectors(strings.TrimSuffix(trimmed, ":"))
			continue
		}

		if len(currentNames) == 0 || !strings.HasPrefix(trimmed, "version ") {
			continue
		}

		currentVersion = normalizeResolvedVersion(strings.Trim(strings.TrimPrefix(trimmed, "version "), "\"'"))
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", lockfilePath, err)
	}

	flush()
	return packages, nil
}

func collectPackagesFromManifestDir(dir string) ([]scannedPackage, error) {
	manifestPath := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", manifestPath, err)
	}

	var manifest manifestFile
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse %s: %w", manifestPath, err)
	}

	packages := make([]scannedPackage, 0)
	seen := make(map[string]struct{})
	for _, deps := range []map[string]string{
		manifest.Dependencies,
		manifest.DevDependencies,
		manifest.PeerDependencies,
		manifest.OptionalDependencies,
	} {
		for name, versionRange := range deps {
			version := normalizeManifestVersion(versionRange)
			if name == "" || version == "" {
				continue
			}

			appendPackage(&packages, seen, scannedPackage{
				Name:       name,
				Version:    version,
				SourcePath: manifestPath,
				SourceType: "package.json",
			})
		}
	}

	return packages, nil
}

func appendPackage(target *[]scannedPackage, seen map[string]struct{}, pkg scannedPackage) {
	key := pkg.Name + "|" + pkg.Version
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	*target = append(*target, pkg)
}

func derivePackageNameFromNodeModulesPath(packagePath string) string {
	idx := strings.LastIndex(packagePath, "node_modules/")
	if idx == -1 {
		return ""
	}
	name := packagePath[idx+len("node_modules/"):]
	parts := strings.Split(name, "/")
	if len(parts) == 0 {
		return ""
	}
	if strings.HasPrefix(parts[0], "@") && len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

func parsePNPMPackageKey(key string) (string, string) {
	trimmed := strings.TrimPrefix(key, "/")
	trimmed = strings.Split(trimmed, "(")[0]
	idx := strings.LastIndex(trimmed, "@")
	if idx <= 0 || idx == len(trimmed)-1 {
		return "", ""
	}
	return trimmed[:idx], normalizeResolvedVersion(trimmed[idx+1:])
}

func parseYarnSelectors(header string) []string {
	parts := strings.Split(header, ",")
	names := make([]string, 0, len(parts))
	seen := make(map[string]struct{})
	for _, part := range parts {
		name := parseYarnPackageName(strings.TrimSpace(strings.Trim(part, "\"'")))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}

func parseYarnPackageName(selector string) string {
	if selector == "" {
		return ""
	}
	if strings.HasPrefix(selector, "@") {
		idx := strings.Index(selector[1:], "@")
		if idx == -1 {
			return ""
		}
		return selector[:idx+1]
	}
	idx := strings.Index(selector, "@")
	if idx == -1 {
		return ""
	}
	return selector[:idx]
}

func normalizeResolvedVersion(version string) string {
	version = strings.TrimSpace(strings.Trim(version, "\"'"))
	version = strings.TrimPrefix(version, "v")
	version = strings.Split(version, "(")[0]
	if !looksLikeExactVersion(version) {
		return ""
	}
	return version
}

func normalizeManifestVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}

	lower := strings.ToLower(version)
	for _, prefix := range []string{"workspace:", "file:", "link:", "git+", "github:", "http://", "https://", "npm:"} {
		if strings.HasPrefix(lower, prefix) {
			return ""
		}
	}

	if strings.ContainsAny(version, "*xX ") || strings.Contains(version, "||") {
		return ""
	}

	version = strings.TrimLeft(version, "^~<>= ")
	version = strings.TrimPrefix(version, "v")
	if !looksLikeExactVersion(version) {
		return ""
	}
	return version
}

func looksLikeExactVersion(version string) bool {
	if version == "" {
		return false
	}
	parts := strings.SplitN(version, "+", 2)
	base := parts[0]
	base = strings.SplitN(base, "-", 2)[0]
	segments := strings.Split(base, ".")
	if len(segments) != 3 {
		return false
	}
	for _, segment := range segments {
		if segment == "" {
			return false
		}
		for _, ch := range segment {
			if ch < '0' || ch > '9' {
				return false
			}
		}
	}
	return true
}

func severityForVulnerability(vuln Vulnerability) string {
	return ExtractMaxSeverity([]Vulnerability{vuln})
}

func compactDetails(details string) string {
	details = strings.TrimSpace(details)
	if details == "" {
		return ""
	}

	details = strings.Join(strings.Fields(details), " ")
	if len(details) > 180 {
		return details[:177] + "..."
	}

	return details
}

func buildRecommendation(packageName string) string {
	return fmt.Sprintf(
		"Upgrade %s to a patched version with your project package manager and rerun `npu audit`.",
		packageName,
	)
}

package dependency

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/GNURub/node-package-updater/internal/cache"
	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/semver"
	"github.com/iancoleman/orderedmap"
	"github.com/valyala/fasthttp"
)

var (
	ErrInvalidCache     = errors.New("cache is nil")
	ErrKeyNotFound      = errors.New("key not found")
	ErrInvalidVersion   = errors.New("invalid version")
	ErrRegistryResponse = errors.New("npm registry error")
)

// NpmRegistryResponse represents the response structure from the NPM registry
type NpmRegistryResponse struct {
	DistTags map[string]string `json:"dist-tags"`
	Versions map[string]struct {
		Dist *struct {
			Name         string `json:"name"`
			UnpackedSize uint64 `json:"unpackedSize"`
		} `json:"dist"`
		Deprecated string `json:"deprecated"`
	} `json:"versions"`
}

// NpmrcConfig holds NPM configuration from .npmrc file
type NpmrcConfig struct {
	Registry  string
	AuthToken string
	Scope     string
	mu        sync.RWMutex
}

type versionJson struct {
	Version    string `json:"version"`
	Weight     uint64 `json:"weight"`
	Deprecated bool   `json:"deprecated"`
}

// Version represents a single package version with metadata
type Version struct {
	*semver.Version
	VersionStr string `json:"version"`
	Weight     uint64 `json:"weight"`
	Deprecated bool   `json:"deprecated"`
}

// Versions manages an ordered collection of package versions
type Versions struct {
	*orderedmap.OrderedMap
	mu sync.RWMutex
}

func NewVersions() *Versions {
	return &Versions{
		OrderedMap: orderedmap.New(),
	}
}

func (v *Versions) SetVersions(versions []*Version) *Versions {
	v.mu.Lock()
	defer v.mu.Unlock()

	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Compare(versions[j].Version) > 0
	})

	for _, version := range versions {
		v.OrderedMap.Set(version.Version.String(), version)
	}

	return v
}

func (v *Versions) Values() []*Version {
	v.mu.RLock()
	defer v.mu.RUnlock()

	keys := v.OrderedMap.Keys()
	versions := make([]*Version, len(keys))

	for i, key := range keys {
		if value, ok := v.OrderedMap.Get(key); ok {
			versions[i] = value.(*Version)
		}
	}

	return versions
}

func (v *Versions) GetVersion(version string) *Version {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if version, ok := v.Get(version); ok {
		return version.(*Version)
	}
	return nil
}

func (v *Versions) ListVersions() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.Keys()
}

func (v *Versions) Len() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.OrderedMap.Keys())
}

func (v *Versions) Less(i, j int) bool {
	values := v.Values()
	return values[i].Compare(values[j].Version) < 0
}

func (v *Versions) Swap(i, j int) {
	values := v.Values()
	values[i], values[j] = values[j], values[i]
}

func (versions *Versions) Save(pkgName string, cache *cache.Cache) error {
	versions.mu.RLock()
	defer versions.mu.RUnlock()

	var m = make(map[string]versionJson)
	for _, version := range versions.Values() {
		m[version.String()] = versionJson{
			Version:    version.VersionStr,
			Weight:     version.Weight,
			Deprecated: version.Deprecated,
		}
	}

	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)

	if err := encoder.Encode(m); err != nil {
		return fmt.Errorf("error encoding versions: %w", err)
	}

	return cache.Set(pkgName, buf.Bytes())
}

func (versions *Versions) Restore(pkgName string, cache *cache.Cache) error {
	if cache == nil {
		return ErrInvalidCache
	}

	if !cache.Has(pkgName) {
		return ErrKeyNotFound
	}

	d, err := cache.Get(pkgName)
	if err != nil {
		return fmt.Errorf("error getting from cache: %w", err)
	}

	buf := bytes.NewBuffer(d)
	decoder := gob.NewDecoder(buf)

	var m = make(map[string]versionJson)
	if err := decoder.Decode(&m); err != nil {
		return fmt.Errorf("error decoding versions: %w", err)
	}

	vs := make([]*Version, 0, len(m))
	for _, version := range m {
		vs = append(vs, &Version{
			Version:    semver.NewSemver(version.Version),
			VersionStr: version.Version,
			Weight:     version.Weight,
			Deprecated: version.Deprecated,
		})
	}

	versions.SetVersions(vs)
	return nil
}

// Dependency represents a package dependency with version information
type Dependency struct {
	Versions       *Versions
	PackageName    string
	CurrentVersion *semver.Version
	LatestVersion  *semver.Version
	NextVersion    *Version
	HaveToUpdate   bool
	Env            string
	Workspace      string
	mu             sync.RWMutex
}

type Dependencies []*Dependency

// Implement sort.Interface for Dependencies
func (d Dependencies) Len() int {
	return len(d)
}

func (d Dependencies) Less(i, j int) bool {
	envI := d[i].getScoreForSort()
	envJ := d[j].getScoreForSort()

	if envI != envJ {
		return envI < envJ
	}

	packageNameDifference := strings.Compare(d[i].PackageName, d[j].PackageName)
	if packageNameDifference != 0 {
		return packageNameDifference < 0
	}

	return strings.Compare(d[i].Workspace, d[j].Workspace) < 0
}

func (d Dependencies) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}

func (d Dependencies) FilterByRegex(filter string) Dependencies {
	r, err := regexp.Compile(filter)
	if err != nil {
		return d
	}

	filtered := make(Dependencies, 0, len(d))
	for _, dep := range d {
		match := r.Match([]byte(dep.PackageName))
		if match {
			filtered = append(filtered, dep)
		}
	}
	return filtered
}

func (d Dependencies) FilterWithNewVersion() Dependencies {
	filtered := make(Dependencies, 0, len(d))
	for _, dep := range d {
		if dep.NextVersion != nil {
			filtered = append(filtered, dep)
		}
	}
	return filtered
}

func (d Dependencies) FilterForUpdate() Dependencies {
	filtered := make(Dependencies, 0, len(d))
	for _, dep := range d {
		if dep.HaveToUpdate {
			filtered = append(filtered, dep)
		}
	}
	return filtered
}

func parseNpmrc() (*NpmrcConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("error getting home directory: %w", err)
	}

	npmrcPath := ".npmrc"
	if _, err := os.Stat(npmrcPath); os.IsNotExist(err) {
		npmrcPath = filepath.Join(home, ".npmrc")
	}

	content, err := os.ReadFile(npmrcPath)
	if err != nil {
		return nil, fmt.Errorf("error reading .npmrc: %w", err)
	}

	config := &NpmrcConfig{}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, "=")
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch {
		case strings.HasSuffix(key, ":registry"):
			config.Registry = value
			config.Scope = strings.TrimSuffix(key, ":registry")
		case strings.HasSuffix(key, ":_authToken"):
			config.AuthToken = value
		}
	}

	return config, nil
}

func NewDependency(packageName, currentVersion, env, workspace string) (*Dependency, error) {
	version := semver.NewSemver(currentVersion)
	if !version.IsValid() {
		return nil, fmt.Errorf("%w: %s", ErrInvalidVersion, currentVersion)
	}

	return &Dependency{
		PackageName:    packageName,
		CurrentVersion: version,
		Versions:       NewVersions(),
		Env:            env,
		Workspace:      workspace,
	}, nil
}

func (d *Dependency) getScoreForSort() int {
	switch d.Env {
	case "prod":
		return 0
	case "dev":
		return 1
	default:
		return 2
	}
}

func (d *Dependency) FetchNewVersion(ctx context.Context, flags *cli.Flags, cache *cache.Cache) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	var etag string
	reqToNpm := true

	if cache != nil {
		if err := d.Versions.Restore(d.PackageName, cache); err == nil {
			if cachedEtag, err := cache.Get(d.PackageName + "-etag"); err == nil {
				etag = string(cachedEtag)
				if remoteEtag, err := headEtagFromRegistry(flags.Registry, d.PackageName); err == nil {
					reqToNpm = etag != remoteEtag
				}
			}
		}
	}

	if reqToNpm {
		var err error
		etag, d.LatestVersion, d.Versions, err = getVersionsFromRegistry(flags.Registry, d.PackageName)
		if err != nil {
			return fmt.Errorf("failed to fetch versions for %s: %w", d.PackageName, err)
		}

		if cache != nil {
			if err := cache.Set(d.PackageName+"-etag", []byte(etag)); err != nil {
				return fmt.Errorf("failed to cache etag for %s: %w", d.PackageName, err)
			}
			if err := d.Versions.Save(d.PackageName, cache); err != nil {
				return fmt.Errorf("failed to cache versions for %s: %w", d.PackageName, err)
			}
		}
	}

	vm, err := NewVersionManager(d.CurrentVersion, d.Versions, flags)
	if err != nil {
		return fmt.Errorf("failed to create version manager for %s: %w", d.PackageName, err)
	}

	if d.NextVersion, err = vm.GetUpdatedVersion(flags); err != nil {
		return fmt.Errorf("failed to determine updated version for %s: %w", d.PackageName, err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func makeRegistryRequest(method, registry, packageName string, npmConfig *NpmrcConfig) (*fasthttp.Response, error) {
	isPrivate := strings.HasPrefix(packageName, npmConfig.Scope)
	registryURL := registry
	if isPrivate && npmConfig.Registry != "" {
		registryURL = npmConfig.Registry
	}

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.SetRequestURI(fmt.Sprintf("%s/%s", registryURL, packageName))
	req.Header.SetMethod(method)
	req.Header.Set("Accept", "application/vnd.npm.install-v1+json; q=1.0, application/json; q=0.8, */*")
	req.Header.Set("User-Agent", "node-package-updater")

	if isPrivate && npmConfig.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+npmConfig.AuthToken)
	}

	resp := fasthttp.AcquireResponse()
	if err := fasthttp.Do(req, resp); err != nil {
		fasthttp.ReleaseResponse(resp)
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

func getVersionsFromRegistry(registry, packageName string) (string, *semver.Version, *Versions, error) {
	npmConfig, err := parseNpmrc()
	if err != nil {
		return "", nil, nil, fmt.Errorf("error parsing .npmrc: %w", err)
	}

	resp, err := makeRegistryRequest("GET", registry, packageName, npmConfig)
	if err != nil {
		return "", nil, nil, err
	}
	defer fasthttp.ReleaseResponse(resp)

	if resp.StatusCode() != fasthttp.StatusOK {
		return "", nil, nil, fmt.Errorf("%w: status %d: %s", ErrRegistryResponse, resp.StatusCode(), string(resp.Body()))
	}

	var npmResp NpmRegistryResponse
	if err := json.Unmarshal(resp.Body(), &npmResp); err != nil {
		return "", nil, nil, fmt.Errorf("error parsing JSON response: %w", err)
	}

	etag := string(resp.Header.Peek("ETag"))

	versions := make([]*Version, 0, len(npmResp.Versions))
	for version, v := range npmResp.Versions {
		if v.Dist == nil {
			continue
		}

		versions = append(versions, &Version{
			Version:    semver.NewSemver(version),
			VersionStr: version,
			Weight:     v.Dist.UnpackedSize,
			Deprecated: v.Deprecated != "",
		})
	}

	var latestVersion *semver.Version
	if latest, ok := npmResp.DistTags["latest"]; ok {
		latestVersion = semver.NewSemver(latest)
	}

	return etag, latestVersion, NewVersions().SetVersions(versions), nil
}

func headEtagFromRegistry(registry, packageName string) (string, error) {
	npmConfig, err := parseNpmrc()
	if err != nil {
		return "", fmt.Errorf("error parsing .npmrc: %w", err)
	}

	resp, err := makeRegistryRequest("HEAD", registry, packageName, npmConfig)
	if err != nil {
		return "", err
	}
	defer fasthttp.ReleaseResponse(resp)

	if resp.StatusCode() != fasthttp.StatusOK {
		return "", fmt.Errorf("%w: status %d", ErrRegistryResponse, resp.StatusCode())
	}

	return string(resp.Header.Peek("ETag")), nil
}

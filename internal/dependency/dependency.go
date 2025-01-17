package dependency

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/GNURub/node-package-updater/internal/cache"
	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/utils"
	"github.com/Masterminds/semver/v3"
	"github.com/iancoleman/orderedmap"
	"github.com/valyala/fasthttp"
)

type NpmRegistryResponse struct {
	DistTags map[string]string `json:"dist-tags"`
	Versions map[string]struct {
		Dist *struct {
			Name         string `json:"name"`
			UnpackedSize uint64 `json:"unpackedSize"`
		} `json:"dist"`
	} `json:"versions"`
}

type NpmrcConfig struct {
	Registry  string
	AuthToken string
	Scope     string
}

type versionJson struct {
	Version string `json:"version"`
	Weight  uint64 `json:"weight"`
}

type Version struct {
	*semver.Version
	VersionStr string `json:"version"`
	Weight     uint64 `json:"weight"`
}

type Versions struct {
	*orderedmap.OrderedMap
}

func NewVersions() *Versions {
	orderedMap := orderedmap.New()

	v := &Versions{
		OrderedMap: orderedMap,
	}

	return v
}

func (v *Versions) SetVersions(versions []*Version) *Versions {
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].GreaterThan(versions[j].Version)
	})

	for _, version := range versions {
		v.OrderedMap.Set(version.Original(), version)
	}

	return v
}

func (v *Versions) Values() []*Version {
	values := v.OrderedMap.Keys()
	versions := make([]*Version, len(values))
	for i, key := range values {
		if value, ok := v.OrderedMap.Get(key); ok {
			versions[i] = value.(*Version)
		}
	}

	return versions
}

func (v *Versions) GetVersion(version string) *Version {
	if version, ok := v.Get(version); ok {
		return version.(*Version)
	}

	return nil
}

func (v *Versions) ListVersions() []string {
	return v.Keys()
}

func (v *Versions) Len() int {
	return len(v.OrderedMap.Keys())
}

// implement sort.Interface
func (v *Versions) Less(i, j int) bool {
	return v.Values()[i].LessThan(v.Values()[j].Version)
}

func (v *Versions) Swap(i, j int) {
	values := v.Values()
	values[i], values[j] = values[j], values[i]
}

func (versions *Versions) Save(pkgName string, cache *cache.Cache) error {
	var m = make(map[string]versionJson)

	for _, version := range versions.Values() {
		m[version.Original()] = versionJson{
			Version: version.VersionStr,
			Weight:  version.Weight,
		}
	}

	d, err := json.Marshal(m)

	if err != nil {
		return err
	}

	return cache.Set(pkgName, d)
}

func (versions *Versions) Restore(pkgName string, cache *cache.Cache) error {
	if !cache.Has(pkgName) {
		return errors.New("key not found")
	}

	d, err := cache.Get(pkgName)

	if err != nil {
		return err
	}

	var m = make(map[string]versionJson)
	err = json.Unmarshal(d, &m)

	if err != nil {
		return err
	}

	vs := make([]*Version, 0, len(m))

	for _, version := range m {
		vs = append(vs, &Version{
			Version:    semver.MustParse(version.Version),
			VersionStr: version.Version,
			Weight:     version.Weight,
		})
	}

	versions.SetVersions(vs)

	return nil
}

type Dependency struct {
	mx                *sync.Mutex
	Versions          *Versions
	PackageName       string
	CurrentVersion    *semver.Version
	CurrentVersionStr string
	LatestVersion     *semver.Version
	NextVersion       *semver.Version
	HaveToUpdate      bool
	Env               string
	Workspace         string
}

type Dependencies []*Dependency

func (d Dependencies) FilterWithNewVersion() Dependencies {
	var filtered Dependencies
	for _, dep := range d {
		if dep.NextVersion != nil {
			filtered = append(filtered, dep)
		}
	}

	return filtered
}

func (d Dependencies) FilterForUpdate() Dependencies {
	var filtered Dependencies
	for _, dep := range d {
		if dep.HaveToUpdate {
			filtered = append(filtered, dep)
		}
	}

	return filtered
}

// parseNpmrc lee y parsea el archivo .npmrc
func parseNpmrc() (*NpmrcConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("error getting home directory: %w", err)
	}

	// Buscar primero en el directorio actual
	npmrcPath := ".npmrc"
	if _, err := os.Stat(npmrcPath); os.IsNotExist(err) {
		// Si no existe, buscar en el directorio home
		npmrcPath = filepath.Join(home, ".npmrc")
	}

	content, err := os.ReadFile(npmrcPath)
	if err != nil {
		return nil, fmt.Errorf("error reading .npmrc: %w", err)
	}

	// Parsear el contenido del .npmrc
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
	version, err := semver.NewVersion(utils.GetVersionWithoutPrefix(currentVersion))
	if err != nil {
		return nil, fmt.Errorf("error parsing version: %w", err)
	}

	return &Dependency{
		PackageName:       packageName,
		CurrentVersionStr: currentVersion,
		CurrentVersion:    version,
		LatestVersion:     nil,
		NextVersion:       nil,
		Env:               env,
		HaveToUpdate:      false,
		Workspace:         workspace,
	}, nil
}

func (d *Dependency) FetchNewVersion(flags *cli.Flags, cache *cache.Cache) (err error) {
	var latest *semver.Version
	var etag string
	versions := NewVersions()
	reqToNpm := true

	versions.Restore(d.PackageName, cache)
	cachedEtag, err := cache.Get(d.PackageName + "-etag")
	if err == nil {
		etag = string(cachedEtag)
	}

	if etag != "" {
		remoteEtag, _ := headEtagFromRegistry(flags.Registry, d.PackageName)
		reqToNpm = etag != remoteEtag
	}

	if reqToNpm {
		etag, latest, versions, err = getVersionsFromRegistry(flags.Registry, d.PackageName)
		if err != nil {
			return fmt.Errorf("error fetching versions from npm registry: %w", err)
		}

		cache.Set(d.PackageName+"-etag", []byte(etag))
		versions.Save(d.PackageName, cache)
	}

	d.Versions = versions

	vm, err := NewVersionManager(d.CurrentVersionStr, versions, flags)
	if err != nil {
		return fmt.Errorf("error creating version manager: %w", err)
	}

	newVersion, err := vm.GetUpdatedVersion(flags)
	if err != nil {
		return fmt.Errorf("error getting updated version: %w", err)
	}

	if newVersion == nil {
		return fmt.Errorf("no new version found")
	}

	d.NextVersion = newVersion
	d.LatestVersion = latest

	return nil
}

func getVersionsFromRegistry(registry, packageName string) (string, *semver.Version, *Versions, error) {
	var etag string
	npmConfig, err := parseNpmrc()
	if err != nil {
		return etag, nil, nil, fmt.Errorf("error parsing .npmrc: %w", err)
	}

	isPrivate := strings.HasPrefix(packageName, npmConfig.Scope)

	registryURL := registry
	if isPrivate && npmConfig.Registry != "" {
		registryURL = npmConfig.Registry
	}

	url := fmt.Sprintf("%s/%s", registryURL, packageName)

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.SetRequestURI(url)
	req.Header.SetMethod("GET")
	req.Header.Set("Accept", "application/vnd.npm.install-v1+json; q=1.0, application/json; q=0.8, */*")
	req.Header.Set("User-Agent", "node-package-updater")

	if isPrivate && npmConfig.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+npmConfig.AuthToken)
	}

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	err = fasthttp.Do(req, resp)
	if err != nil {
		return etag, nil, nil, fmt.Errorf("error making request: %w", err)
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		return etag, nil, nil, fmt.Errorf("npm registry returned status %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	body := resp.Body()
	var npmResp NpmRegistryResponse
	if err := json.Unmarshal(body, &npmResp); err != nil {
		return etag, nil, nil, fmt.Errorf("error parsing JSON response: %w", err)
	}

	etag = string(resp.Header.Peek("ETag"))

	versions := make([]*Version, 0)
	for version, v := range npmResp.Versions {
		if v.Dist == nil {
			continue
		}

		versions = append(versions, &Version{
			Version:    semver.MustParse(version),
			VersionStr: version,
			Weight:     v.Dist.UnpackedSize,
		})
	}
	var latestVersion *semver.Version
	if latest, ok := npmResp.DistTags["latest"]; ok {
		latestVersion = semver.MustParse(latest)
	}

	return etag, latestVersion, NewVersions().SetVersions(versions), nil
}

func headEtagFromRegistry(registry, packageName string) (string, error) {
	var etag string
	npmConfig, err := parseNpmrc()
	if err != nil {
		return etag, fmt.Errorf("error parsing .npmrc: %w", err)
	}

	isPrivate := strings.HasPrefix(packageName, npmConfig.Scope)

	registryURL := registry
	if isPrivate && npmConfig.Registry != "" {
		registryURL = npmConfig.Registry
	}

	url := fmt.Sprintf("%s/%s", registryURL, packageName)

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.SetRequestURI(url)
	req.Header.SetMethod("HEAD")
	req.Header.Set("Accept", "application/vnd.npm.install-v1+json; q=1.0, application/json; q=0.8, */*")
	req.Header.Set("User-Agent", "node-package-updater")

	if isPrivate && npmConfig.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+npmConfig.AuthToken)
	}

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	err = fasthttp.Do(req, resp)
	if err != nil {
		return etag, fmt.Errorf("error making request: %w", err)
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		return etag, fmt.Errorf("npm registry returned status %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	etag = string(resp.Header.Peek("ETag"))

	return etag, nil
}

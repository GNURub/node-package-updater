package dependency

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/GNURub/node-package-updater/internal/cache"
	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/Masterminds/semver/v3"
	"github.com/iancoleman/orderedmap"
	"github.com/valyala/fasthttp"
)

type NpmRegistryResponse struct {
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

type Version struct {
	*semver.Version
	wight uint64
}

type Versions struct {
	*orderedmap.OrderedMap
}

func NewVersions(versions []*Version) *Versions {
	orderedMap := orderedmap.New()
	for _, version := range versions {
		orderedMap.Set(version.Original(), version)
	}

	return &Versions{
		OrderedMap: orderedMap,
	}
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

type Dependency struct {
	*sync.Mutex
	Versions       *Versions
	PackageName    string
	CurrentVersion string
	NextVersion    string
	HaveToUpdate   bool
	Env            string
}

type Dependencies []*Dependency

func (d Dependencies) FilterWithNewVersion() Dependencies {
	var filtered Dependencies
	for _, dep := range d {
		if dep.NextVersion != "" {
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

func NewDependency(packageName, currentVersion, env string) (*Dependency, error) {
	return &Dependency{
		PackageName:    packageName,
		CurrentVersion: currentVersion,
		NextVersion:    "",
		Env:            env,
		HaveToUpdate:   false,
	}, nil
}

func (d *Dependency) FetchNewVersion(flags *cli.Flags, cache *cache.Cache) (err error) {
	var vs []*Version
	versions := NewVersions(vs)

	// TODO: fix cache
	// if exists := cache.Has(d.PackageName); exists {
	// 	if cached, err := cache.Get(d.PackageName); err == nil {
	// 		json.Unmarshal(cached, versions)
	// 	}
	// }

	if versions.Len() == 0 {
		versions, err = getVersionsFromRegistry(flags.Registry, d.PackageName)
		if err != nil {
			return fmt.Errorf("error fetching versions from npm registry: %w", err)
		}

		data, err := json.Marshal(versions)
		if err != nil {
			return fmt.Errorf("error marshalling versions: %w", err)
		}

		cache.Set(d.PackageName, data)
	}

	d.Versions = versions

	vm, err := NewVersionManager(d.CurrentVersion, versions, flags)
	if err != nil {
		return fmt.Errorf("error creating version manager: %w", err)
	}

	newVersion, err := vm.GetUpdatedVersion(flags)
	if err != nil {
		return fmt.Errorf("error getting updated version: %w", err)
	}

	if newVersion == "" {
		return fmt.Errorf("no new version found")
	}

	d.NextVersion = newVersion

	return nil
}

func getVersionsFromRegistry(registry, packageName string) (*Versions, error) {
	npmConfig, err := parseNpmrc()
	if err != nil {
		return nil, fmt.Errorf("error parsing .npmrc: %w", err)
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
	req.Header.Set("Accept", "application/vnd.npm.install-v1+json")
	req.Header.Set("User-Agent", "node-package-updater")

	if isPrivate && npmConfig.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+npmConfig.AuthToken)
	}

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	err = fasthttp.Do(req, resp)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		return nil, fmt.Errorf("npm registry returned status %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	body := resp.Body()
	var npmResp NpmRegistryResponse
	if err := json.Unmarshal(body, &npmResp); err != nil {
		return nil, fmt.Errorf("error parsing JSON response: %w", err)
	}

	versions := make([]*Version, 0)
	for version, v := range npmResp.Versions {
		if v.Dist == nil {
			continue
		}

		versions = append(versions, &Version{
			Version: semver.MustParse(version),
			wight:   v.Dist.UnpackedSize,
		})
	}

	return NewVersions(versions), nil
}

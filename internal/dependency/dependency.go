package dependency

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/GNURub/node-package-updater/internal/cli"
)

type NpmRegistryResponse struct {
	Versions map[string]interface{} `json:"versions"`
}

type Dependency struct {
	PackageName    string
	CurrentVersion string
	NextVersion    string
	HaveToUpdate   bool
	Env            string
}

type Dependencies []*Dependency

// httpClient es un cliente HTTP reutilizable con timeout
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
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

func (d *Dependency) GetNewVersion(flags *cli.Flags) (string, error) {
	versions, err := getVersionsFromRegistry(flags.Registry, d.PackageName)
	if err != nil {
		return "", fmt.Errorf("error fetching versions from npm registry: %w", err)
	}

	vm, err := NewVersionManager(d.CurrentVersion, versions, flags)
	if err != nil {
		return "", fmt.Errorf("error creating version manager: %w", err)
	}

	newVersion, err := vm.GetUpdatedVersion(flags)

	if err != nil {
		return "", fmt.Errorf("error getting updated version: %w", err)
	}

	if newVersion == nil {
		return "", nil
	}

	return newVersion.String(), nil
}

func getVersionsFromRegistry(registry, packageName string) ([]string, error) {
	url := fmt.Sprintf("%s/%s", registry, packageName)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Agregar headers necesarios
	req.Header.Set("Accept", "application/vnd.npm.install-v1+json")
	req.Header.Set("User-Agent", "node-package-updater")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("npm registry returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	var npmResp NpmRegistryResponse
	if err := json.Unmarshal(body, &npmResp); err != nil {
		return nil, fmt.Errorf("error parsing JSON response: %w", err)
	}

	versions := make([]string, 0, len(npmResp.Versions))
	for version := range npmResp.Versions {
		versions = append(versions, version)
	}

	return versions, nil
}

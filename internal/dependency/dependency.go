package dependency

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/GNURub/node-package-updater/internal/cli"
)

type NpmRegistryResponse struct {
	Versions map[string]interface{} `json:"versions"`
}

type NpmrcConfig struct {
	Registry  string
	AuthToken string
	Scope     string
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

	return newVersion, nil
}

func getVersionsFromRegistry(registry, packageName string) ([]string, error) {
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

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.npm.install-v1+json")
	req.Header.Set("User-Agent", "node-package-updater")

	if isPrivate && npmConfig.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+npmConfig.AuthToken)
	}

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

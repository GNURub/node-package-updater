package upgrade

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/GNURub/node-package-updater/internal/semver"
	"github.com/GNURub/node-package-updater/internal/version"
)

type Release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func getLatestRelease(repoOwner, repoName string) (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned unexpected status code: %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}

	return &release, nil
}

func isNewerVersion(current, latest string) bool {
	currentVersion := semver.NewSemver(current)
	latestVersion := semver.NewSemver(latest)

	return currentVersion.IsValid() &&
		latestVersion.IsValid() &&
		currentVersion.Compare(latestVersion) < 0
}

func downloadBinary(url, destination string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status code: %d", resp.StatusCode)
	}

	dir := filepath.Dir(destination)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	outFile, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, resp.Body); err != nil {
		return fmt.Errorf("file writing failed: %w", err)
	}

	return nil
}

func replaceBinary(newBinary string) error {
	currentBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not get current executable path: %w", err)
	}

	backupBinary := currentBinary + ".bak"

	// Create a backup of the current binary
	if err := os.Rename(currentBinary, backupBinary); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	// Move new binary to current binary's location
	if err := os.Rename(newBinary, currentBinary); err != nil {
		// Restore backup if replacement fails
		if restoreErr := os.Rename(backupBinary, currentBinary); restoreErr != nil {
			log.Printf("Critical error: Could not restore backup binary: %v", restoreErr)
		}
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	os.Remove(backupBinary)

	return nil
}

func GetNewVersion(repoOwner, repoName string) string {
	latestRelease, err := getLatestRelease(repoOwner, repoName)
	if err != nil || !isNewerVersion(version.Version, latestRelease.TagName) {
		return ""
	}

	return latestRelease.TagName
}

func Upgrade(repoOwner, repoName string) error {
	fmt.Println("🔍 Checking for updates...")

	latestRelease, err := getLatestRelease(repoOwner, repoName)
	if err != nil {
		return fmt.Errorf("failed to fetch the latest release: %w", err)
	}

	if !isNewerVersion(version.Version, latestRelease.TagName) {
		fmt.Printf("✅ You already have the latest version: %s 🎉\n", version.Version)
		return nil
	}

	fmt.Printf("📦 New release available: %s\n", latestRelease.TagName)

	if len(latestRelease.Assets) == 0 {
		return fmt.Errorf("no binaries found in the release")
	}

	dir := os.TempDir()
	newBinary := filepath.Join(dir, fmt.Sprintf("npu_%s", latestRelease.TagName))
	fmt.Printf("⬇️ Downloading binary to: %s\n", newBinary)

	if err := downloadBinary(latestRelease.Assets[0].BrowserDownloadURL, newBinary); err != nil {
		return fmt.Errorf("failed to download the binary: %w", err)
	}

	if err := os.Chmod(newBinary, 0755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}

	fmt.Println("🔄 Replacing the current binary...")
	if err := replaceBinary(newBinary); err != nil {
		return fmt.Errorf("failed to replace the binary: %w", err)
	}

	fmt.Println("🎉 Upgrade completed successfully!")
	return nil
}

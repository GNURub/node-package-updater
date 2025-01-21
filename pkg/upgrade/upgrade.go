package upgrade

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/GNURub/node-package-updater/internal/version"
)

type Release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func getLatestRelease() (*Release, error) {
	url := "https://api.github.com/repos/GNURub/node-package-updater/releases/latest"
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}

	return &release, nil
}

func isNewerVersion(latest string) bool {
	return strings.Compare(version.Version, latest) < 0
}

func downloadBinary(url, destination string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	outFile, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	return err
}

func replaceBinary(newBinary string) error {
	// Obtenemos el directorio donde esta el binario actual
	// y lo reemplazamos por el nuevo binario
	currentBinary, err := os.Executable()
	if err != nil {
		return err
	}
	err = os.Rename(newBinary, currentBinary)
	if err != nil {
		return err
	}
	return nil
}

func GetNewVersion() string {
	latestRelease, err := getLatestRelease()
	if err != nil || !isNewerVersion(latestRelease.TagName) {
		return ""
	}

	return latestRelease.TagName
}

func Upgrade() error {
	fmt.Println("🔍 Checking for updates...")

	latestRelease, err := getLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to fetch the latest release: %w", err)
	}

	if !isNewerVersion(latestRelease.TagName) {
		fmt.Printf("✅ You already have the latest version: %s 🎉\n", version.Version)
		return nil
	}

	fmt.Printf("📦 New release available: %s\n", latestRelease.TagName)

	dir := os.TempDir()
	newBinary := fmt.Sprintf("%s/npu", dir)
	fmt.Printf("⬇️ Downloading binary to: %s\n", newBinary)

	if err := downloadBinary(latestRelease.Assets[0].BrowserDownloadURL, newBinary); err != nil {
		return fmt.Errorf("failed to download the binary: %w", err)
	}

	if err := os.Chmod(newBinary, 0755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}

	// Reemplazar el binario actual
	fmt.Println("🔄 Replacing the current binary...")
	if err := replaceBinary(newBinary); err != nil {
		return fmt.Errorf("failed to replace the binary: %w", err)
	}

	fmt.Println("🎉 Upgrade completed successfully!")
	return nil
}

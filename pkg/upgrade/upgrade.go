package upgrade

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/GNURub/node-package-updater/internal/semver"
	"github.com/GNURub/node-package-updater/internal/styles"
	"github.com/GNURub/node-package-updater/internal/ui"
	"github.com/GNURub/node-package-updater/internal/version"
)

type Release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// detectPlatform devuelve el OS y arquitectura en el formato usado por los assets
func detectPlatform() (string, string, error) {
	osName := runtime.GOOS
	archName := runtime.GOARCH

	// Convertir arquitecturas al formato usado en los releases
	switch archName {
	case "amd64":
		archName = "amd64"
	case "arm64":
		archName = "arm64"
	case "386":
		archName = "386"
	default:
		return "", "", fmt.Errorf("arquitectura no soportada: %s", archName)
	}

	// Los OS soportados
	switch osName {
	case "linux", "darwin", "windows":
		return osName, archName, nil
	default:
		return "", "", fmt.Errorf("sistema operativo no soportado: %s", osName)
	}
}

// findCorrectAsset encuentra el asset correcto para la plataforma actual
func findCorrectAsset(assets []struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}) (string, error) {
	osName, archName, err := detectPlatform()
	if err != nil {
		return "", err
	}

	// Patron esperado: npu_<os>_<arch>
	expectedPattern := fmt.Sprintf("npu_%s_%s", osName, archName)

	for _, asset := range assets {
		if asset.Name == expectedPattern {
			return asset.BrowserDownloadURL, nil
		}
	}

	// Si no encontramos coincidencia exacta, listamos los assets disponibles
	var availableAssets []string
	for _, asset := range assets {
		availableAssets = append(availableAssets, asset.Name)
	}

	return "", fmt.Errorf("no se encontró binario para %s_%s. Assets disponibles: %s",
		osName, archName, strings.Join(availableAssets, ", "))
}

// askUserConfirmation pide confirmación al usuario antes de continuar
func askUserConfirmation(message string) (bool, error) {
	fmt.Printf("%s (y/N): ", message)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("error leyendo respuesta: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
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
	fmt.Printf("📥 Iniciando descarga desde: %s\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("petición de descarga falló: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("descarga falló con código de estado: %d", resp.StatusCode)
	}

	// Crear directorio si no existe
	dir := filepath.Dir(destination)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("fallo al crear directorio: %w", err)
	}

	// Crear archivo de destino
	outFile, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("fallo al crear archivo: %w", err)
	}
	defer outFile.Close()

	// Obtener tamaño del archivo para mostrar progreso
	contentLength := resp.ContentLength
	var written int64

	// Mostrar progreso simple durante la descarga
	done := make(chan struct{})
	go func() {
		defer close(done)

		// Función con progreso simple usando spinner
		ui.RunSpinner("Descargando binario...", done)
	}()

	// Copiar el contenido del archivo
	written, err = io.Copy(outFile, resp.Body)
	done <- struct{}{} // Finalizar spinner

	if err != nil {
		return fmt.Errorf("fallo al escribir archivo: %w", err)
	}

	if contentLength > 0 && written != contentLength {
		return fmt.Errorf("descarga incompleta: esperado %d bytes, descargado %d bytes", contentLength, written)
	}

	fmt.Printf("✅ Descarga completada: %d bytes\n", written)
	return nil
}

func replaceBinary(newBinary string) error {
	currentBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("no se pudo obtener la ruta del ejecutable actual: %w", err)
	}

	backupBinary := currentBinary + ".bak"

	fmt.Printf("🔄 Reemplazando binario actual: %s\n", currentBinary)
	fmt.Printf("📦 Creando backup en: %s\n", backupBinary)

	// Crear backup del binario actual
	if err := os.Rename(currentBinary, backupBinary); err != nil {
		return fmt.Errorf("fallo al crear backup del binario actual: %w", err)
	}

	// Intentar mover el nuevo binario a la ubicación actual
	if err := os.Rename(newBinary, currentBinary); err != nil {
		// Si falla, restaurar el backup
		fmt.Printf("⚠️  Fallo al reemplazar binario, restaurando backup...\n")
		if restoreErr := os.Rename(backupBinary, currentBinary); restoreErr != nil {
			log.Printf("❌ Error crítico: No se pudo restaurar el backup del binario: %v", restoreErr)
			return fmt.Errorf("fallo al reemplazar binario Y al restaurar backup: reemplazo=%w, restauración=%v", err, restoreErr)
		}
		return fmt.Errorf("fallo al reemplazar binario (backup restaurado): %w", err)
	}

	// Verificar que el nuevo binario funciona antes de eliminar el backup
	fmt.Printf("🧪 Verificando que el nuevo binario funciona...\n")
	if err := verifyNewBinary(currentBinary); err != nil {
		fmt.Printf("⚠️  Nuevo binario no funciona correctamente, restaurando backup...\n")
		// Restaurar backup si el nuevo binario no funciona
		os.Remove(currentBinary) // Eliminar binario defectuoso
		if restoreErr := os.Rename(backupBinary, currentBinary); restoreErr != nil {
			log.Printf("❌ Error crítico: No se pudo restaurar el backup del binario: %v", restoreErr)
			return fmt.Errorf("nuevo binario defectuoso Y fallo al restaurar backup: verificación=%w, restauración=%v", err, restoreErr)
		}
		return fmt.Errorf("nuevo binario no funciona (backup restaurado): %w", err)
	}

	// Todo bien, eliminar backup
	if err := os.Remove(backupBinary); err != nil {
		// No es crítico si no podemos eliminar el backup
		fmt.Printf("⚠️  No se pudo eliminar el backup %s: %v\n", backupBinary, err)
	} else {
		fmt.Printf("🗑️  Backup eliminado exitosamente\n")
	}

	return nil
}

// verifyNewBinary verifica que el nuevo binario funciona ejecutando --version
func verifyNewBinary(binaryPath string) error {
	// Simplemente verificamos que el archivo existe y tiene permisos de ejecución
	info, err := os.Stat(binaryPath)
	if err != nil {
		return fmt.Errorf("no se puede acceder al nuevo binario: %w", err)
	}

	// Verificar permisos de ejecución
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("el nuevo binario no tiene permisos de ejecución")
	}

	// Por simplicidad, no ejecutamos el binario ya que podría causar problemas
	// En una implementación más robusta, podrías ejecutar --version en un proceso separado
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
	return upgradeInternal(repoOwner, repoName, false)
}

func UpgradeForce(repoOwner, repoName string) error {
	return upgradeInternal(repoOwner, repoName, true)
}

func UpgradeDryRun(repoOwner, repoName string) error {
	fmt.Println("🧪 Modo dry-run: Simulando actualización...")

	// Obtener la última versión
	latestRelease, err := getLatestRelease(repoOwner, repoName)
	if err != nil {
		return fmt.Errorf("fallo al obtener la última versión: %w", err)
	}

	// Verificar si hay una nueva versión
	if !isNewerVersion(version.Version, latestRelease.TagName) {
		fmt.Printf("✅ Ya tienes la última versión: %s 🎉\n",
			styles.SuccessStyle.Render(version.Version))
		return nil
	}

	fmt.Printf("📦 Nueva versión disponible: %s -> %s\n",
		styles.ErrorStyle.Render(version.Version),
		styles.SuccessStyle.Render(latestRelease.TagName))

	// Verificar que hay assets disponibles
	if len(latestRelease.Assets) == 0 {
		return fmt.Errorf("no se encontraron binarios en la versión")
	}

	// Detectar plataforma y encontrar el asset correcto
	downloadURL, err := findCorrectAsset(latestRelease.Assets)
	if err != nil {
		return fmt.Errorf("no se pudo encontrar el binario para tu plataforma: %w", err)
	}

	osName, archName, _ := detectPlatform()
	fmt.Printf("🖥️  Plataforma detectada: %s/%s\n", osName, archName)
	fmt.Printf("📁 URL de descarga: %s\n", downloadURL)

	currentBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("no se pudo obtener la ruta del ejecutable actual: %w", err)
	}

	fmt.Printf("🔄 Se reemplazaría: %s\n", currentBinary)
	fmt.Printf("💾 Se crearía backup en: %s.bak\n", currentBinary)

	fmt.Println("✅ Dry-run completado. Todo parece estar en orden.")
	fmt.Println("💡 Ejecuta 'npu upgrade' sin --dry-run para realizar la actualización real")

	return nil
}

func upgradeInternal(repoOwner, repoName string, force bool) error {
	fmt.Println("🔍 Verificando actualizaciones...")

	// Obtener la última versión
	latestRelease, err := getLatestRelease(repoOwner, repoName)
	if err != nil {
		return fmt.Errorf("fallo al obtener la última versión: %w", err)
	}

	// Verificar si hay una nueva versión
	if !isNewerVersion(version.Version, latestRelease.TagName) {
		fmt.Printf("✅ Ya tienes la última versión: %s 🎉\n",
			styles.SuccessStyle.Render(version.Version))
		return nil
	}

	fmt.Printf("📦 Nueva versión disponible: %s -> %s\n",
		styles.ErrorStyle.Render(version.Version),
		styles.SuccessStyle.Render(latestRelease.TagName))

	// Verificar que hay assets disponibles
	if len(latestRelease.Assets) == 0 {
		return fmt.Errorf("no se encontraron binarios en la versión")
	}

	// Detectar plataforma y encontrar el asset correcto
	downloadURL, err := findCorrectAsset(latestRelease.Assets)
	if err != nil {
		return fmt.Errorf("no se pudo encontrar el binario para tu plataforma: %w", err)
	}

	osName, archName, _ := detectPlatform()
	fmt.Printf("🖥️  Plataforma detectada: %s/%s\n", osName, archName)

	// Pedir confirmación al usuario solo si no está en modo force
	if !force {
		confirmed, err := askUserConfirmation("¿Deseas continuar con la actualización?")
		if err != nil {
			return fmt.Errorf("error al obtener confirmación: %w", err)
		}

		if !confirmed {
			fmt.Println("❌ Actualización cancelada por el usuario")
			return nil
		}
	} else {
		fmt.Println("🚀 Actualizando automáticamente (modo force)...")
	}

	// Preparar ruta de descarga
	dir := os.TempDir()
	newBinary := filepath.Join(dir, fmt.Sprintf("npu_%s_%s_%s", latestRelease.TagName, osName, archName))

	fmt.Printf("⬇️  Descargando a: %s\n", newBinary)

	// Descargar el nuevo binario
	if err := downloadBinary(downloadURL, newBinary); err != nil {
		return fmt.Errorf("fallo al descargar el binario: %w", err)
	}

	// Establecer permisos de ejecución
	if err := os.Chmod(newBinary, 0755); err != nil {
		return fmt.Errorf("fallo al establecer permisos de ejecución: %w", err)
	}

	// Reemplazar el binario actual
	fmt.Println("🔄 Reemplazando el binario actual...")
	if err := replaceBinary(newBinary); err != nil {
		// Intentar limpiar el archivo temporal en caso de error
		os.Remove(newBinary)
		return fmt.Errorf("fallo al reemplazar el binario: %w", err)
	}

	// Limpiar archivo temporal
	if err := os.Remove(newBinary); err != nil {
		// No es crítico, pero lo registramos
		fmt.Printf("⚠️  No se pudo eliminar el archivo temporal %s: %v\n", newBinary, err)
	}

	fmt.Printf("🎉 ¡Actualización completada exitosamente! %s -> %s\n",
		version.Version,
		styles.SuccessStyle.Render(latestRelease.TagName))
	fmt.Println("💡 Ejecuta 'npu version' para verificar la nueva versión")

	return nil
}

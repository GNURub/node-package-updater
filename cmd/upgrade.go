package cmd

import (
	"fmt"
	"os"

	"github.com/GNURub/node-package-updater/internal/constants"
	"github.com/GNURub/node-package-updater/internal/styles"
	"github.com/GNURub/node-package-updater/pkg/upgrade"
	"github.com/spf13/cobra"
)

var (
	upgradeForce  bool
	upgradeCheck  bool
	upgradeDryRun bool
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Actualizar a la última versión del CLI",
	Long: `Descarga e instala la última versión disponible del CLI.

El comando detectará automáticamente tu plataforma (OS/arquitectura) y descargará
el binario correcto desde GitHub Releases.

Ejemplos:
  npu upgrade                    # Actualizar con confirmación
  npu upgrade --force            # Actualizar sin confirmación
  npu upgrade --check            # Solo verificar si hay actualizaciones
  npu upgrade --dry-run          # Simular actualización sin cambios reales`,
	Run: func(cmd *cobra.Command, args []string) {
		if upgradeCheck {
			// Solo verificar si hay nueva versión
			newVersion := upgrade.GetNewVersion(constants.RepoOwner, constants.RepoName)
			if newVersion == "" {
				fmt.Printf("✅ Ya tienes la última versión disponible\n")
			} else {
				fmt.Printf("📦 Nueva versión disponible: %s\n",
					styles.SuccessStyle.Render(newVersion))
				fmt.Printf("💡 Ejecuta 'npu upgrade' para actualizar\n")
			}
			return
		}

		// Configurar el modo de upgrade basado en flags
		if upgradeDryRun {
			if err := upgrade.UpgradeDryRun(constants.RepoOwner, constants.RepoName); err != nil {
				fmt.Printf("❌ %s: %v\n",
					styles.ErrorStyle.Render("Error en dry-run"), err)
				os.Exit(1)
			}
			return
		}

		upgradeFunc := upgrade.Upgrade
		if upgradeForce {
			upgradeFunc = upgrade.UpgradeForce
		}

		if err := upgradeFunc(constants.RepoOwner, constants.RepoName); err != nil {
			fmt.Printf("❌ %s: %v\n",
				styles.ErrorStyle.Render("Actualización fallida"), err)
			os.Exit(1)
		}
	},
}

func init() {
	upgradeCmd.Flags().BoolVarP(&upgradeForce, "force", "f", false,
		"Forzar actualización sin pedir confirmación")
	upgradeCmd.Flags().BoolVarP(&upgradeCheck, "check", "c", false,
		"Solo verificar si hay actualizaciones disponibles")
	upgradeCmd.Flags().BoolVarP(&upgradeDryRun, "dry-run", "d", false,
		"Simular la actualización sin realizar cambios reales")

	rootCmd.AddCommand(upgradeCmd)
}

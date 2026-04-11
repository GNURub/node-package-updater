package cmd

import (
	"fmt"
	"os"

	"github.com/GNURub/node-package-updater/internal/packagejson"
	"github.com/GNURub/node-package-updater/internal/styles"
	"github.com/spf13/cobra"
)

var (
	installDir            string
	installPackageManager string
)

var installCmd = &cobra.Command{
	Use:     "install [path] -- [package-manager args...]",
	Short:   "Install dependencies with the correct package manager for the project",
	Aliases: []string{"i"},
	Args:    cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		baseDir, passthrough, err := parseInstallArgs(cmd, args)
		if err != nil {
			fmt.Println(styles.ErrorStyle.Render(err.Error()))
			os.Exit(1)
		}

		rootDir, pm, err := packagejson.ResolveProjectRoot(baseDir, installPackageManager)
		if err != nil {
			fmt.Println(styles.ErrorStyle.Render(err.Error()))
			os.Exit(1)
		}

		fmt.Printf("📦 Installing dependencies with %s in %s\n", pm.Name, rootDir)
		if err := pm.InstallInDir(rootDir, passthrough...); err != nil {
			fmt.Println(styles.ErrorStyle.Render(err.Error()))
			os.Exit(1)
		}
	},
}

type argsLenAtDasher interface {
	ArgsLenAtDash() int
}

func parseInstallArgs(cmd argsLenAtDasher, args []string) (string, []string, error) {
	baseDir := installDir
	passthrough := []string{}
	dashIndex := cmd.ArgsLenAtDash()

	switch {
	case dashIndex == -1:
		if len(args) > 1 {
			return "", nil, fmt.Errorf("extra package-manager arguments must be passed after `--`")
		}
		if len(args) == 1 {
			baseDir = args[0]
		}
	case dashIndex == 0:
		passthrough = args
	case dashIndex == 1:
		baseDir = args[0]
		passthrough = args[1:]
	default:
		return "", nil, fmt.Errorf("only one install path is allowed before `--`")
	}

	return baseDir, passthrough, nil
}

func init() {
	installCmd.Flags().StringVarP(&installDir, "dir", "d", "", "Project directory to install")
	installCmd.Flags().StringVarP(&installPackageManager, "packageManager", "M", "", "Package manager override")

	rootCmd.AddCommand(installCmd)
}

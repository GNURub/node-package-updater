package main

import (
	"fmt"
	"os"

	"github.com/GNURub/node-package-updater/cmd"
	"github.com/GNURub/node-package-updater/pkg/upgrade"
)

func main() {
	newVersion := upgrade.GetNewVersion()

	if newVersion != "" {
		fmt.Printf("\nNew version available: %s\nRun `npu upgrade`\n\n", newVersion)
	}

	if err := cmd.Exec(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

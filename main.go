package main

import (
	"fmt"

	"github.com/GNURub/node-package-updater/cmd/npu"
	"github.com/GNURub/node-package-updater/pkg/upgrade"
)

func main() {
	go func() {
		newVersion := upgrade.GetNewVersion()

		if newVersion != "" {
			fmt.Printf("\nNew version available: %s\nRun `npu upgrade`\n\n", newVersion)
		}
	}()

	if err := npu.Exec(); err != nil {
		fmt.Println(err)
	}
}

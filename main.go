package main

import (
	"fmt"

	"github.com/GNURub/node-package-updater/cmd/npu"
)

func main() {
	if err := npu.Exec(); err != nil {
		fmt.Println(err)
	}
}

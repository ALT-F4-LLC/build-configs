package main

import (
	"os"

	"github.com/ALT-F4-LLC/build-configs/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}

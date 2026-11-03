package main

import (
	"log"

	"github.com/ALT-F4-LLC/vorpal/sdk/go/pkg/artifact/language"
	"github.com/ALT-F4-LLC/vorpal/sdk/go/pkg/config"
)

func main() {
	ctx := config.GetContext()

	// -> 1. Activate: `source "$(vorpal build --path 'build-configs-dev')/bin/activate"`
	// -> 2. Deactivate: `deactivate`

	_, err := language.NewGoDevelopmentEnvironment("build-configs-dev", config.SYSTEMS).
		Build(ctx)
	if err != nil {
		log.Fatalf("error building development environment: %v", err)
	}

	// -> 1. Build: `vorpal build 'build-configs'`
	// -> 2. Run: `$(vorpal build --path 'build-configs')/bin/build-configs`

	_, err = language.NewGo("build-configs", config.SYSTEMS).
		WithBuildDirectory("cmd/build-configs").
		WithIncludes([]string{
			"cmd/build-configs",
			"go.mod",
			"go.sum",
			"internal/cli",
			"internal/config",
			"internal/templates",
		}).
		Build(ctx)
	if err != nil {
		log.Fatalf("error building: %v", err)
	}

	ctx.Run()
}

package main

import (
	"log"

	"github.com/ALT-F4-LLC/vorpal/sdk/go/pkg/artifact/language"
	"github.com/ALT-F4-LLC/vorpal/sdk/go/pkg/config"
)

func main() {
	ctx := config.GetContext()

	_, err := language.NewGoDevelopmentEnvironment("build-configs-shell", config.SYSTEMS).
		Build(ctx)
	if err != nil {
		log.Fatalf("error building development environment: %v", err)
	}

	_, err = language.NewGo("build-configs", config.SYSTEMS).
		WithBuildDirectory("cmd/build-configs").
		WithIncludes([]string{
			"cmd/build-configs",
			"go.mod",
			"go.sum",
			"internal",
		}).
		Build(ctx)
	if err != nil {
		log.Fatalf("error building: %v", err)
	}

	ctx.Run()
}

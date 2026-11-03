package main

import (
	"log"

	"github.com/ALT-F4-LLC/vorpal/sdk/go/pkg/artifact/language"
	"github.com/ALT-F4-LLC/vorpal/sdk/go/pkg/config"
)

func main() {
	ctx := config.GetContext()

	golangciLint, err := ctx.FetchArtifactAlias("golangci-lint:2.11.4")
	if err != nil {
		log.Fatalf("failed fetching golangci-lint alias: %v", err)
	}

	_, err = language.NewGoDevelopmentEnvironment("build-configs-dev", config.SYSTEMS).
		WithArtifacts([]*string{golangciLint}).
		Build(ctx)
	if err != nil {
		log.Fatalf("failed building development environment: %v", err)
	}

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
		log.Fatalf("failed building: %v", err)
	}

	ctx.Run() //nolint:errcheck,gosec // keeps the current exit behavior; Run's error is not surfaced today
}

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

	govulncheck, err := ctx.FetchArtifactAlias("govulncheck:1.9.0")
	if err != nil {
		log.Fatalf("failed fetching govulncheck alias: %v", err)
	}

	gitleaks, err := ctx.FetchArtifactAlias("gitleaks:8.30.1")
	if err != nil {
		log.Fatalf("failed fetching gitleaks alias: %v", err)
	}

	rumdl, err := ctx.FetchArtifactAlias("rumdl:0.3.2")
	if err != nil {
		log.Fatalf("failed fetching rumdl alias: %v", err)
	}

	_, err = language.NewGoDevelopmentEnvironment("build-configs-dev", config.SYSTEMS).
		WithArtifacts([]*string{golangciLint, govulncheck, gitleaks, rumdl}).
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

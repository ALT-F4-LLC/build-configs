package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

const GoLambdaName = "go-lambda"

type GoLambdaConfig struct {
	Config
	GoVersion      string             `json:"goVersion,omitempty" yaml:"goVersion,omitempty"`
	Lint           GolangCILintConfig `json:"lint,omitempty" yaml:"lint,omitempty"`
	Nix            NixGoConfig        `json:"nix,omitempty" yaml:"nix,omitempty"`
	Quirk          QuirkConfig        `json:"quirk,omitempty" yaml:"quirk,omitempty"`
	Deploy         []DeployConfig     `json:"deploy,omitempty" yaml:"deploy,omitempty"`
	PrivateModules string             `json:"privateModules,omitempty" yaml:"privateModules,omitempty"`
	Lambdas        []string           `json:"lambdas,omitempty" yaml:"lambdas,omitempty"`
	OpenAPI        OpenAPIConfig      `json:"openapi,omitempty" yaml:"openapi,omitempty"`
}

func NewGoLambdaConfig(c Config) GoLambdaConfig {
	return GoLambdaConfig{
		Config:         c,
		GoVersion:      "1.22",
		PrivateModules: "github.com/ALT-F4-LLC/quirk-service-kit",
		Lint:           NewGolangCiLintConfig(),
		Quirk:          NewQuirkConfig(c),
		Deploy:         []DeployConfig{},
		Lambdas:        []string{c.Name},
		OpenAPI:        NewOpenAPIConfig(),

		Nix: NixGoConfig{
			NixConfig:     NewNixConfig(),
			GoPackage:     "go",
			BuildGoModule: "buildGoModule",
		},
	}
}

func (c GoLambdaConfig) Render() error {
	renderMap := templates.RenderMap{
		templates.AllCommonTemplates: {
			".envrc",
		},
		templates.GoCommonTemplates: {
			".editorconfig",
			".github/workflows/golangci-lint.yaml",
			".golangci.yaml",
		},
		templates.GoLambdaTemplates: {
			".github/workflows/flake.yaml",
			"nix/lambda.nix",
			"nix/default.nix",
			"flake.nix",
			"justfile",
		},
	}

	if c.OpenAPI.Enable {
		renderMap[templates.GoLambdaTemplates] = append(renderMap[templates.GoLambdaTemplates], "nix/client.nix")
	}

	files, err := templates.RenderTemplates(renderMap, c)
	if err != nil {
		return err
	}

	// Create lambda entrypoints on first template run
	for _, lambda := range c.Lambdas {
		entry := fmt.Sprintf("cmd/%s/main.go", lambda)

		if _, err := os.Stat(entry); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				out, err := templates.RenderTemplate(
					templates.GoLambdaTemplates,
					"cmd/[lambda]/main.go.tmpl",
					c,
				)
				if err != nil {
					return err
				}
				files[entry] = out
			} else {
				// Error was unexpected
				return err
			}
		} else {
			// File exists
			continue
		}
	}

	return templates.WriteFiles(files)
}

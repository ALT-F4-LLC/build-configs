package config

import (
	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

type GoLambdaConfig struct {
	Config
	GoVersion      string             `json:"goVersion,omitempty" yaml:"goVersion,omitempty"`
	Lint           GolangCILintConfig `json:"lint,omitempty" yaml:"lint,omitempty"`
	Nix            NixGoConfig        `json:"nix,omitempty" yaml:"nix,omitempty"`
	Quirk          QuirkConfig        `json:"quirk,omitempty" yaml:"quirk,omitempty"`
	Deploy         DeployConfig       `json:"deploy,omitempty" yaml:"deploy,omitempty"`
	PrivateModules string             `json:"privateModules,omitempty" yaml:"privateModules,omitempty"`
	Lambdas        []string           `json:"lambdas,omitempty" yaml:"lambdas,omitempty"`
}

func NewGoLambdaConfig(c Config) GoLambdaConfig {
	return GoLambdaConfig{
		Config:    c,
		GoVersion: "1.22",
		Lint:      NewGolangCiLintConfig(),
		Quirk:     NewQuirkConfig(c),
		Deploy:    NewDeployConfig(),
		Lambdas:   []string{c.Name},

		Nix: NixGoConfig{
			NixConfig:     NewNixConfig(),
			GoPackage:     "go",
			BuildGoModule: "buildGoModule",
		},
	}
}

func (c GoLambdaConfig) Render() error {
	files, err := templates.RenderTemplates(templates.RenderMap{
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
			"flake.nix",
			"justfile",
		},
	}, c)
	if err != nil {
		return err
	}

	return templates.WriteFiles(files)
}

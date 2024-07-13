package config

import (
	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

const GoCobraCliName = "go-cobra-cli"

type GoCobraCliConfig struct {
	Config
	CgoEnabled     bool               `json:"cgoEnabled,omitempty" yaml:"cgoEnabled,omitempty"`
	GoVersion      string             `json:"goVersion,omitempty" yaml:"goVersion,omitempty"`
	Lint           GolangCILintConfig `json:"lint,omitempty" yaml:"lint,omitempty"`
	Nix            NixGoConfig        `json:"nix,omitempty" yaml:"nix,omitempty"`
	PrivateModules string             `json:"privateModules,omitempty" yaml:"privateModules,omitempty"`
	Version        string             `json:"version,omitempty" yaml:"version,omitempty"`
}

func NewGoCobraCliConfig(c Config) GoCobraCliConfig {
	return GoCobraCliConfig{
		CgoEnabled: false,
		Config:     c,
		GoVersion:  "1.22",
		Lint:       NewGolangCiLintConfig(),
		Nix: NixGoConfig{
			BuildGoModule: "buildGoModule",
			GoPackage:     "go",
			NixConfig:     NewNixConfig(),
		},
		Version: "0.1.0",
	}
}

func (c GoCobraCliConfig) Render() error {
	files, err := templates.RenderTemplates(templates.RenderMap{
		templates.AllCommonTemplates: {
			".envrc",
			".github/renovate.json",
		},
		templates.GoCommonTemplates: {
			".editorconfig",
			".github/workflows/golangci-lint.yaml",
			".golangci.yaml",
		},
		templates.GoCobraCliTemplates: {
			".github/workflows/flake.yaml",
			"flake.nix",
			"justfile",
		},
	}, c)
	if err != nil {
		return err
	}

	return templates.WriteFiles(files)
}

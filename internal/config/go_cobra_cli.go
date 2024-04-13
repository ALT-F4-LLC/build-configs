package config

import (
	"text/template"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

type GoCobraCliConfig struct {
	Config
	Nix  NixGoConfig        `json:"nix,omitempty" yaml:"nix,omitempty"`
	Lint GolangCILintConfig `json:"lint,omitempty" yaml:"lint,omitempty"`

	PrivateModules string `json:"privateModules,omitempty" yaml:"privateModules,omitempty"`
	GoVersion      string `json:"goVersion,omitempty" yaml:"goVersion,omitempty"`
}

func NewGoCobraCliConfig(c Config) GoCobraCliConfig {
	return GoCobraCliConfig{
		Config: c,

		GoVersion: "1.22",

		Lint: NewGolangCiLintConfig(),
		Nix: NixGoConfig{
			NixConfig:     NewNixConfig(),
			GoPackage:     "go",
			BuildGoModule: "buildGoModule",
		},
	}
}

func (c GoCobraCliConfig) Render() error {
	files, err := templates.RenderTemplates(map[*template.Template][]string{
		templates.AllCommonTemplates: {
			".envrc",
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

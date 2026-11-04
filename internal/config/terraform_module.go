package config

import (
	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

const TerraformModuleName = "terraform-module"

type TerraformModuleVorpalConfig struct {
	TerraformVersion string `json:"terraformVersion,omitempty" yaml:"terraformVersion,omitempty"`
}

type TerraformModuleConfig struct {
	Config
	Nix       NixConfig                   `json:"nix,omitempty" yaml:"nix,omitempty"`
	Providers []string                    `json:"providers,omitempty" yaml:"providers,omitempty"`
	Vorpal    TerraformModuleVorpalConfig `json:"vorpal,omitempty" yaml:"vorpal,omitempty"`
}

func NewTerraformModuleConfig(c Config) TerraformModuleConfig {
	return TerraformModuleConfig{
		Config: c,
		Nix:    NewNixConfig(),
		Vorpal: TerraformModuleVorpalConfig{
			TerraformVersion: "1.13.1",
		},
	}
}

func (c TerraformModuleConfig) Render() error {
	renderMap := templates.RenderMap{
		templates.AllCommonTemplates: {
			".envrc",
			".github/renovate.json",
		},
		templates.TerraformModuleTemplates: {
			".github/workflows/terraform.yaml",
			".gitignore",
			"Vorpal.toml",
			"flake.nix",
			"justfile",
		},
	}

	files, err := templates.RenderTemplates(renderMap, c)
	if err != nil {
		return err
	}

	program, err := templates.RenderTemplate(
		templates.TerraformModuleTemplates,
		"cmd/vorpal/main.go.tmpl",
		c,
	)
	if err != nil {
		return err
	}
	files["./cmd/vorpal/main.go"] = program

	return templates.WriteFiles(files)
}

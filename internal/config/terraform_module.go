package config

import (
	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

const TerraformModuleName = "terraform-module"

type TerraformModuleConfig struct {
	Config
	Nix       NixConfig `json:"nix,omitempty" yaml:"nix,omitempty"`
	Providers []string  `json:"providers,omitempty" yaml:"providers,omitempty"`
}

func NewTerraformModuleConfig(c Config) TerraformModuleConfig {
	return TerraformModuleConfig{
		Config: c,
		Nix:    NewNixConfig(),
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
			"flake.nix",
			"justfile",
		},
	}

	files, err := templates.RenderTemplates(renderMap, c)
	if err != nil {
		return err
	}

	return templates.WriteFiles(files)
}

package config

import (
	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

const TerraformModuleName = "terraform-module"

type TerraformModuleVorpalConfig struct {
	Script           string `json:"script,omitempty" yaml:"script,omitempty"`
	TerraformVersion string `json:"terraformVersion,omitempty" yaml:"terraformVersion,omitempty"`
}

type TerraformModuleConfig struct {
	Config
	Vorpal TerraformModuleVorpalConfig `json:"vorpal,omitempty" yaml:"vorpal,omitempty"`
}

func NewTerraformModuleConfig(c Config) TerraformModuleConfig {
	return TerraformModuleConfig{
		Config: c,
		Vorpal: TerraformModuleVorpalConfig{
			TerraformVersion: "1.13.1",
		},
	}
}

func (c TerraformModuleConfig) renderMap() templates.RenderMap {
	return templates.RenderMap{
		templates.AllCommonTemplates: {
			".github/renovate.json",
		},
		templates.VorpalCommonTemplates: vorpalCommonFiles(),
		templates.TerraformCommonTemplates: {
			".gitignore",
			"Vorpal.toml",
		},
		templates.TerraformModuleTemplates: {
			".github/workflows/terraform.yaml",
			"justfile",
		},
	}
}

func (c TerraformModuleConfig) Render() error {
	files, err := templates.RenderTemplates(c.renderMap(), c)
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

	if err := templates.WriteFiles(files); err != nil {
		return err
	}

	return makeVorpalShellExecutable()
}

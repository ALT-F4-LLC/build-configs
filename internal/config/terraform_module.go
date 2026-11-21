package config

import (
	"slices"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

const TerraformModuleName = "terraform-module"

// TerraformModuleVorpalConfig drives the generated Vorpal.toml and cmd/vorpal
// program. Tools adds Vorpal artifact aliases to the default list;
// ExcludeTools removes defaults by name or full alias.
type TerraformModuleVorpalConfig struct {
	Script           string   `json:"script,omitempty" yaml:"script,omitempty"`
	TerraformVersion string   `json:"terraformVersion,omitempty" yaml:"terraformVersion,omitempty"`
	Tools            []string `json:"tools,omitempty" yaml:"tools,omitempty"`
	ExcludeTools     []string `json:"excludeTools,omitempty" yaml:"excludeTools,omitempty"`
}

// DevTools returns the Vorpal artifact aliases the development environment
// fetches: the defaults and terraform at TerraformVersion, plus Tools, minus
// ExcludeTools.
func (c TerraformModuleConfig) DevTools() []string {
	defaults := append(slices.Clone(terraformModuleDefaultTools), "terraform:"+c.Vorpal.TerraformVersion)
	return resolveTools(defaults, c.Vorpal.Tools, c.Vorpal.ExcludeTools)
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

// HasDevTool reports whether the development environment carries the named
// tool at any version.
func (c TerraformModuleConfig) HasDevTool(name string) bool {
	return hasTool(c.DevTools(), name)
}

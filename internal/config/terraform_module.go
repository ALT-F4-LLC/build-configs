package config

import (
	"fmt"
	"os"
	"slices"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

const (
	TerraformModuleName = "terraform-module"

	terraformModuleVorpalShell = ".github/vorpal-shell.sh"
)

type TerraformModuleVorpalConfig struct {
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
		templates.TerraformModuleTemplates: {
			".envrc",
			terraformModuleVorpalShell,
			".github/workflows/terraform.yaml",
			".gitignore",
			"Vorpal.toml",
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

	return terraformModuleMakeVorpalShellExecutable()
}

// terraformModuleMakeVorpalShellExecutable sets the CI step shell to 0755,
// since templates.WriteFiles writes every file 0644. A path listed in
// .bcignore is left untouched, including any copy the repository keeps.
func terraformModuleMakeVorpalShellExecutable() error {
	ignored, err := templates.GetIgnoredFiles(".")
	if err != nil {
		return fmt.Errorf("failed to get ignored files: %w", err)
	}
	if slices.Contains(ignored, terraformModuleVorpalShell) {
		return nil
	}
	return os.Chmod(terraformModuleVorpalShell, 0o755) //nolint:gosec // CI runs this script directly, so it must be executable
}

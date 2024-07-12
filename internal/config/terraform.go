package config

import (
	"fmt"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

const TerraformName = "terraform"

type TerraformConfigRole struct {
	PlanARN  string `json:"planArn,omitempty" yaml:"planArn,omitempty"`
	ApplyARN string `json:"applyArn,omitempty" yaml:"applyArn,omitempty"`
}

type TerraformConfig struct {
	Config
	Nix       NixConfig           `json:"nix,omitempty" yaml:"nix,omitempty"`
	Region    string              `json:"region,omitempty" yaml:"region,omitempty"`
	Role      TerraformConfigRole `json:"role,omitempty" yaml:"role,omitempty"`
	Schedule  *string             `json:"schedule,omitempty" yaml:"schedule,omitempty"`
	Providers []string            `json:"providers,omitempty" yaml:"providers,omitempty"`
}

func NewTerraformConfigRole(name string) TerraformConfigRole {
	return TerraformConfigRole{
		ApplyARN: fmt.Sprintf("arn:aws:iam::677459762413:role/altf4llc-gha-%s-apply", name),
		PlanARN:  fmt.Sprintf("arn:aws:iam::677459762413:role/altf4llc-gha-%s-plan", name),
	}
}

func NewTerraformConfig(c Config) TerraformConfig {
	return TerraformConfig{
		Config:   c,
		Nix:      NewNixConfig(),
		Region:   "us-west-2",
		Role:     NewTerraformConfigRole(c.Name),
		Schedule: nil,
	}
}

func (c TerraformConfig) Render() error {
	renderMap := templates.RenderMap{
		templates.AllCommonTemplates: {
			".envrc",
			".github/renovate.json",
		},
		templates.TerraformTemplates: {
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

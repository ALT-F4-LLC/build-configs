package config

import (
	"fmt"
	"slices"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

const TerraformName = "terraform"

type GitHubConfig struct {
	Env    map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	RunsOn string            `json:"runsOn,omitempty" yaml:"runsOn,omitempty"`
}

type TerraformConfigRole struct {
	PlanARN  string `json:"planArn,omitempty" yaml:"planArn,omitempty"`
	ApplyARN string `json:"applyArn,omitempty" yaml:"applyArn,omitempty"`
}

type TerraformConfig struct {
	Config
	GitHub   GitHubConfig          `json:"github,omitempty" yaml:"github,omitempty"`
	Region   string                `json:"region,omitempty" yaml:"region,omitempty"`
	Role     TerraformConfigRole   `json:"role,omitempty" yaml:"role,omitempty"`
	Schedule *string               `json:"schedule,omitempty" yaml:"schedule,omitempty"`
	Vorpal   TerraformVorpalConfig `json:"vorpal,omitempty" yaml:"vorpal,omitempty"`
}

// terraformVorpalProgram is the TypeScript Vorpal config program the terraform
// and terraform-module templates render.
const terraformVorpalProgram = "src/vorpal.ts"

// TerraformVorpalConfig drives the generated Vorpal.toml and src/vorpal.ts
// program. Tools adds Vorpal artifact aliases to the default list;
// ExcludeTools removes defaults by name or full alias. The Vorpal CLI reads a
// script only for a Go config program, so there is no script option.
type TerraformVorpalConfig struct {
	TerraformVersion string   `json:"terraformVersion,omitempty" yaml:"terraformVersion,omitempty"`
	Tools            []string `json:"tools,omitempty" yaml:"tools,omitempty"`
	ExcludeTools     []string `json:"excludeTools,omitempty" yaml:"excludeTools,omitempty"`
}

// DevTools returns the Vorpal artifact aliases the development environment
// fetches: the defaults and terraform at TerraformVersion, plus Tools, minus
// ExcludeTools.
func (c TerraformConfig) DevTools() []string {
	defaults := append(slices.Clone(terraformDefaultTools), "terraform:"+c.Vorpal.TerraformVersion)
	return resolveTools(defaults, c.Vorpal.Tools, c.Vorpal.ExcludeTools)
}

func NewGitHubConfig() GitHubConfig {
	return GitHubConfig{
		Env:    map[string]string{},
		RunsOn: "altf4llc",
	}
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
		GitHub:   NewGitHubConfig(),
		Region:   "us-west-2",
		Role:     NewTerraformConfigRole(c.Name),
		Schedule: nil,
		Vorpal: TerraformVorpalConfig{
			TerraformVersion: "1.13.1",
		},
	}
}

func (c TerraformConfig) renderMap() templates.RenderMap {
	return templates.RenderMap{
		templates.AllCommonTemplates: {
			".github/renovate.json",
		},
		templates.VorpalCommonTemplates: vorpalCommonFiles(),
		templates.TerraformCommonTemplates: {
			".gitignore",
			"Vorpal.toml",
		},
		templates.TerraformTemplates: {
			".github/workflows/terraform.yaml",
			"justfile",
		},
	}
}

func (c TerraformConfig) Render() error {
	files, err := templates.RenderTemplates(c.renderMap(), c)
	if err != nil {
		return err
	}

	vorpalProgram, err := templates.RenderTemplate(templates.TerraformTemplates, terraformVorpalProgram+".tmpl", c)
	if err != nil {
		return err
	}
	files["./"+terraformVorpalProgram] = vorpalProgram

	if err := addTerraformScaffold(files, c); err != nil {
		return err
	}

	if err := templates.WriteFiles(files); err != nil {
		return err
	}

	return makeVorpalShellExecutable()
}

// HasDevTool reports whether the development environment carries the named
// tool at any version.
func (c TerraformConfig) HasDevTool(name string) bool {
	return hasTool(c.DevTools(), name)
}

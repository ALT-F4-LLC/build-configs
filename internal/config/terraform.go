package config

import (
	"fmt"
	"os"
	"slices"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

const (
	TerraformName = "terraform"

	terraformVorpalShell = ".github/vorpal-shell.sh"
)

type GitHubConfigAction struct {
	SetupDeployKey bool `json:"setupDeployKey,omitempty" yaml:"setupDeployKey,omitempty"`
}

type GitHubConfig struct {
	Action GitHubConfigAction `json:"action,omitempty" yaml:"action,omitempty"`
	Env    map[string]string  `json:"env,omitempty" yaml:"env,omitempty"`
	RunsOn string             `json:"runsOn,omitempty" yaml:"runsOn,omitempty"`
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

type TerraformVorpalConfig struct {
	Script           string `json:"script,omitempty" yaml:"script,omitempty"`
	TerraformVersion string `json:"terraformVersion,omitempty" yaml:"terraformVersion,omitempty"`
}

func NewGitHubConfigAction() GitHubConfigAction {
	return GitHubConfigAction{
		SetupDeployKey: false,
	}
}

func NewGitHubConfig() GitHubConfig {
	return GitHubConfig{
		Action: NewGitHubConfigAction(),
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
		templates.TerraformTemplates: {
			".envrc",
			terraformVorpalShell,
			".github/workflows/terraform.yaml",
			".gitignore",
			"Vorpal.toml",
			"justfile",
		},
	}
}

func (c TerraformConfig) Render() error {
	files, err := templates.RenderTemplates(c.renderMap(), c)
	if err != nil {
		return err
	}

	vorpalMain, err := templates.RenderTemplate(templates.TerraformTemplates, "cmd/vorpal/main.go.tmpl", c)
	if err != nil {
		return err
	}
	files["./cmd/vorpal/main.go"] = vorpalMain

	if err := templates.WriteFiles(files); err != nil {
		return err
	}

	return terraformMakeVorpalShellExecutable()
}

// terraformMakeVorpalShellExecutable sets the CI step shell to 0755, since
// templates.WriteFiles writes every file 0644. A path listed in .bcignore is
// left untouched, including any copy the repository keeps.
func terraformMakeVorpalShellExecutable() error {
	ignored, err := templates.GetIgnoredFiles(".")
	if err != nil {
		return fmt.Errorf("failed to get ignored files: %w", err)
	}
	if slices.Contains(ignored, terraformVorpalShell) {
		return nil
	}
	return os.Chmod(terraformVorpalShell, 0o755) //nolint:gosec // CI runs this script directly, so it must be executable
}

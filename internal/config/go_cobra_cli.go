package config

import (
	"os"
	"strings"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

var goCobraCliTemplates = []string{
	".editorconfig",
	".envrc",
	".github/workflows/flake.yaml",
	".github/workflows/golangci-lint.yaml",
	".golangci.yaml",
	"flake.nix",
	"justfile",
}

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
	out := map[string]string{}

	for _, tmplPath := range goCobraCliTemplates {
		sb := strings.Builder{}
		err := templates.GoCobraCliTemplates.ExecuteTemplate(
			&sb,
			templates.PathToTemplate(tmplPath),
			c,
		)
		if err != nil {
			return err
		}
		out["./"+tmplPath] = sb.String()
	}

	for k, v := range out {
		if err := templates.EnsureDirExists(k); err != nil {
			return err
		}

		if err := os.WriteFile(k, []byte(v), 0644); err != nil {
			return err
		}
	}

	return nil
}

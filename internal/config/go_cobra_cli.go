package config

import (
	"os"
	"strings"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

var goCobraCliTemplates = []string{
	".editorconfig",
	".github/workflows/flake.yaml",
	"flake.nix",
	"justfile",
}

type GoCobraCliConfig struct {
	Config
	Nix            NixGoConfig `json:"nix,omitempty" yaml:"nix,omitempty"`
	PrivateModules string      `json:"privateModules,omitempty" yaml:"privateModules,omitempty"`
}

func NewGoCobraCliConfig(c Config) GoCobraCliConfig {
	return GoCobraCliConfig{
		Config: c,
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

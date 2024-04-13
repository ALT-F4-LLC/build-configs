package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

var goCobraCliTemplates = []string{
	".editorconfig",
	".github/workflows/flake.yaml",
	"flake.nix",
}

type GoCobraCliConfig struct {
	Config
	Nix NixGoConfig `json:"nix,omitempty" yaml:"nix,omitempty"`
}

func NewGoCobraCliConfig(c Config) GoCobraCliConfig {
	return GoCobraCliConfig{
		Config: c,
		Nix: NixGoConfig{
			NixConfig:     NixConfig{NixpkgsBranch: "unstable"},
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
		out["./" + tmplPath] = sb.String()
	}

	for k, v := range out {
		b := []byte(v)
		if err := os.WriteFile(k, b, os.ModeAppend); err != nil {
			return err
		}
	}

	return nil
}

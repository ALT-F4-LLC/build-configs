package config

import (
	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

const GoCobraCliName = "go-cobra-cli"

type GoCobraCliConfig struct {
	Config
	CgoEnabled     bool                   `json:"cgoEnabled,omitempty" yaml:"cgoEnabled,omitempty"`
	GoVersion      string                 `json:"goVersion,omitempty" yaml:"goVersion,omitempty"`
	Lint           GolangCILintConfig     `json:"lint,omitempty" yaml:"lint,omitempty"`
	PrivateModules string                 `json:"privateModules,omitempty" yaml:"privateModules,omitempty"`
	Version        string                 `json:"version,omitempty" yaml:"version,omitempty"`
	Vorpal         GoCobraCliVorpalConfig `json:"vorpal,omitempty" yaml:"vorpal,omitempty"`
}

// GoCobraCliVorpalConfig drives the generated Vorpal.toml and cmd/vorpal
// program. Includes feeds the CLI artifact build; SourceIncludes feeds the
// Vorpal.toml source table that builds the Vorpal program itself.
type GoCobraCliVorpalConfig struct {
	BuildDirectory string   `json:"buildDirectory,omitempty" yaml:"buildDirectory,omitempty"`
	Includes       []string `json:"includes,omitempty" yaml:"includes,omitempty"`
	SourceIncludes []string `json:"sourceIncludes,omitempty" yaml:"sourceIncludes,omitempty"`
	Tools          []string `json:"tools,omitempty" yaml:"tools,omitempty"`
}

func NewGoCobraCliConfig(c Config) GoCobraCliConfig {
	lint := NewGolangCiLintConfig()
	lint.Exclude = []string{}

	return GoCobraCliConfig{
		CgoEnabled: false,
		Config:     c,
		GoVersion:  "1.22",
		Lint:       lint,
		Version:    "0.1.0",
		Vorpal: GoCobraCliVorpalConfig{
			BuildDirectory: "cmd/" + c.Name,
			Includes:       []string{"cmd/" + c.Name, "go.mod", "go.sum", "internal"},
			SourceIncludes: []string{"cmd/vorpal", "go.mod", "go.sum"},
			Tools: []string{
				"golangci-lint:2.11.4",
				"govulncheck:1.9.0",
				"gitleaks:8.30.1",
				"rumdl:0.3.2",
			},
		},
	}
}

func (c GoCobraCliConfig) renderMap() templates.RenderMap {
	return templates.RenderMap{
		templates.AllCommonTemplates: {
			".github/renovate.json",
		},
		templates.GoCommonTemplates: {
			".editorconfig",
			".golangci.yaml",
		},
		templates.VorpalCommonTemplates: vorpalCommonFiles(),
		templates.GoCobraCliTemplates: {
			".github/workflows/ci.yaml",
			"Vorpal.toml",
			"justfile",
		},
	}
}

func (c GoCobraCliConfig) Render() error {
	files, err := templates.RenderTemplates(c.renderMap(), c)
	if err != nil {
		return err
	}

	program, err := templates.RenderTemplate(templates.GoCobraCliTemplates, "cmd/vorpal/main.go.tmpl", c)
	if err != nil {
		return err
	}
	files["./cmd/vorpal/main.go"] = program

	if err := templates.WriteFiles(files); err != nil {
		return err
	}

	return makeVorpalShellExecutable()
}

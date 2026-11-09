package config

import (
	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

const GoLibraryName = "go-library"

type GoLibraryConfig struct {
	Config
	Lint   GolangCILintConfig    `json:"lint,omitempty" yaml:"lint,omitempty"`
	Vorpal GoLibraryVorpalConfig `json:"vorpal,omitempty" yaml:"vorpal,omitempty"`
}

// GoLibraryVorpalConfig drives the generated Vorpal.toml and cmd/vorpal
// program. A library produces no binary, so cmd/vorpal builds only the
// development environment. SourceIncludes feeds the Vorpal.toml source table
// that builds the Vorpal program itself. Tools replaces the default list when
// set.
type GoLibraryVorpalConfig struct {
	SourceIncludes []string `json:"sourceIncludes,omitempty" yaml:"sourceIncludes,omitempty"`
	Tools          []string `json:"tools,omitempty" yaml:"tools,omitempty"`
}

func NewGoLibraryConfig(c Config) GoLibraryConfig {
	lint := NewGolangCiLintConfig()
	lint.Exclude = []string{}

	return GoLibraryConfig{
		Config: c,
		Lint:   lint,
		Vorpal: GoLibraryVorpalConfig{
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

func (c GoLibraryConfig) renderMap() templates.RenderMap {
	return templates.RenderMap{
		templates.AllCommonTemplates: {
			".github/renovate.json",
		},
		templates.GoCommonTemplates: {
			".editorconfig",
			".golangci.yaml",
		},
		templates.VorpalCommonTemplates: vorpalCommonFiles(),
		templates.GoLibraryTemplates: {
			".github/workflows/ci.yaml",
			"Vorpal.toml",
			"justfile",
		},
	}
}

func (c GoLibraryConfig) Render() error {
	files, err := templates.RenderTemplates(c.renderMap(), c)
	if err != nil {
		return err
	}

	program, err := templates.RenderTemplate(templates.GoLibraryTemplates, "cmd/vorpal/main.go.tmpl", c)
	if err != nil {
		return err
	}
	files["./cmd/vorpal/main.go"] = program

	if err := templates.WriteFiles(files); err != nil {
		return err
	}

	return makeVorpalShellExecutable()
}

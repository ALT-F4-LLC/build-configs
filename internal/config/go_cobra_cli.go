package config

import (
	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

const GoCobraCliName = "go-cobra-cli"

type GoCobraCliConfig struct {
	Config
	Lint           GolangCILintConfig     `json:"lint,omitempty" yaml:"lint,omitempty"`
	PrivateModules string                 `json:"privateModules,omitempty" yaml:"privateModules,omitempty"`
	Vorpal         GoCobraCliVorpalConfig `json:"vorpal,omitempty" yaml:"vorpal,omitempty"`
}

// GoCobraCliVorpalConfig drives the generated Vorpal.toml and cmd/vorpal
// program. Includes feeds the CLI artifact build; SourceIncludes feeds the
// Vorpal.toml source table that builds the Vorpal program itself. Tools adds
// Vorpal artifact aliases to the default list; ExcludeTools removes defaults
// by name or full alias.
type GoCobraCliVorpalConfig struct {
	BuildDirectory string   `json:"buildDirectory,omitempty" yaml:"buildDirectory,omitempty"`
	Includes       []string `json:"includes,omitempty" yaml:"includes,omitempty"`
	SourceIncludes []string `json:"sourceIncludes,omitempty" yaml:"sourceIncludes,omitempty"`
	Tools          []string `json:"tools,omitempty" yaml:"tools,omitempty"`
	ExcludeTools   []string `json:"excludeTools,omitempty" yaml:"excludeTools,omitempty"`
}

// DevTools returns the Vorpal artifact aliases the development environment
// fetches: the defaults plus Tools, minus ExcludeTools.
func (c GoCobraCliConfig) DevTools() []string {
	return resolveTools(goDefaultTools, c.Vorpal.Tools, c.Vorpal.ExcludeTools)
}

func NewGoCobraCliConfig(c Config) GoCobraCliConfig {
	lint := NewGolangCiLintConfig()
	lint.Exclude = []string{}

	return GoCobraCliConfig{
		Config: c,
		Lint:   lint,
		Vorpal: GoCobraCliVorpalConfig{
			BuildDirectory: "cmd/" + c.Name,
			Includes:       []string{"cmd/" + c.Name, "go.mod", "go.sum", "internal"},
			SourceIncludes: []string{"cmd/vorpal", "go.mod", "go.sum"},
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

// HasDevTool reports whether the development environment carries the named
// tool at any version.
func (c GoCobraCliConfig) HasDevTool(name string) bool {
	return hasTool(c.DevTools(), name)
}

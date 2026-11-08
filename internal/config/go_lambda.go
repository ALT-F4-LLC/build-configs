package config

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

const (
	GoLambdaName = "go-lambda"

	goLambdaVorpalShell = ".github/vorpal-shell.sh"

	// goLambdaForkGuard keeps CI jobs, which run on the self-hosted runner
	// and may hold secrets, from running for pull requests opened from forks.
	goLambdaForkGuard = "github.event_name != 'pull_request' || github.event.pull_request.head.repo.full_name == github.repository"
)

type GoLambdaConfig struct {
	Config
	GoVersion      string               `json:"goVersion,omitempty" yaml:"goVersion,omitempty"`
	Lint           GolangCILintConfig   `json:"lint,omitempty" yaml:"lint,omitempty"`
	Quirk          QuirkConfig          `json:"quirk,omitempty" yaml:"quirk,omitempty"`
	Deploy         []DeployConfig       `json:"deploy,omitempty" yaml:"deploy,omitempty"`
	PrivateModules string               `json:"privateModules,omitempty" yaml:"privateModules,omitempty"`
	Lambdas        []string             `json:"lambdas,omitempty" yaml:"lambdas,omitempty"`
	OpenAPI        OpenAPIConfig        `json:"openapi,omitempty" yaml:"openapi,omitempty"`
	Vorpal         GoLambdaVorpalConfig `json:"vorpal,omitempty" yaml:"vorpal,omitempty"`
}

type GoLambdaVorpalConfig struct {
	// Tools are Vorpal artifact aliases added to the `dev` environment.
	Tools []string `json:"tools,omitempty" yaml:"tools,omitempty"`
}

func NewGoLambdaConfig(c Config) GoLambdaConfig {
	return GoLambdaConfig{
		Config:         c,
		GoVersion:      "1.22",
		PrivateModules: "github.com/ALT-F4-LLC/quirk-service-kit",
		Lint:           NewGolangCiLintConfig(),
		Quirk:          NewQuirkConfig(c),
		Deploy:         []DeployConfig{},
		Lambdas:        []string{c.Name},
		OpenAPI:        NewOpenAPIConfig(),
		Vorpal: GoLambdaVorpalConfig{
			Tools: []string{"awscli2:2.33.1", "golangci-lint:2.11.4", "jq:1.8.1", "just:1.45.0"},
		},
	}
}

func (c GoLambdaConfig) renderMap() templates.RenderMap {
	return templates.RenderMap{
		templates.AllCommonTemplates: {
			".github/renovate.json",
		},
		templates.GoCommonTemplates: {
			".editorconfig",
			".github/workflows/golangci-lint.yaml",
			".golangci.yaml",
		},
		templates.GoLambdaTemplates: {
			".envrc",
			goLambdaVorpalShell,
			".github/workflows/ci.yaml",
			"justfile",
			"Vorpal.toml",
		},
	}
}

func (c GoLambdaConfig) Render() error {
	files, err := templates.RenderTemplates(c.renderMap(), c)
	if err != nil {
		return err
	}

	vorpalProgram, err := templates.RenderTemplate(
		templates.GoLambdaTemplates,
		"cmd/vorpal/main.go.tmpl",
		c,
	)
	if err != nil {
		return err
	}
	files["./cmd/vorpal/main.go"] = vorpalProgram

	// Create lambda entrypoints on first template run
	for _, lambda := range c.Lambdas {
		entry := fmt.Sprintf("cmd/%s/main.go", lambda)

		if _, err := os.Stat(entry); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				out, err := templates.RenderTemplate(
					templates.GoLambdaTemplates,
					"cmd/[lambda]/main.go.tmpl",
					c,
				)
				if err != nil {
					return err
				}
				files[entry] = out
			} else {
				// Error was unexpected
				return err
			}
		} else {
			// File exists
			continue
		}
	}

	if err := templates.WriteFiles(files); err != nil {
		return err
	}

	return goLambdaMakeVorpalShellExecutable()
}

// ForkGuardedIf returns a GitHub Actions job `if:` expression that requires
// the fork guard and, when condition is set, condition as well. condition may
// be bare or wrapped in `${{ }}`. Both operands are parenthesised because
// `&&` binds tighter than `||` inside the guard.
func (c GoLambdaConfig) ForkGuardedIf(condition string) string {
	condition = strings.TrimSpace(condition)
	if inner, ok := strings.CutPrefix(condition, "${{"); ok {
		if inner, ok := strings.CutSuffix(inner, "}}"); ok {
			condition = strings.TrimSpace(inner)
		}
	}
	if condition == "" {
		return "${{ " + goLambdaForkGuard + " }}"
	}
	return "${{ (" + goLambdaForkGuard + ") && (" + condition + ") }}"
}

// goLambdaMakeVorpalShellExecutable sets the CI step shell to 0755, since
// templates.WriteFiles writes every file 0644. A path listed in .bcignore is
// left untouched, including any copy the repository keeps.
func goLambdaMakeVorpalShellExecutable() error {
	ignored, err := templates.GetIgnoredFiles(".")
	if err != nil {
		return fmt.Errorf("failed to get ignored files: %w", err)
	}
	if slices.Contains(ignored, goLambdaVorpalShell) {
		return nil
	}
	return os.Chmod(goLambdaVorpalShell, 0o755) //nolint:gosec // CI runs this script directly, so it must be executable
}

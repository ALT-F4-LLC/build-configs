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

	// goLambdaForkGuard keeps CI jobs, which run on the self-hosted runner
	// and may hold secrets, from running for pull requests opened from forks.
	goLambdaForkGuard = "github.event_name != 'pull_request' || github.event.pull_request.head.repo.full_name == github.repository"
)

type GoLambdaConfig struct {
	Config
	GoVersion      string             `json:"goVersion,omitempty" yaml:"goVersion,omitempty"`
	Lint           GolangCILintConfig `json:"lint,omitempty" yaml:"lint,omitempty"`
	Quirk          QuirkConfig        `json:"quirk,omitempty" yaml:"quirk,omitempty"`
	Deploy         []DeployConfig     `json:"deploy,omitempty" yaml:"deploy,omitempty"`
	PrivateModules string             `json:"privateModules,omitempty" yaml:"privateModules,omitempty"`
	Lambdas        []string           `json:"lambdas,omitempty" yaml:"lambdas,omitempty"`
	// Services are Go binaries built from cmd/<name> that are not lambdas.
	// CI only builds them: no package, upload or deploy, and no scaffolded
	// entrypoint.
	Services []string             `json:"services,omitempty" yaml:"services,omitempty"`
	OpenAPI  OpenAPIConfig        `json:"openapi,omitempty" yaml:"openapi,omitempty"`
	Vorpal   GoLambdaVorpalConfig `json:"vorpal,omitempty" yaml:"vorpal,omitempty"`
}

type GoLambdaVorpalConfig struct {
	// Extension is the full import path of the repository's own package that
	// hooks into cmd/vorpal/main.go; empty turns the hooks off. The package
	// must export `Artifacts(ctx *config.ConfigContext) error` and
	// `Go(ctx *config.ConfigContext, name string, b *language.Go) (*language.Go, error)`.
	Extension string `json:"extension,omitempty" yaml:"extension,omitempty"`
	// Script is the `script` command in Vorpal.toml; unset omits the line.
	Script string `json:"script,omitempty" yaml:"script,omitempty"`
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
			".golangci.yaml",
		},
		templates.VorpalCommonTemplates: vorpalCommonFiles(),
		templates.GoLambdaTemplates: {
			".github/workflows/ci.yaml",
			"justfile",
			"Vorpal.toml",
		},
	}
}

func (c GoLambdaConfig) Render() error {
	for _, service := range c.Services {
		if slices.Contains(c.Lambdas, service) {
			return fmt.Errorf("%q is listed in both lambdas and services; a name can be only one", service)
		}
	}

	// Deploy jobs publish lambda zips, so without lambdas they would have
	// nothing to deploy. Reject the config instead of dropping them silently.
	if len(c.Services) > 0 && len(c.Lambdas) == 0 && len(c.Deploy) > 0 {
		return errors.New("deploy is set but lambdas is empty; services are built only, so there is nothing to deploy")
	}

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

	return makeVorpalShellExecutable()
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

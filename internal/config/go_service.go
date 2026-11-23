package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

const (
	GoServiceName = "go-service"

	// goServiceDefaultRootfs is the Vorpal alias of the root filesystem the
	// image layers the services onto.
	goServiceDefaultRootfs = "linux-vorpal:latest"

	// goServicePushCondition limits the registry login and push to pushes to
	// main. The workflow runs on push only for main, so pull requests,
	// including same-repository ones, never reach the registry.
	goServicePushCondition = "github.event_name == 'push' && github.ref == 'refs/heads/main'"
)

var (
	// goServiceNamePattern bounds the names that reach shell words, justfile
	// strings, YAML scalars and Vorpal artifact names: the project name,
	// services and artifacts.
	goServiceNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

	// goServiceAliasPattern bounds a Vorpal artifact alias, such as
	// `linux-vorpal:latest` or `library/linux-vorpal:latest`.
	goServiceAliasPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*:[A-Za-z0-9][A-Za-z0-9._-]*$`)

	// goServicePrivateModulesPattern bounds the GOPRIVATE glob list, which
	// reaches Go string literals and the development shell's activate script.
	goServicePrivateModulesPattern = regexp.MustCompile(`^[A-Za-z0-9._~/*,-]+$`)

	// goServiceRepositoryPattern bounds the registry repository the image is
	// pushed to, such as `quirk/backend-discord-bot`.
	goServiceRepositoryPattern = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)*$`)
)

// GoServiceConfig renders a Go service repository: long-running binaries
// built from cmd/<name> by Vorpal, optionally packed into one OCI image that
// CI pushes to a container registry from the default branch.
type GoServiceConfig struct {
	Config
	Lint           GolangCILintConfig `json:"lint,omitempty" yaml:"lint,omitempty"`
	PrivateModules string             `json:"privateModules,omitempty" yaml:"privateModules,omitempty"`
	// Services are the Go binaries built from cmd/<name>. It defaults to
	// [<name>].
	Services []string `json:"services,omitempty" yaml:"services,omitempty"`
	// Generate lists shell lines for a `generate` justfile recipe that
	// `check` depends on, as in go-lambda.
	Generate []string `json:"generate,omitempty" yaml:"generate,omitempty"`
	// Artifacts are extra Vorpal artifact names that the CI build job builds
	// with `just package <name>`.
	Artifacts []string             `json:"artifacts,omitempty" yaml:"artifacts,omitempty"`
	Image     GoServiceImageConfig `json:"image,omitempty" yaml:"image,omitempty"`
	// Vorpal takes the same options as go-lambda's: extension, script, tools,
	// excludeTools and includes.
	Vorpal GoLambdaVorpalConfig `json:"vorpal,omitempty" yaml:"vorpal,omitempty"`
}

// GoServiceImageConfig turns on the OCI image. An empty Repository builds no
// image and pushes nothing.
type GoServiceImageConfig struct {
	// Repository is the registry repository the image is pushed to, without
	// the registry host, such as `quirk/backend-discord-bot`.
	Repository string `json:"repository,omitempty" yaml:"repository,omitempty"`
	// Rootfs is the Vorpal alias of the root filesystem under the services.
	Rootfs string `json:"rootfs,omitempty" yaml:"rootfs,omitempty"`
}

func NewGoServiceConfig(c Config) GoServiceConfig {
	return GoServiceConfig{
		Config:         c,
		PrivateModules: "github.com/ALT-F4-LLC/quirk-service-kit",
		Lint:           NewGolangCiLintConfig(),
		Services:       []string{c.Name},
		Image:          GoServiceImageConfig{Rootfs: goServiceDefaultRootfs},
	}
}

// DevTools returns the Vorpal artifact aliases the development environment
// fetches: the Go defaults plus Tools, minus ExcludeTools.
func (c GoServiceConfig) DevTools() []string {
	return resolveTools(goDefaultTools, c.Vorpal.Tools, c.Vorpal.ExcludeTools)
}

// HasDevTool reports whether the development environment carries the named
// tool at any version.
func (c GoServiceConfig) HasDevTool(name string) bool {
	return hasTool(c.DevTools(), name)
}

// ForkGuardedIf returns a job `if:` expression that requires the fork guard
// and, when condition is set, condition as well, as go-lambda's does.
func (c GoServiceConfig) ForkGuardedIf(condition string) string {
	return GoLambdaConfig{}.ForkGuardedIf(condition)
}

// ImageArtifact returns the Vorpal artifact name of the OCI image, which is
// also the image name and `latest` tag inside its tarball.
func (c GoServiceConfig) ImageArtifact() string {
	return c.Name + "-image"
}

// PushIf is the step condition for the registry login and push: pushes to
// main only, never a pull request.
func (c GoServiceConfig) PushIf() string {
	return "${{ " + goServicePushCondition + " }}"
}

// PushIfAlways is PushIf on every outcome, for the step that removes the
// registry credential from the runner even when the push fails.
func (c GoServiceConfig) PushIfAlways() string {
	return "${{ always() && " + goServicePushCondition + " }}"
}

// CheckDeps returns the recipes `just check` depends on, each preceded by a
// space, or the empty string.
func (c GoServiceConfig) CheckDeps() string {
	var deps []string
	if len(c.Generate) > 0 {
		deps = append(deps, "generate")
	}
	if len(deps) == 0 {
		return ""
	}
	return " " + strings.Join(deps, " ")
}

func (c GoServiceConfig) validate() error {
	if len(c.Services) == 0 {
		return errors.New("services is empty; a go-service project builds at least one service")
	}
	if !goServiceNamePattern.MatchString(c.Name) {
		return fmt.Errorf("name %q must be lowercase letters, digits, _ and -, starting with a letter or digit", c.Name)
	}
	if c.PrivateModules != "" && !goServicePrivateModulesPattern.MatchString(c.PrivateModules) {
		return fmt.Errorf("privateModules %q must be a comma-separated list of module path globs", c.PrivateModules)
	}

	names := map[string]string{}
	claim := func(name, what string) error {
		if prev, ok := names[name]; ok {
			return fmt.Errorf("%q is both %s and %s; Vorpal artifact names must be unique", name, prev, what)
		}
		names[name] = what
		return nil
	}
	if err := claim(c.DevEnvironment(), "the development environment"); err != nil {
		return err
	}

	for _, service := range c.Services {
		if !goServiceNamePattern.MatchString(service) {
			return fmt.Errorf("service %q must be lowercase letters, digits, _ and -, starting with a letter or digit", service)
		}
		if err := claim(service, "a service"); err != nil {
			return err
		}
	}
	for _, name := range c.Artifacts {
		if !goServiceNamePattern.MatchString(name) {
			return fmt.Errorf("artifact %q must be lowercase letters, digits, _ and -, starting with a letter or digit", name)
		}
		if err := claim(name, "an artifact"); err != nil {
			return err
		}
	}

	if c.Image.Repository != "" {
		if !goServiceRepositoryPattern.MatchString(c.Image.Repository) {
			return fmt.Errorf("image.repository %q must be a lowercase registry path such as quirk/backend-discord-bot", c.Image.Repository)
		}
		if !goServiceAliasPattern.MatchString(c.Image.Rootfs) {
			return fmt.Errorf("image.rootfs %q must be a Vorpal alias of the form <name>:<version>", c.Image.Rootfs)
		}
		if err := claim(c.ImageArtifact(), "the image"); err != nil {
			return err
		}
	}

	for _, line := range c.Generate {
		if strings.ContainsAny(line, "\r\n") {
			return fmt.Errorf("generate line %q contains a newline; list one shell line per entry", line)
		}
	}

	return nil
}

func (c GoServiceConfig) renderMap() templates.RenderMap {
	return templates.RenderMap{
		templates.AllCommonTemplates: {
			".github/renovate.json",
		},
		templates.GoCommonTemplates: {
			".editorconfig",
			".golangci.yaml",
		},
		templates.VorpalCommonTemplates: vorpalCommonFiles(),
		templates.GoServiceTemplates: {
			".github/workflows/ci.yaml",
			"justfile",
			"Vorpal.toml",
		},
	}
}

func (c GoServiceConfig) Render() error {
	if err := c.validate(); err != nil {
		return err
	}

	files, err := templates.RenderTemplates(c.renderMap(), c)
	if err != nil {
		return err
	}

	vorpalProgram, err := templates.RenderTemplate(templates.GoServiceTemplates, "cmd/vorpal/main.go.tmpl", c)
	if err != nil {
		return err
	}
	files["./cmd/vorpal/main.go"] = vorpalProgram

	if err := templates.WriteFiles(files); err != nil {
		return err
	}

	return makeVorpalShellExecutable()
}

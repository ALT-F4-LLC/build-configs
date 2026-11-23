package config

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
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
	// services, artifacts and clients.
	goServiceNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

	// goServiceAliasPattern bounds a Vorpal artifact alias, such as
	// `quirk-lambda-ai-client-go:0.1.0` or `library/linux-vorpal:latest`.
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
	Artifacts []string `json:"artifacts,omitempty" yaml:"artifacts,omitempty"`
	// Clients are Vorpal artifacts, such as generated API clients, copied
	// into the source tree before every build.
	Clients []GoServiceClientConfig `json:"clients,omitempty" yaml:"clients,omitempty"`
	Image   GoServiceImageConfig    `json:"image,omitempty" yaml:"image,omitempty"`
	// Vorpal takes the same options as go-lambda's: extension, script, tools,
	// excludeTools and includes.
	Vorpal GoLambdaVorpalConfig `json:"vorpal,omitempty" yaml:"vorpal,omitempty"`
}

// GoServiceClientConfig names one Vorpal artifact that the service source
// needs but the repository does not commit.
type GoServiceClientConfig struct {
	// Name identifies the client; the wrapper artifact is
	// <project>-client-<name>.
	Name string `json:"name" yaml:"name"`
	// Alias is the Vorpal artifact alias whose output holds the client files.
	Alias string `json:"alias" yaml:"alias"`
	// Output is the directory, relative to the repository root, the files are
	// copied into. `just clients` deletes it first.
	Output string `json:"output" yaml:"output"`
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

// ClientArtifact returns the Vorpal artifact name that wraps a client alias,
// so `vorpal build --path` can fetch it.
func (c GoServiceConfig) ClientArtifact(client GoServiceClientConfig) string {
	return c.Name + "-client-" + client.Name
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
	if len(c.Clients) > 0 {
		deps = append(deps, "clients")
	}
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

	// `just clients` runs rm -rf on each output, so no output may hold or sit
	// inside a path the build needs, or overlap another output.
	reserved := []string{"cmd", "dist", ".github", "go.mod", "go.sum", "justfile", "Vorpal.toml", "Vorpal.lock", "build-configs.yaml"}
	var outputs []string
	for _, client := range c.Clients {
		if !goServiceNamePattern.MatchString(client.Name) {
			return fmt.Errorf("client name %q must be lowercase letters, digits, _ and -, starting with a letter or digit", client.Name)
		}
		if !goServiceAliasPattern.MatchString(client.Alias) {
			return fmt.Errorf("client %q alias %q must be a Vorpal alias of the form <name>:<version>", client.Name, client.Alias)
		}
		if !openAPIOutputPattern.MatchString(client.Output) || slices.Contains(strings.Split(client.Output, "/"), "..") ||
			slices.Contains(strings.Split(client.Output, "/"), ".") {
			return fmt.Errorf("client %q output %q must be a relative path of letters, digits, _, ., / and -, with no . or .. segment", client.Name, client.Output)
		}
		if strings.HasSuffix(client.Output, "/") || strings.Contains(client.Output, "//") {
			return fmt.Errorf("client %q output %q must have no trailing slash or empty segment", client.Name, client.Output)
		}
		// Compare case-insensitively: on a case-insensitive filesystem,
		// rm -rf 'Cmd' deletes cmd.
		output := strings.ToLower(client.Output)
		for _, path := range reserved {
			if goServicePathsOverlap(output, strings.ToLower(path)) {
				return fmt.Errorf("client %q output %q overlaps %s, which `just clients` would delete", client.Name, client.Output, path)
			}
		}
		for _, other := range outputs {
			if goServicePathsOverlap(output, other) {
				return fmt.Errorf("client output %q overlaps client output %q", client.Output, other)
			}
		}
		outputs = append(outputs, output)
		if err := claim(c.ClientArtifact(client), "a client"); err != nil {
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

// goServicePathsOverlap reports whether slash-separated relative paths a and b
// are equal or one lies inside the other.
func goServicePathsOverlap(a, b string) bool {
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
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

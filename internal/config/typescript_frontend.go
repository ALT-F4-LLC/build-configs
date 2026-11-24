package config

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

const TypeScriptFrontendName = "typescript-frontend"

// typeScriptFrontendVorpalProgram is the TypeScript Vorpal config program,
// the entrypoint Vorpal.toml names.
const typeScriptFrontendVorpalProgram = "src/vorpal.ts"

var (
	// typeScriptFrontendScriptPattern bounds a package.json script name, which
	// reaches justfile recipe bodies and the Vorpal program. It admits the
	// `build:tsc` style that npm and bun accept.
	typeScriptFrontendScriptPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9:._-]*$`)

	// typeScriptFrontendOutputPattern bounds the build output directory, a
	// relative path that reaches shell words in the Vorpal build step.
	typeScriptFrontendOutputPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*(/[A-Za-z0-9][A-Za-z0-9._-]*)*$`)

	// typeScriptFrontendDefaultTools are the Vorpal aliases the development
	// environment fetches. Bun comes from the SDK's TypeScript development
	// environment, so it is not listed and cannot be excluded.
	typeScriptFrontendDefaultTools = []string{
		"gitleaks:8.30.1",
		"jq:1.8.1",
		"just:1.45.0",
		"rumdl:0.3.2",
	}

	// typeScriptFrontendDefaultSourceIncludes is the frontend source when
	// vorpal.includes is unset. Vorpal ignores .gitignore, uploads the source
	// to the registry and keeps a path only when an include equals it or one
	// of its leading directories, so the list names each file a Vite build
	// reads, in each common extension, while a local .env stays out. An entry
	// that matches nothing keeps nothing.
	typeScriptFrontendDefaultSourceIncludes = []string{
		"bun.lock",
		"bunfig.toml",
		"index.html",
		"package.json",
		"postcss.config.cjs",
		"postcss.config.js",
		"postcss.config.mjs",
		"postcss.config.ts",
		"public",
		"src",
		"tailwind.config.cjs",
		"tailwind.config.js",
		"tailwind.config.mjs",
		"tailwind.config.ts",
		"tsconfig.app.json",
		"tsconfig.json",
		"tsconfig.node.json",
		"vite.config.js",
		"vite.config.mjs",
		"vite.config.mts",
		"vite.config.ts",
	}

	// typeScriptFrontendAuditLevels are the severities `bun audit
	// --audit-level` accepts.
	typeScriptFrontendAuditLevels = []string{"low", "moderate", "high", "critical"}
)

// TypeScriptFrontendConfig renders a static TypeScript frontend repository:
// a bun project whose build script writes static files that Vorpal packages
// as one artifact. It has no deploy job.
type TypeScriptFrontendConfig struct {
	Config
	// Scripts names the package.json scripts the justfile runs.
	Scripts TypeScriptFrontendScripts `json:"scripts,omitempty" yaml:"scripts,omitempty"`
	// Output is the directory the build script writes, relative to the
	// repository root. It defaults to dist.
	Output string `json:"output,omitempty" yaml:"output,omitempty"`
	// Generate lists shell lines for a `generate` justfile recipe that `check`
	// depends on, as in go-service.
	Generate []string `json:"generate,omitempty" yaml:"generate,omitempty"`
	// Audit tunes the `bun audit` run by `vuln-scan`.
	Audit  TypeScriptFrontendAuditConfig  `json:"audit,omitempty" yaml:"audit,omitempty"`
	Vorpal TypeScriptFrontendVorpalConfig `json:"vorpal,omitempty" yaml:"vorpal,omitempty"`
}

// TypeScriptFrontendAuditConfig maps to `bun audit` flags. Unset, every
// advisory in every locked dependency fails the scan.
type TypeScriptFrontendAuditConfig struct {
	// Level is the lowest severity that fails the scan: low, moderate, high or
	// critical.
	Level string `json:"level,omitempty" yaml:"level,omitempty"`
	// Production skips packages only devDependencies need.
	Production bool `json:"production,omitempty" yaml:"production,omitempty"`
}

// AuditArgs returns the `bun audit` flags, each preceded by a space, or the
// empty string.
func (c TypeScriptFrontendConfig) AuditArgs() string {
	var args []string
	if c.Audit.Level != "" {
		args = append(args, "--audit-level="+c.Audit.Level)
	}
	if c.Audit.Production {
		args = append(args, "--production")
	}
	if len(args) == 0 {
		return ""
	}
	return " " + strings.Join(args, " ")
}

// TypeScriptFrontendVorpalConfig takes go-lambda's vorpal options except
// extension, whose hook contract builds Go artifacts, and script, which the
// Vorpal CLI reads only for a Go config program.
type TypeScriptFrontendVorpalConfig struct {
	// Tools are Vorpal artifact aliases added to the default development
	// environment tools.
	Tools []string `json:"tools,omitempty" yaml:"tools,omitempty"`
	// ExcludeTools removes default tools by name (`jq`) or full alias
	// (`jq:1.8.1`).
	ExcludeTools []string `json:"excludeTools,omitempty" yaml:"excludeTools,omitempty"`
	// Includes replaces the frontend source allowlist with these paths,
	// relative to the project root. The output directory, node_modules and
	// the config program are excluded either way.
	Includes []string `json:"includes,omitempty" yaml:"includes,omitempty"`
}

// TypeScriptFrontendScripts names package.json scripts. An empty Test renders
// no test recipe and no CI test step.
type TypeScriptFrontendScripts struct {
	Build     string `json:"build,omitempty" yaml:"build,omitempty"`
	Lint      string `json:"lint,omitempty" yaml:"lint,omitempty"`
	Test      string `json:"test,omitempty" yaml:"test,omitempty"`
	Typecheck string `json:"typecheck,omitempty" yaml:"typecheck,omitempty"`
}

func NewTypeScriptFrontendConfig(c Config) TypeScriptFrontendConfig {
	return TypeScriptFrontendConfig{
		Config: c,
		Scripts: TypeScriptFrontendScripts{
			Build:     "build",
			Lint:      "lint",
			Typecheck: "typecheck",
		},
		Output: "dist",
	}
}

// DevTools returns the Vorpal artifact aliases the development environment
// fetches: the defaults plus Tools, minus ExcludeTools.
func (c TypeScriptFrontendConfig) DevTools() []string {
	return resolveTools(typeScriptFrontendDefaultTools, c.Vorpal.Tools, c.Vorpal.ExcludeTools)
}

// HasDevTool reports whether the development environment carries the named
// tool at any version.
func (c TypeScriptFrontendConfig) HasDevTool(name string) bool {
	return hasTool(c.DevTools(), name)
}

// ForkGuardedIf returns a job `if:` expression that requires the fork guard
// and, when condition is set, condition as well, as go-lambda's does.
func (c TypeScriptFrontendConfig) ForkGuardedIf(condition string) string {
	return GoLambdaConfig{}.ForkGuardedIf(condition)
}

// SourceIncludes returns the paths the frontend source keeps:
// vorpal.includes when set, else typeScriptFrontendDefaultSourceIncludes.
func (c TypeScriptFrontendConfig) SourceIncludes() []string {
	if len(c.Vorpal.Includes) > 0 {
		return c.Vorpal.Includes
	}
	return typeScriptFrontendDefaultSourceIncludes
}

// SourceExcludes returns the paths no frontend source keeps: the build
// output, so a local build never changes the artifact digest; node_modules,
// which the build step installs from bun.lock; and the Vorpal config
// program, which the frontend build never reads.
func (c TypeScriptFrontendConfig) SourceExcludes() []string {
	excludes := []string{c.Output}
	for _, p := range []string{"node_modules", typeScriptFrontendVorpalProgram} {
		if p != c.Output {
			excludes = append(excludes, p)
		}
	}
	return excludes
}

// CheckDeps returns the recipes `just check` depends on, each preceded by a
// space.
func (c TypeScriptFrontendConfig) CheckDeps() string {
	deps := []string{"install"}
	if len(c.Generate) > 0 {
		deps = append(deps, "generate")
	}
	return " " + strings.Join(deps, " ")
}

func (c TypeScriptFrontendConfig) validate() error {
	if !goServiceNamePattern.MatchString(c.Name) {
		return fmt.Errorf("name %q must be lowercase letters, digits, _ and -, starting with a letter or digit", c.Name)
	}

	for _, s := range []struct {
		field, script string
		optional      bool
	}{
		{"scripts.build", c.Scripts.Build, false},
		{"scripts.lint", c.Scripts.Lint, false},
		{"scripts.typecheck", c.Scripts.Typecheck, false},
		{"scripts.test", c.Scripts.Test, true},
	} {
		if s.optional && s.script == "" {
			continue
		}
		if !typeScriptFrontendScriptPattern.MatchString(s.script) {
			return fmt.Errorf("%s %q must be a package.json script name: letters, digits, :, ., _ and -", s.field, s.script)
		}
	}

	if c.Audit.Level != "" && !slices.Contains(typeScriptFrontendAuditLevels, c.Audit.Level) {
		return fmt.Errorf("audit.level %q must be one of %s", c.Audit.Level, strings.Join(typeScriptFrontendAuditLevels, ", "))
	}

	if !typeScriptFrontendOutputPattern.MatchString(c.Output) || slices.Contains(strings.Split(c.Output, "/"), "..") {
		return fmt.Errorf("output %q must be a relative directory such as dist, with no leading ./ or /, and no .. segment", c.Output)
	}

	// Vorpal matches includes as path prefixes from the root: an empty entry
	// keeps every path and a ./ or / prefix keeps none.
	for _, include := range c.Vorpal.Includes {
		if include == "" || strings.HasPrefix(include, "/") || strings.HasPrefix(include, "./") || slices.Contains(strings.Split(include, "/"), "..") {
			return fmt.Errorf("vorpal.includes entry %q must be a non-empty relative path with no leading ./ or /, and no .. segment", include)
		}
	}

	for _, line := range c.Generate {
		if strings.ContainsAny(line, "\r\n") {
			return fmt.Errorf("generate line %q contains a newline; list one shell line per entry", line)
		}
	}

	return nil
}

func (c TypeScriptFrontendConfig) renderMap() templates.RenderMap {
	return templates.RenderMap{
		templates.AllCommonTemplates: {
			".github/renovate.json",
		},
		templates.VorpalCommonTemplates: vorpalCommonFiles(),
		templates.TypeScriptFrontendTemplates: {
			".github/workflows/ci.yaml",
			"justfile",
			"Vorpal.toml",
		},
	}
}

func (c TypeScriptFrontendConfig) Render() error {
	if err := c.validate(); err != nil {
		return err
	}

	files, err := templates.RenderTemplates(c.renderMap(), c)
	if err != nil {
		return err
	}

	vorpalProgram, err := templates.RenderTemplate(templates.TypeScriptFrontendTemplates, typeScriptFrontendVorpalProgram+".tmpl", c)
	if err != nil {
		return err
	}
	files["./"+typeScriptFrontendVorpalProgram] = vorpalProgram

	if err := templates.WriteFiles(files); err != nil {
		return err
	}

	return makeVorpalShellExecutable()
}

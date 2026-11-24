package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// typeScriptFrontendPortalGenerate is quirk-frontend-portal's client
// generation: openapi-generator-cli's typescript-axios generator over the
// tracked src/api/portal.json, into the gitignored src/client.
const typeScriptFrontendPortalGenerate = "openapi-generator-cli generate --input-spec src/api/portal.json --generator-name typescript-axios --output src/client"

// typeScriptFrontendPortalConfig describes quirk-frontend-portal: a static
// Vite SPA on bun whose typecheck script is `build:tsc`, which has no test
// script, whose Vite envDir is env/, and whose API client is generated with
// openapi-generator-cli, a Java tool.
// typeScriptFrontendPortalIncludes are the paths quirk-frontend-portal's Vite
// build reads, by exact name.
var typeScriptFrontendPortalIncludes = []string{"bun.lock", "env", "index.html", "package.json", "postcss.config.js", "src", "tailwind.config.js", "tsconfig.json", "tsconfig.node.json", "vite.config.ts"}

func typeScriptFrontendPortalConfig() TypeScriptFrontendConfig {
	cfg := NewTypeScriptFrontendConfig(Config{Name: "quirk-frontend-portal", Template: TypeScriptFrontendName})
	cfg.Scripts.Build = "build:prod"
	cfg.Scripts.Typecheck = "build:tsc"
	cfg.Generate = []string{typeScriptFrontendPortalGenerate}
	cfg.Vorpal.Tools = []string{"openapi-generator-cli:7.18.0", "openjdk:25.0.1"}
	cfg.Vorpal.Includes = typeScriptFrontendPortalIncludes
	return cfg
}

// typeScriptFrontendRender renders cfg into a fresh working directory.
func typeScriptFrontendRender(t *testing.T, cfg TypeScriptFrontendConfig) {
	t.Helper()
	t.Chdir(t.TempDir())
	if err := cfg.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

// The typescript-frontend snapshots pin every rendered file: the defaults,
// and the shape of quirk-frontend-portal.
func TestTypeScriptFrontendSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name   string
		golden string
		config func() TypeScriptFrontendConfig
	}{
		{
			name:   "default",
			golden: "typescript-frontend-default.golden",
			config: func() TypeScriptFrontendConfig {
				return NewTypeScriptFrontendConfig(Config{Name: "example", Template: TypeScriptFrontendName})
			},
		},
		{
			name:   "quirk-frontend-portal",
			golden: "typescript-frontend-portal.golden",
			config: typeScriptFrontendPortalConfig,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			golden, err := filepath.Abs(filepath.Join("testdata", tc.golden))
			if err != nil {
				t.Fatalf("resolve golden path: %v", err)
			}

			root := t.TempDir()
			out := filepath.Join(root, "out")
			if err := os.Mkdir(out, 0o750); err != nil {
				t.Fatalf("create render dir: %v", err)
			}
			t.Chdir(out)

			if err := tc.config().Render(); err != nil {
				t.Fatalf("Render: %v", err)
			}

			terraformModuleCompareGolden(t, golden, terraformModuleSnapshot(t, root))
		})
	}
}

// The example configuration loads through New and GetTemplater and renders
// every file.
func TestTypeScriptFrontendExampleRenders(t *testing.T) {
	example, err := filepath.Abs(filepath.Join("..", "..", "examples", TypeScriptFrontendName, "build-configs.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(example) //nolint:gosec // reads an example from this repository
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	t.Chdir(t.TempDir())
	if err := os.WriteFile("build-configs.yaml", src, 0o600); err != nil { //nolint:gosec // writes the example into the test's own temp dir
		t.Fatal(err)
	}
	cfg, err := New("build-configs.yaml")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tpl, err := cfg.GetTemplater()
	if err != nil {
		t.Fatalf("GetTemplater: %v", err)
	}
	fe, ok := tpl.(TypeScriptFrontendConfig)
	if !ok {
		t.Fatalf("templater is %T, want TypeScriptFrontendConfig", tpl)
	}
	if fe.Scripts.Build != "build:prod" || fe.Scripts.Lint != "lint" || fe.Scripts.Typecheck != "build:tsc" || fe.Scripts.Test != "" {
		t.Errorf("scripts = %+v, want build:prod, typecheck build:tsc and no test", fe.Scripts)
	}
	if fe.Output != "dist" {
		t.Errorf("output = %q, want the dist default", fe.Output)
	}
	if want := typeScriptFrontendPortalConfig(); !slices.Equal(fe.SourceIncludes(), want.SourceIncludes()) ||
		!slices.Equal(fe.Generate, want.Generate) || !slices.Equal(fe.DevTools(), want.DevTools()) {
		t.Errorf("example includes, generate, tools = %q, %q, %q; want the portal config's %q, %q, %q",
			fe.SourceIncludes(), fe.Generate, fe.DevTools(), want.SourceIncludes(), want.Generate, want.DevTools())
	}
	if err := tpl.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, file := range []string{
		".envrc", ".github/renovate.json", ".github/vorpal-shell.sh",
		".github/workflows/ci.yaml", "Vorpal.toml", "justfile", typeScriptFrontendVorpalProgram,
	} {
		if _, err := os.Stat(file); err != nil {
			t.Errorf("%s was not rendered: %v", file, err)
		}
	}
	for _, absent := range []string{".editorconfig", ".golangci.yaml"} {
		if _, err := os.Stat(absent); err == nil {
			t.Errorf("%s rendered; it belongs to the Go templates", absent)
		}
	}
	info, err := os.Stat(vorpalShell)
	if err != nil {
		t.Fatalf("stat %s: %v", vorpalShell, err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Errorf("%s mode = %#o, want %#o", vorpalShell, got, 0o755)
	}
}

// Render rejects a config whose names would break the generated shell, Go
// or YAML, and writes nothing.
func TestTypeScriptFrontendRenderRejectsInvalidConfig(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*TypeScriptFrontendConfig)
		want   string
	}{
		{"uppercase name", func(c *TypeScriptFrontendConfig) { c.Name = "Portal" }, "name"},
		{"name with quote", func(c *TypeScriptFrontendConfig) { c.Name = "p'; rm -rf /" }, "name"},
		{"build script with space", func(c *TypeScriptFrontendConfig) { c.Scripts.Build = "build && curl x" }, "scripts.build"},
		{"empty lint script", func(c *TypeScriptFrontendConfig) { c.Scripts.Lint = "" }, "scripts.lint"},
		{"typecheck with backtick", func(c *TypeScriptFrontendConfig) { c.Scripts.Typecheck = "tsc`id`" }, "scripts.typecheck"},
		{"test with semicolon", func(c *TypeScriptFrontendConfig) { c.Scripts.Test = "test;id" }, "scripts.test"},
		{"absolute output", func(c *TypeScriptFrontendConfig) { c.Output = "/tmp/out" }, "output"},
		{"dotted output", func(c *TypeScriptFrontendConfig) { c.Output = "./dist" }, "output"},
		{"parent output", func(c *TypeScriptFrontendConfig) { c.Output = "dist/../.." }, "output"},
		{"output with quote", func(c *TypeScriptFrontendConfig) { c.Output = `dist"` }, "output"},
		{"empty include", func(c *TypeScriptFrontendConfig) { c.Vorpal.Includes = []string{"src", ""} }, "vorpal.includes"},
		{"dotted include", func(c *TypeScriptFrontendConfig) { c.Vorpal.Includes = []string{"./src"} }, "vorpal.includes"},
		{"absolute include", func(c *TypeScriptFrontendConfig) { c.Vorpal.Includes = []string{"/etc"} }, "vorpal.includes"},
		{"parent include", func(c *TypeScriptFrontendConfig) { c.Vorpal.Includes = []string{"src/../.."} }, "vorpal.includes"},
		{"empty build script", func(c *TypeScriptFrontendConfig) { c.Scripts.Build = "" }, "scripts.build"},
		{"build script flag", func(c *TypeScriptFrontendConfig) { c.Scripts.Build = "-e" }, "scripts.build"},
		{"multi-line generate", func(c *TypeScriptFrontendConfig) { c.Generate = []string{"a\nb"} }, "generate"},
		{"unknown audit level", func(c *TypeScriptFrontendConfig) { c.Audit.Level = "severe" }, "audit.level"},
		{"audit level injection", func(c *TypeScriptFrontendConfig) { c.Audit.Level = "high; id" }, "audit.level"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewTypeScriptFrontendConfig(Config{Name: "demo", Template: TypeScriptFrontendName})
			tc.mutate(&cfg)
			t.Chdir(t.TempDir())
			err := cfg.Render()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Render = %v, want an error naming %q", err, tc.want)
			}
			if _, statErr := os.Stat("justfile"); statErr == nil {
				t.Errorf("Render wrote files before rejecting the config")
			}
		})
	}
}

// The Vorpal program builds the TypeScript development environment, bun, and
// one artifact that installs from bun.lock, runs the build script under bun,
// and copies the output directory.
func TestTypeScriptFrontendVorpalProgramBuildsFrontend(t *testing.T) {
	cfg := typeScriptFrontendPortalConfig()
	cfg.Output = "out"
	typeScriptFrontendRender(t, cfg)
	src := goLambdaReadFile(t, typeScriptFrontendVorpalProgram)

	if got, want := typeScriptFrontendArray(t, src, "tools"), []string{"gitleaks:8.30.1", "jq:1.8.1", "just:1.45.0", "rumdl:0.3.2", "openapi-generator-cli:7.18.0", "openjdk:25.0.1"}; !slices.Equal(got, want) {
		t.Errorf("tools = %q, want %q", got, want)
	}
	for _, want := range []string{
		`from "@altf4llc/vorpal-sdk";`,
		`const frontend = "quirk-frontend-portal";`,
		`const devEnvironment = "quirk-frontend-portal-dev";`,
		"new TypeScriptDevelopmentEnvironment(devEnvironment, SYSTEMS)",
		`const bunVersion = "1.4.4";`,
		"withAliases([`bun:${bunVersion}`])",
		"await shell(context, [bun], [], buildScript, [])",
		`"pushd \"./source/quirk-frontend-portal\"",`,
		`"bun install --frozen-lockfile",`,
		`"bun --bun run build:prod",`,
		`"test -d \"out\"",`,
		`"cp -R \"out/.\" \"$VORPAL_OUTPUT/\"",`,
		"await context.run();",
		"void main();",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("%s does not contain %q", typeScriptFrontendVorpalProgram, want)
		}
	}
	for _, absent := range []string{"<no value>", "OciImage", "withSecrets", "process.env", "require("} {
		if strings.Contains(src, absent) {
			t.Errorf("%s contains %q", typeScriptFrontendVorpalProgram, absent)
		}
	}
}

// Vorpal.toml names the TypeScript program and builds it from the lockfile,
// package.json and tsconfig.json alone.
func TestTypeScriptFrontendVorpalTomlUsesTypeScript(t *testing.T) {
	typeScriptFrontendRender(t, typeScriptFrontendPortalConfig())
	got := goLambdaReadFile(t, "Vorpal.toml")
	want := hashNoticeLines + `language = "typescript"

[source]
includes = [
    "bun.lock",
    "bunfig.toml",
    "package.json",
    "src/vorpal.ts",
    "tsconfig.json"
]

[source.typescript]
entrypoint = "src/vorpal.ts"
`
	if got != want {
		t.Errorf("Vorpal.toml =\n%s\nwant\n%s", got, want)
	}
	if _, err := os.Stat("cmd/vorpal/main.go"); err == nil {
		t.Error("cmd/vorpal/main.go rendered; the config program is TypeScript")
	}
}

// Every string the config reaches the program through is a JSON string
// literal, so a quote or backslash in a tool alias or include cannot end it.
func TestTypeScriptFrontendVorpalProgramQuotesValues(t *testing.T) {
	for name, mutate := range map[string]func(*TypeScriptFrontendConfig){
		"default": func(*TypeScriptFrontendConfig) {},
		"no tools": func(c *TypeScriptFrontendConfig) {
			c.Vorpal.ExcludeTools = []string{"gitleaks", "jq", "just", "rumdl"}
		},
		"odd values": func(c *TypeScriptFrontendConfig) {
			c.Vorpal.Tools = []string{`a"b:1`, `c\d:2`, "e`${f}`:3"}
			c.Vorpal.Includes = []string{`src/"x"`, `back\slash`}
		},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := NewTypeScriptFrontendConfig(Config{Name: "demo", Template: TypeScriptFrontendName})
			mutate(&cfg)
			typeScriptFrontendRender(t, cfg)
			src := goLambdaReadFile(t, typeScriptFrontendVorpalProgram)
			if got := typeScriptFrontendArray(t, src, "tools"); !slices.Equal(got, cfg.DevTools()) {
				t.Errorf("tools = %q, want %q", got, cfg.DevTools())
			}
			if got := typeScriptFrontendArray(t, src, "sourceIncludes"); !slices.Equal(got, cfg.SourceIncludes()) {
				t.Errorf("sourceIncludes = %q, want %q", got, cfg.SourceIncludes())
			}
			if strings.Contains(src, "<no value>") {
				t.Errorf("%s has an unrendered value:\n%s", typeScriptFrontendVorpalProgram, src)
			}
		})
	}
}

// typeScriptFrontendArray decodes the string array the rendered program
// declares as `const <name>: string[] = [ ... ];`, one JSON string per line.
func typeScriptFrontendArray(t *testing.T, src, name string) []string {
	t.Helper()
	_, rest, found := strings.Cut(src, "const "+name+": string[] = [\n")
	if !found {
		t.Fatalf("no %s array in:\n%s", name, src)
	}
	body, _, found := strings.Cut(rest, "];\n")
	if !found {
		t.Fatalf("%s array is not closed", name)
	}
	values := []string{}
	for line := range strings.SplitSeq(strings.TrimSuffix(body, "\n"), "\n") {
		if line == "" {
			continue
		}
		var v string
		if err := json.Unmarshal([]byte(strings.TrimSuffix(strings.TrimSpace(line), ",")), &v); err != nil {
			t.Fatalf("%s entry %q is not a JSON string: %v", name, line, err)
		}
		values = append(values, v)
	}
	return values
}

// The frontend source keeps only the default allowlist, so a local .env
// never reaches the registry, and always drops the output, node_modules and
// the config program.
func TestTypeScriptFrontendVorpalSourceIsAllowlisted(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate   func(*TypeScriptFrontendConfig)
		includes []string
		excludes []string
	}{
		"default": {
			func(*TypeScriptFrontendConfig) {},
			typeScriptFrontendDefaultSourceIncludes,
			[]string{"dist", "node_modules", "src/vorpal.ts"},
		},
		"portal": {
			func(c *TypeScriptFrontendConfig) { *c = typeScriptFrontendPortalConfig() },
			typeScriptFrontendPortalIncludes,
			[]string{"dist", "node_modules", "src/vorpal.ts"},
		},
		"output": {
			func(c *TypeScriptFrontendConfig) { c.Output = "build/web" },
			typeScriptFrontendDefaultSourceIncludes,
			[]string{"build/web", "node_modules", "src/vorpal.ts"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := NewTypeScriptFrontendConfig(Config{Name: "demo", Template: TypeScriptFrontendName})
			tc.mutate(&cfg)
			typeScriptFrontendRender(t, cfg)
			src := goLambdaReadFile(t, typeScriptFrontendVorpalProgram)
			excludes, includes := typeScriptFrontendArray(t, src, "sourceExcludes"), typeScriptFrontendArray(t, src, "sourceIncludes")
			if !slices.Equal(includes, tc.includes) {
				t.Errorf("source includes = %q, want %q", includes, tc.includes)
			}
			if !slices.Equal(excludes, tc.excludes) {
				t.Errorf("source excludes = %q, want %q", excludes, tc.excludes)
			}
		})
	}
}

// The justfile runs the named package.json scripts under bun, renders test
// only with scripts.test, and runs generate before check when set.
func TestTypeScriptFrontendJustfileRecipes(t *testing.T) {
	typeScriptFrontendRender(t, typeScriptFrontendPortalConfig())
	src := goLambdaReadFile(t, "justfile")
	for _, want := range []string{
		"build: install\n    bun --bun run build:prod\n",
		"check: install generate\n    bun --bun run lint\n    bun --bun run build:tsc\n",
		"generate: install\n    #!/usr/bin/env bash\n    set -euxo pipefail\n    " + typeScriptFrontendPortalGenerate + "\n",
		"install:\n    bun install --frozen-lockfile\n",
		"package name=\"quirk-frontend-portal\":\n    vorpal build --path '{{ name }}'\n",
		"vuln-scan: install\n    bun audit\n",
		"secret-scan:\n    gitleaks git",
		"doc-validate:\n    rumdl check",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("justfile does not contain %q:\n%s", want, src)
		}
	}
	for _, absent := range []string{"test:", "npm run", "npx", "node "} {
		if strings.Contains(src, absent) {
			t.Errorf("portal justfile contains %q", absent)
		}
	}

	cfg := NewTypeScriptFrontendConfig(Config{Name: "demo", Template: TypeScriptFrontendName})
	typeScriptFrontendRender(t, cfg)
	src = goLambdaReadFile(t, "justfile")
	if !strings.Contains(src, "check: install\n") || strings.Contains(src, "generate:") {
		t.Errorf("default justfile check has extra dependencies or a generate recipe:\n%s", src)
	}

	cfg.Scripts.Test = "test:unit"
	cfg.Audit = TypeScriptFrontendAuditConfig{Level: "high", Production: true}
	typeScriptFrontendRender(t, cfg)
	src = goLambdaReadFile(t, "justfile")
	for _, want := range []string{
		"test: install\n    bun --bun run test:unit\n",
		"vuln-scan: install\n    bun audit --audit-level=high --production\n",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("justfile does not contain %q:\n%s", want, src)
		}
	}
}

type typeScriptFrontendWorkflow struct {
	On          map[string]any `yaml:"on"`
	Permissions map[string]any `yaml:"permissions"`
	Jobs        map[string]struct {
		If          string            `yaml:"if"`
		Needs       scanJobsNeeds     `yaml:"needs"`
		Permissions map[string]string `yaml:"permissions"`
		RunsOn      string            `yaml:"runs-on"`
		Environment any               `yaml:"environment"`
		Steps       []scanJobsStep    `yaml:"steps"`
	} `yaml:"jobs"`
}

func typeScriptFrontendReadWorkflow(t *testing.T) typeScriptFrontendWorkflow {
	t.Helper()
	src := goLambdaReadFile(t, ".github/workflows/ci.yaml")
	var wf typeScriptFrontendWorkflow
	if err := yaml.Unmarshal([]byte(src), &wf); err != nil {
		t.Fatalf("workflow does not parse: %v\n%s", err, src)
	}
	return wf
}

// The workflow has exactly a check and a build job, each with the fork guard
// and `contents: read` only. Nothing requests an OIDC token or a secret, and
// no job deploys or publishes.
func TestTypeScriptFrontendWorkflowHardening(t *testing.T) {
	for name, cfg := range map[string]TypeScriptFrontendConfig{
		"default": NewTypeScriptFrontendConfig(Config{Name: "demo", Template: TypeScriptFrontendName}),
		"portal":  typeScriptFrontendPortalConfig(),
	} {
		t.Run(name, func(t *testing.T) {
			typeScriptFrontendRender(t, cfg)
			src := goLambdaReadFile(t, ".github/workflows/ci.yaml")
			wf := typeScriptFrontendReadWorkflow(t)

			if _, ok := wf.On["push"]; !ok {
				t.Errorf("workflow on = %v, want push", wf.On)
			}
			if _, ok := wf.On["pull_request"]; !ok {
				t.Errorf("workflow on = %v, want pull_request", wf.On)
			}
			if _, ok := wf.On["pull_request_target"]; ok {
				t.Errorf("workflow runs on pull_request_target")
			}
			if wf.Permissions != nil {
				t.Errorf("workflow-level permissions = %v, want none", wf.Permissions)
			}

			var jobs []string
			for job := range wf.Jobs {
				jobs = append(jobs, job)
			}
			slices.Sort(jobs)
			if !slices.Equal(jobs, []string{"build", "check"}) {
				t.Errorf("jobs = %q, want [build check]", jobs)
			}

			guard := GoLambdaConfig{}.ForkGuardedIf("")
			for name, job := range wf.Jobs {
				if job.If != guard {
					t.Errorf("job %s if = %q, want the fork guard %q", name, job.If, guard)
				}
				if len(job.Permissions) != 1 || job.Permissions["contents"] != "read" {
					t.Errorf("job %s permissions = %v, want contents: read only", name, job.Permissions)
				}
				if job.RunsOn != "altf4llc" {
					t.Errorf("job %s runs-on = %q, want altf4llc", name, job.RunsOn)
				}
				if job.Environment != nil {
					t.Errorf("job %s runs in environment %v", name, job.Environment)
				}
			}
			for _, absent := range []string{"id-token", "secrets.", "configure-aws-credentials", "aws s3", "cloudfront", "netrc", "upload-artifact"} {
				if strings.Contains(src, absent) {
					t.Errorf("workflow contains %q", absent)
				}
			}
		})
	}
}

// The check job runs the scans, `just check` and `vuln-scan` in that order,
// with `test` last only when scripts.test is set; build packages the
// frontend with Vorpal after generate.
func TestTypeScriptFrontendWorkflowSteps(t *testing.T) {
	typeScriptFrontendRender(t, typeScriptFrontendPortalConfig())
	wf := scanJobsWorkflow{}
	if err := yaml.Unmarshal([]byte(goLambdaReadFile(t, ".github/workflows/ci.yaml")), &wf); err != nil {
		t.Fatal(err)
	}
	want := []string{"just secret-scan", "just doc-validate", "just check", "just vuln-scan"}
	if got := scanJobsCheckRuns(wf); !slices.Equal(got, want) {
		t.Errorf("portal check runs %q, want %q", got, want)
	}
	var build []string
	for _, s := range wf.Jobs["build"].Steps {
		if s.Run != "" {
			build = append(build, s.Run)
		}
	}
	if want := []string{"just generate", `just package "quirk-frontend-portal"`}; !slices.Equal(build, want) {
		t.Errorf("portal build runs %q, want %q", build, want)
	}

	cfg := NewTypeScriptFrontendConfig(Config{Name: "demo", Template: TypeScriptFrontendName})
	cfg.Scripts.Test = "test"
	cfg.Generate = []string{"just gen"}
	typeScriptFrontendRender(t, cfg)
	wf = scanJobsWorkflow{}
	if err := yaml.Unmarshal([]byte(goLambdaReadFile(t, ".github/workflows/ci.yaml")), &wf); err != nil {
		t.Fatal(err)
	}
	want = []string{"just secret-scan", "just doc-validate", "just check", "just vuln-scan", "just test"}
	if got := scanJobsCheckRuns(wf); !slices.Equal(got, want) {
		t.Errorf("check runs %q, want %q", got, want)
	}
	build = nil
	for _, s := range wf.Jobs["build"].Steps {
		if s.Run != "" {
			build = append(build, s.Run)
		}
	}
	if want := []string{"just generate", `just package "demo"`}; !slices.Equal(build, want) {
		t.Errorf("build runs %q, want %q", build, want)
	}
}

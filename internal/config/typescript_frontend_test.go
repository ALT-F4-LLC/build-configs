package config

import (
	"go/format"
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
func typeScriptFrontendPortalConfig() TypeScriptFrontendConfig {
	cfg := NewTypeScriptFrontendConfig(Config{Name: "quirk-frontend-portal", Template: TypeScriptFrontendName})
	cfg.Scripts.Typecheck = "build:tsc"
	cfg.Generate = []string{typeScriptFrontendPortalGenerate}
	cfg.Vorpal.Tools = []string{"openapi-generator-cli:7.18.0", "openjdk:25.0.1"}
	cfg.Vorpal.Includes = append(slices.Clone(typeScriptFrontendDefaultSourceIncludes), "env")
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
	if fe.Scripts.Build != "build" || fe.Scripts.Lint != "lint" || fe.Scripts.Typecheck != "build:tsc" || fe.Scripts.Test != "" {
		t.Errorf("scripts = %+v, want the defaults with typecheck build:tsc and no test", fe.Scripts)
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
		".github/workflows/ci.yaml", "Vorpal.toml", "cmd/vorpal/main.go", "justfile",
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

// The Vorpal program builds the TypeScript development environment and one
// artifact that installs from bun.lock, runs the build script under bun, and
// copies the output directory.
func TestTypeScriptFrontendVorpalProgramBuildsFrontend(t *testing.T) {
	cfg := typeScriptFrontendPortalConfig()
	cfg.Scripts.Build = "build:prod"
	cfg.Output = "out"
	typeScriptFrontendRender(t, cfg)
	src := goLambdaReadFile(t, "cmd/vorpal/main.go")
	lits := goServiceProgramStrings(t)

	for _, want := range []string{"quirk-frontend-portal", "quirk-frontend-portal-dev", "gitleaks:8.30.1", "rumdl:0.3.2", "openapi-generator-cli:7.18.0", "openjdk:25.0.1"} {
		if !slices.Contains(lits, want) {
			t.Errorf("cmd/vorpal/main.go has no string literal %q", want)
		}
	}
	for _, want := range []string{
		"language.NewTypeScriptDevelopmentEnvironment(",
		"artifact.Bun(ctx)",
		"artifact.Shell(ctx, []*string{bun}",
		"bun install --frozen-lockfile\n",
		"bun --bun run build:prod\n",
		`test -d "out"`,
		`cp -R "out/." "$VORPAL_OUTPUT/"`,
		`pushd "./source/quirk-frontend-portal"`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("cmd/vorpal/main.go does not contain %q", want)
		}
	}
	for _, absent := range []string{"NewOciImage", "readNetrc", "WithSecrets", "GOPRIVATE", "project."} {
		if strings.Contains(src, absent) {
			t.Errorf("cmd/vorpal/main.go contains %q", absent)
		}
	}
}

// The rendered Vorpal program is gofmt-clean for every combination of
// tools, script and includes.
func TestTypeScriptFrontendVorpalProgramGofmtClean(t *testing.T) {
	for name, mutate := range map[string]func(*TypeScriptFrontendConfig){
		"default": func(*TypeScriptFrontendConfig) {},
		"portal":  func(c *TypeScriptFrontendConfig) { *c = typeScriptFrontendPortalConfig() },
		"no tools": func(c *TypeScriptFrontendConfig) {
			c.Vorpal.ExcludeTools = []string{"gitleaks", "jq", "just", "rumdl"}
		},
		"script tools": func(c *TypeScriptFrontendConfig) {
			c.Vorpal.Script = "echo hi"
			c.Vorpal.Tools = []string{"awscli2:2.33.1"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := NewTypeScriptFrontendConfig(Config{Name: "demo", Template: TypeScriptFrontendName})
			mutate(&cfg)
			typeScriptFrontendRender(t, cfg)
			src := goLambdaReadFile(t, "cmd/vorpal/main.go")
			formatted, err := format.Source([]byte(src))
			if err != nil {
				t.Fatalf("cmd/vorpal/main.go does not parse: %v\n%s", err, src)
			}
			if string(formatted) != src {
				t.Errorf("cmd/vorpal/main.go is not gofmt-clean:\n%s", src)
			}
		})
	}
}

// The frontend source keeps only the default allowlist, so a local .env
// never reaches the registry, and always drops the output and node_modules.
func TestTypeScriptFrontendVorpalSourceIsAllowlisted(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate   func(*TypeScriptFrontendConfig)
		includes []string
		excludes []string
	}{
		"default": {
			func(*TypeScriptFrontendConfig) {},
			[]string{"bun.lock", "bunfig.toml", "index.html", "package.json", "postcss.config", "public", "src", "tailwind.config", "tsconfig", "vite.config"},
			[]string{"dist", "node_modules"},
		},
		"portal": {
			func(c *TypeScriptFrontendConfig) { *c = typeScriptFrontendPortalConfig() },
			[]string{"bun.lock", "bunfig.toml", "index.html", "package.json", "postcss.config", "public", "src", "tailwind.config", "tsconfig", "vite.config", "env"},
			[]string{"dist", "node_modules"},
		},
		"output": {
			func(c *TypeScriptFrontendConfig) { c.Output = "build/web" },
			[]string{"bun.lock", "bunfig.toml", "index.html", "package.json", "postcss.config", "public", "src", "tailwind.config", "tsconfig", "vite.config"},
			[]string{"build/web", "node_modules"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := NewTypeScriptFrontendConfig(Config{Name: "demo", Template: TypeScriptFrontendName})
			tc.mutate(&cfg)
			typeScriptFrontendRender(t, cfg)
			excludes, includes := goLambdaSourceFilter(t, goLambdaParseVorpalProgram(t))
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
		"build: install\n    bun --bun run build\n",
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

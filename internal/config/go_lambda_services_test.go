package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/ast"
	"go/format"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const goLambdaTestExtension = "github.com/example/platform/internal/project"

// goLambdaRenderServices renders go-lambda with the given lambdas, services and
// vorpal.extension into a fresh temporary directory, which becomes the working
// directory.
func goLambdaRenderServices(t *testing.T, lambdas, services []string, extension string) {
	t.Helper()
	t.Chdir(t.TempDir())

	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Lambdas = lambdas
	cfg.Services = services
	cfg.Vorpal.Extension = extension
	if err := cfg.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

func TestGoLambdaServiceVorpalProgram(t *testing.T) {
	goLambdaRenderServices(t, []string{"api"}, []string{"svc"}, "")
	file := goLambdaParseVorpalProgram(t)

	loops := goLambdaBuildLoops(t, file)
	if len(loops) != 2 {
		t.Fatalf("got %d build loops, want a lambda loop and a service loop", len(loops))
	}
	for i, want := range []struct {
		loopVar string
		names   []string
		flags   string
	}{
		{"lambda", []string{"api"}, `"-ldflags='-s -w' -tags='lambda.norpc'"`},
		{"service", []string{"svc"}, `"-ldflags='-s -w'"`}, // no lambda.norpc
	} {
		loop := loops[i]
		if loop.Var != want.loopVar || !slices.Equal(loop.Names, want.names) {
			t.Errorf("loop %d over %s builds %q, want %s over %q", i, loop.Var, loop.Names, want.loopVar, want.names)
		}
		if got, wantDir := loop.Calls["WithBuildDirectory"], `"cmd/" + `+want.loopVar; got != wantDir {
			t.Errorf("%s build directory = %s, want %s", want.loopVar, got, wantDir)
		}
		if got := loop.Calls["WithBuildFlags"]; got != want.flags {
			t.Errorf("%s build flags = %s, want %s", want.loopVar, got, want.flags)
		}
	}
	cgo := goLambdaCgoDisabledUses(t, file)
	if cgo != 2 {
		t.Errorf("cgoDisabled is used %d times, want 2", cgo)
	}
}

func TestGoLambdaServiceOnly(t *testing.T) {
	goLambdaRenderServices(t, nil, []string{"svc"}, "")

	file := goLambdaParseVorpalProgram(t)
	loops := goLambdaBuildLoops(t, file)
	if len(loops) != 1 || loops[0].Var != "service" || !slices.Equal(loops[0].Names, []string{"svc"}) {
		t.Errorf("build loops = %+v, want one service loop over [svc]", loops)
	}

	wf := goLambdaParseWorkflow(t)
	if src := goLambdaReadFile(t, goLambdaWorkflowPath); strings.HasSuffix(src, "\n\n") {
		t.Errorf("workflow ends with a blank line:\n%q", src)
	}
	if _, ok := wf.Jobs["build-services"]; ok {
		t.Error("service-only config renders a build-services job")
	}
	var runs []string
	for _, step := range wf.Jobs["build"].Steps {
		if step.Run != "" && !strings.HasPrefix(step.Run, "rm -f") {
			runs = append(runs, step.Run)
		}
	}
	if want := []string{`just package-artifact "svc"`}; !slices.Equal(runs, want) {
		t.Errorf("service-only build runs %q, want %q", runs, want)
	}
}

func TestGoLambdaServiceCI(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Lambdas = []string{"api", "worker"}
	cfg.Services = []string{"svc"}
	cfg.PrivateModules = "github.com/example/private"
	cfg.Deploy = []DeployConfig{{Environment: "dev", Region: "us-west-2", RoleARN: "arn:aws:iam::111111111111:role/dev"}}
	if err := cfg.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	src := goLambdaReadFile(t, goLambdaWorkflowPath)
	var wf struct {
		Jobs map[string]struct {
			Needs    any `yaml:"needs"`
			Strategy struct {
				Matrix map[string][]string `yaml:"matrix"`
			} `yaml:"strategy"`
			Steps []goLambdaWorkflowStep `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(src), &wf); err != nil {
		t.Fatalf("workflow does not parse: %v\n%s", err, src)
	}

	if _, ok := wf.Jobs["build-services"]; ok {
		t.Error("services render a separate build-services job")
	}
	build, ok := wf.Jobs["build"]
	if !ok {
		t.Fatalf("no build job:\n%s", src)
	}
	if len(build.Strategy.Matrix) != 0 {
		t.Errorf("build job has a matrix %v, want one job for every entry", build.Strategy.Matrix)
	}
	var runs, uploads []string
	for _, step := range build.Steps {
		if step.Run != "" && !strings.HasPrefix(step.Run, "rm -f") {
			runs = append(runs, step.Run)
		}
		if strings.HasPrefix(step.Uses, "actions/upload-artifact") {
			uploads = append(uploads, step.Uses)
		}
	}
	if want := []string{`just package-artifact "svc"`, `just package "api"`, `just package "worker"`}; !slices.Equal(runs, want) {
		t.Errorf("build runs %q, want %q", runs, want)
	}
	if len(uploads) != 2 {
		t.Errorf("build uploads %d artifacts, want one zip per lambda", len(uploads))
	}

	// The upload and deploy steps must not list the service.
	if strings.Count(src, "svc") != 1 {
		t.Errorf("svc appears %d times in the workflow, want only its package step:\n%s", strings.Count(src, "svc"), src)
	}

	just := goLambdaReadFile(t, "justfile")
	if strings.Contains(just, "svc") {
		t.Errorf("justfile mentions svc:\n%s", just)
	}
}

func TestGoLambdaServiceNotScaffolded(t *testing.T) {
	goLambdaRenderServices(t, []string{"api"}, []string{"svc"}, "")

	if _, err := os.Stat("cmd/svc/main.go"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("cmd/svc/main.go: stat error = %v, want not exist", err)
	}
	if _, err := os.Stat("cmd/api/main.go"); err != nil {
		t.Errorf("cmd/api/main.go was not scaffolded: %v", err)
	}
}

func TestGoLambdaNameInLambdasAndServices(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Lambdas = []string{"api", "worker"}
	cfg.Services = []string{"svc", "worker"}

	err := cfg.Render()
	if err == nil {
		t.Fatal("Render succeeded with worker in both lambdas and services")
	}
	if !strings.Contains(err.Error(), `"worker"`) {
		t.Errorf("error %q does not name worker", err)
	}
	entries, readErr := os.ReadDir(".")
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Errorf("failed Render wrote %d entries", len(entries))
	}
}

func TestGoLambdaServicesOnlyWithDeployIsAnError(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Lambdas = nil
	cfg.Services = []string{"svc"}
	cfg.Deploy = []DeployConfig{{Environment: "dev", Region: "us-west-2", RoleARN: "arn:aws:iam::111111111111:role/dev"}}

	err := cfg.Render()
	if err == nil || !strings.Contains(err.Error(), "deploy") {
		t.Fatalf("Render error = %v, want one about deploy without lambdas", err)
	}
	if entries, readErr := os.ReadDir("."); readErr != nil || len(entries) != 0 {
		t.Errorf("failed Render wrote %d entries (err %v)", len(entries), readErr)
	}
}

func TestGoLambdaExtensionVorpalProgram(t *testing.T) {
	for _, tc := range []struct {
		name     string
		lambdas  []string
		services []string
	}{
		{name: "lambda only", lambdas: []string{"api"}},
		{name: "service only", services: []string{"svc"}},
		{name: "nothing to build"},
		{name: "both", lambdas: []string{"api", "worker"}, services: []string{"svc", "mcp-server"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			goLambdaRenderServices(t, tc.lambdas, tc.services, goLambdaTestExtension)

			src, err := os.ReadFile("cmd/vorpal/main.go")
			if err != nil {
				t.Fatal(err)
			}
			formatted, err := format.Source(src)
			if err != nil {
				t.Fatalf("gofmt: %v\n%s", err, src)
			}
			if !bytes.Equal(formatted, src) {
				t.Errorf("cmd/vorpal/main.go is not gofmt-clean:\n%s", src)
			}

			file := goLambdaParseVorpalProgram(t)

			imported := false
			for _, imp := range file.Imports {
				path, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					t.Fatal(err)
				}
				if path == goLambdaTestExtension {
					imported = true
					if imp.Name == nil || imp.Name.Name != "project" {
						t.Errorf("extension import alias = %v, want project", imp.Name)
					}
				}
			}
			if !imported {
				t.Errorf("program does not import %q", goLambdaTestExtension)
			}

			// Source-ordered positions of the calls that matter.
			var devEnv, artifacts []int
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch {
				case goLambdaIsSelector(call.Fun, "language", "NewGoDevelopmentEnvironment"):
					devEnv = append(devEnv, int(call.Pos()))
				case goLambdaIsSelector(call.Fun, "project", "Artifacts"):
					artifacts = append(artifacts, int(call.Pos()))
				}
				return true
			})

			if len(artifacts) != 1 {
				t.Fatalf("project.Artifacts called %d times, want 1", len(artifacts))
			}
			if len(devEnv) != 1 || artifacts[0] < devEnv[0] {
				t.Errorf("project.Artifacts must follow the development environment")
			}

			want := slices.Concat(tc.lambdas, tc.services)
			loops := goLambdaBuildLoops(t, file)
			if got := goLambdaLoopNames(loops); !slices.Equal(got, want) {
				t.Errorf("build loop names = %q, want %q", got, want)
			}
			// An unused `builder` variable would not compile.
			if len(want) == 0 && bytes.Contains(src, []byte("builder")) {
				t.Errorf("builder declared with nothing to build:\n%s", src)
			}
			for _, loop := range loops {
				if int(loop.Loop.Pos()) < artifacts[0] {
					t.Errorf("loop over %s precedes project.Artifacts", loop.Var)
				}
				if !loop.Hooked {
					t.Errorf("loop over %s does not pass its builder through project.Go(ctx, %s, ...)", loop.Var, loop.Var)
				}
				if got, wantNewGo := loop.Calls["NewGo"], loop.Var+", config.SYSTEMS"; got != wantNewGo {
					t.Errorf("NewGo(%s), want NewGo(%s)", got, wantNewGo)
				}

				// Inside the loop body, project.Go must come before builder.Build.
				var goPos, buildPos []int
				ast.Inspect(loop.Loop.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					if goLambdaIsSelector(call.Fun, "project", "Go") {
						goPos = append(goPos, int(call.Pos()))
					}
					if goLambdaIsSelector(call.Fun, "builder", "Build") {
						buildPos = append(buildPos, int(call.Pos()))
					}
					return true
				})
				if len(goPos) != 1 || len(buildPos) != 1 {
					t.Fatalf("loop over %s: project.Go %d times, builder.Build %d times, want 1 each", loop.Var, len(goPos), len(buildPos))
				}
				if goPos[0] > buildPos[0] {
					t.Errorf("loop over %s builds before passing through project.Go", loop.Var)
				}
			}
		})
	}
}

func TestGoLambdaExtensionUnsetHasNoProjectHooks(t *testing.T) {
	goLambdaRenderServices(t, []string{"api"}, []string{"svc"}, "")

	src, err := os.ReadFile("cmd/vorpal/main.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"project.", "project \"", "builder"} {
		if bytes.Contains(src, []byte(banned)) {
			t.Errorf("cmd/vorpal/main.go contains %q with vorpal.extension unset:\n%s", banned, src)
		}
	}
}

func TestGoLambdaServicesAndExtensionRoundTrip(t *testing.T) {
	const (
		yamlParams = `name: example
template: go-lambda
parameters:
  services: [svc, mcp]
  vorpal:
    extension: ` + goLambdaTestExtension + "\n"
		jsonParams = `{"name":"example","template":"go-lambda","parameters":{"services":["svc","mcp"],"vorpal":{"extension":"` + goLambdaTestExtension + `"}}}`
	)

	for _, tc := range []struct {
		name  string
		parse func(t *testing.T) Config
	}{
		{"yaml", func(t *testing.T) Config {
			var c Config
			if err := yaml.Unmarshal([]byte(yamlParams), &c); err != nil {
				t.Fatal(err)
			}
			return c
		}},
		{"json", func(t *testing.T) Config {
			var c Config
			if err := json.Unmarshal([]byte(jsonParams), &c); err != nil {
				t.Fatal(err)
			}
			return c
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tpl, err := tc.parse(t).GetTemplater()
			if err != nil {
				t.Fatalf("GetTemplater: %v", err)
			}
			got, ok := tpl.(GoLambdaConfig)
			if !ok {
				t.Fatalf("templater is %T, want GoLambdaConfig", tpl)
			}
			if !slices.Equal(got.Services, []string{"svc", "mcp"}) {
				t.Errorf("Services = %q", got.Services)
			}
			if got.Vorpal.Extension != goLambdaTestExtension {
				t.Errorf("Vorpal.Extension = %q", got.Vorpal.Extension)
			}
			if want := NewGoLambdaConfig(Config{}).DevTools(); !slices.Equal(got.DevTools(), want) {
				t.Errorf("DevTools() = %q, want default %q", got.DevTools(), want)
			}
			if !slices.Equal(got.Lambdas, []string{"example"}) {
				t.Errorf("Lambdas = %q, want default [example]", got.Lambdas)
			}
		})
	}
}

func TestGoLambdaServicesDefaultEmpty(t *testing.T) {
	cfg := NewGoLambdaConfig(Config{Name: "example"})
	if len(cfg.Services) != 0 || cfg.Vorpal.Extension != "" {
		t.Errorf("defaults: services = %q, extension = %q, want both empty", cfg.Services, cfg.Vorpal.Extension)
	}
}

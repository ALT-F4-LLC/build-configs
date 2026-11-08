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

	if got, want := goLambdaCallStringArgs(t, file, "language", "NewGo"), []string{"api", "svc"}; !slices.Equal(got, want) {
		t.Errorf("NewGo artifacts = %q, want %q", got, want)
	}
	if got, want := goLambdaMethodStringArgs(t, file, "WithBuildDirectory"), []string{"cmd/api", "cmd/svc"}; !slices.Equal(got, want) {
		t.Errorf("build directories = %q, want %q", got, want)
	}
	wantFlags := []string{
		"-ldflags='-s -w' -tags='lambda.norpc'", // api
		"-ldflags='-s -w'",                      // svc: no lambda.norpc
	}
	if got := goLambdaMethodStringArgs(t, file, "WithBuildFlags"); !slices.Equal(got, wantFlags) {
		t.Errorf("build flags = %q, want %q", got, wantFlags)
	}
	cgo := 0
	for _, lit := range goLambdaStringLits(t, file) {
		if lit == "CGO_ENABLED=0" {
			cgo++
		}
	}
	if cgo != 2 {
		t.Errorf("CGO_ENABLED=0 appears %d times, want 2", cgo)
	}
}

func TestGoLambdaServiceOnly(t *testing.T) {
	goLambdaRenderServices(t, nil, []string{"svc"}, "")

	file := goLambdaParseVorpalProgram(t)
	if got, want := goLambdaCallStringArgs(t, file, "language", "NewGo"), []string{"svc"}; !slices.Equal(got, want) {
		t.Errorf("NewGo artifacts = %q, want %q", got, want)
	}

	wf := goLambdaParseWorkflow(t)
	if src := goLambdaReadFile(t, goLambdaWorkflowPath); strings.HasSuffix(src, "\n\n") {
		t.Errorf("workflow ends with a blank line:\n%q", src)
	}
	if _, ok := wf.Jobs["build"]; ok {
		t.Error("service-only config renders a lambda build job with an empty matrix")
	}
	if _, ok := wf.Jobs["build-services"]; !ok {
		t.Error("service-only config has no build-services job")
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

	svc, ok := wf.Jobs["build-services"]
	if !ok {
		t.Fatalf("no build-services job:\n%s", src)
	}
	if got := svc.Strategy.Matrix["service"]; !slices.Equal(got, []string{"svc"}) {
		t.Errorf("service matrix = %q, want [svc]", got)
	}
	var runs []string
	for _, step := range svc.Steps {
		if step.Run != "" {
			runs = append(runs, step.Run)
		}
		if strings.HasPrefix(step.Uses, "actions/upload-artifact") {
			t.Error("build-services uploads an artifact")
		}
	}
	if !slices.Contains(runs, `just build "${{ matrix.service }}"`) {
		t.Errorf("build-services runs %q, want just build", runs)
	}
	for _, run := range runs {
		if strings.Contains(run, "just package") || strings.Contains(run, "just deploy") {
			t.Errorf("build-services runs %q", run)
		}
	}

	// The lambda build, upload and deploy matrices must not list the service.
	for name, job := range wf.Jobs {
		if name == "build-services" {
			continue
		}
		if slices.Contains(job.Strategy.Matrix["profile"], "svc") {
			t.Errorf("job %s lists svc in its profile matrix", name)
		}
	}
	if strings.Count(src, "svc") != 1 {
		t.Errorf("svc appears %d times in the workflow, want only its matrix entry:\n%s", strings.Count(src, "svc"), src)
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
			var goNames, buildCalls []string
			var goPos, buildPos []int
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
				case goLambdaIsSelector(call.Fun, "project", "Go"):
					goNames = append(goNames, goLambdaStringLit(t, call.Args[1]))
					goPos = append(goPos, int(call.Pos()))
				default:
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Build" && len(call.Args) == 1 {
						if id, ok := sel.X.(*ast.Ident); ok && id.Name == "builder" {
							buildCalls = append(buildCalls, "builder")
							buildPos = append(buildPos, int(call.Pos()))
						}
					}
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
			if !slices.Equal(goNames, want) {
				t.Errorf("project.Go names = %q, want %q", goNames, want)
			}
			// An unused `builder` variable would not compile.
			if len(want) == 0 && bytes.Contains(src, []byte("builder")) {
				t.Errorf("builder declared with nothing to build:\n%s", src)
			}
			if len(buildCalls) != len(want) {
				t.Fatalf("builder.Build called %d times, want %d", len(buildCalls), len(want))
			}
			for i := range want {
				if artifacts[0] > goPos[i] {
					t.Errorf("project.Go(%q) precedes project.Artifacts", want[i])
				}
				if goPos[i] > buildPos[i] {
					t.Errorf("%q is built before it passes through project.Go", want[i])
				}
				if i+1 < len(want) && buildPos[i] > goPos[i+1] {
					t.Errorf("%q is built after %q passes through project.Go", want[i], want[i+1])
				}
			}
			if got := goLambdaCallStringArgs(t, file, "language", "NewGo"); !slices.Equal(got, want) {
				t.Errorf("NewGo artifacts = %q, want %q", got, want)
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
			if want := NewGoLambdaConfig(Config{}).Vorpal.Tools; !slices.Equal(got.Vorpal.Tools, want) {
				t.Errorf("Vorpal.Tools = %q, want default %q", got.Vorpal.Tools, want)
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

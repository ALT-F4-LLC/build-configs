package config

import (
	"encoding/json"
	"go/format"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGoLambdaOpenAPIOutputAndPackageNameRenderIntoBuildClient(t *testing.T) {
	t.Chdir(t.TempDir())

	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Lambdas = []string{"api"}
	cfg.OpenAPI.Enable = true
	cfg.OpenAPI.Filename = "ai.json"
	cfg.OpenAPI.Output = "internal/client_ai"
	cfg.OpenAPI.PackageName = "client_ai"
	if err := cfg.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	_, body := goLambdaJustfileRecipe(t, goLambdaReadFile(t, "justfile"), "build-client")
	recipe := strings.Join(body, "\n")
	for _, want := range []string{
		"rm -rf internal/client_ai\n",
		"-i api/ai.json",
		"-o internal/client_ai",
		"-p packageName=client_ai",
		"internal/client_ai/go.mod",
	} {
		if !strings.Contains(recipe, want) {
			t.Errorf("build-client recipe missing %q:\n%s", want, recipe)
		}
	}
	if strings.Contains(recipe, "pkg/client") {
		t.Errorf("build-client recipe still names pkg/client:\n%s", recipe)
	}
}

func TestGoLambdaOpenAPIGenerateInCIRunsClientBeforeCheckAndBuild(t *testing.T) {
	t.Chdir(t.TempDir())

	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Lambdas = []string{"api"}
	cfg.OpenAPI.Enable = true
	cfg.OpenAPI.GenerateInCI = true
	if err := cfg.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	justfile := goLambdaReadFile(t, "justfile")
	if !strings.Contains(justfile, "\ncheck: build-client\n") {
		t.Errorf("check does not depend on build-client:\n%s", justfile)
	}

	ci := goLambdaReadFile(t, ".github/workflows/ci.yaml")
	client := strings.Index(ci, "just build-client")
	build := strings.Index(ci, "just build \"")
	if client < 0 || build < 0 || client > build {
		t.Errorf("CI runs build-client at %d and build at %d; want build-client first:\n%s", client, build, ci)
	}
}

func TestGoLambdaOpenAPIGenerateInCIDefaultsOff(t *testing.T) {
	t.Chdir(t.TempDir())

	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Lambdas = []string{"api"}
	cfg.OpenAPI.Enable = true
	if err := cfg.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	if strings.Contains(goLambdaReadFile(t, ".github/workflows/ci.yaml"), "just build-client") {
		t.Error("CI runs build-client without openapi.generateInCI")
	}
	if !strings.Contains(goLambdaReadFile(t, "justfile"), "\ncheck:\n") {
		t.Error("check gained dependencies without openapi.generateInCI")
	}
}

func TestDeployConfigDefaultRoleUsesConfiguredEnvironment(t *testing.T) {
	old := Cfg
	t.Cleanup(func() { Cfg = old })
	Cfg = Config{Name: "example"}

	var got DeployConfig
	if err := json.Unmarshal([]byte(`{"environment":"dev"}`), &got); err != nil {
		t.Fatal(err)
	}
	if want := "arn:aws:iam::677459762413:role/altf4llc-gha-example-deploy-dev"; got.RoleARN != want {
		t.Errorf("RoleARN = %q, want %q", got.RoleARN, want)
	}

	if err := json.Unmarshal([]byte(`{"environment":"dev","roleArn":"arn:aws:iam::1:role/x"}`), &got); err != nil {
		t.Fatal(err)
	}
	if got.RoleARN != "arn:aws:iam::1:role/x" {
		t.Errorf("explicit RoleARN overwritten: %q", got.RoleARN)
	}
}

func TestGolangciConfigDisableRendersLinterList(t *testing.T) {
	out := golangciRender(t, ".golangci.yaml", golangciRenderContext{
		Lint: GolangCILintConfig{Disable: []string{"errcheck"}},
	})

	var doc struct {
		Linters struct {
			Disable []string `yaml:"disable"`
		} `yaml:"linters"`
	}
	if err := yaml.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("rendered output is not valid YAML: %v\n%s", err, out)
	}
	if len(doc.Linters.Disable) != 1 || doc.Linters.Disable[0] != "errcheck" {
		t.Errorf("linters.disable = %q, want [errcheck]\n%s", doc.Linters.Disable, out)
	}
}

func TestGoLambdaOpenAPIRejectsUnsafeOutputAndPackageName(t *testing.T) {
	for name, mutate := range map[string]func(*OpenAPIConfig){
		"empty output":      func(o *OpenAPIConfig) { o.Output = "" },
		"absolute output":   func(o *OpenAPIConfig) { o.Output = "/tmp/client" },
		"parent output":     func(o *OpenAPIConfig) { o.Output = "../client" },
		"nested parent":     func(o *OpenAPIConfig) { o.Output = "a/../../client" },
		"shell output":      func(o *OpenAPIConfig) { o.Output = "client; rm -rf x" },
		"spaced output":     func(o *OpenAPIConfig) { o.Output = "my client" },
		"empty package":     func(o *OpenAPIConfig) { o.PackageName = "" },
		"dashed package":    func(o *OpenAPIConfig) { o.PackageName = "client-ai" },
		"numbered package":  func(o *OpenAPIConfig) { o.PackageName = "1client" },
		"injecting package": func(o *OpenAPIConfig) { o.PackageName = "client;ls" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
			cfg.Lambdas = []string{"api"}
			cfg.OpenAPI.Enable = true
			mutate(&cfg.OpenAPI)
			if err := cfg.Render(); err == nil {
				t.Error("Render accepted an unsafe openapi setting")
			}
		})
	}
}

func TestGoLambdaOpenAPIDisabledSkipsOutputValidation(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Lambdas = []string{"api"}
	cfg.OpenAPI.Output = ""
	if err := cfg.Render(); err != nil {
		t.Errorf("Render with openapi disabled: %v", err)
	}
}

func TestGoLambdaGenerateInCIOrdersClientBeforeTokenAndKeepsGenerateDependency(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Lambdas = []string{"api"}
	cfg.Services = []string{"mcp"}
	cfg.Generate = []string{"echo hi"}
	cfg.OpenAPI.Enable = true
	cfg.OpenAPI.GenerateInCI = true
	if err := cfg.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	if !strings.Contains(goLambdaReadFile(t, "justfile"), "\ncheck: generate build-client\n") {
		t.Errorf("check dependencies wrong:\n%s", goLambdaReadFile(t, "justfile"))
	}

	ci := goLambdaReadFile(t, ".github/workflows/ci.yaml")
	if got := strings.Count(ci, "just build-client"); got != 3 {
		t.Errorf("CI runs build-client %d times, want 3 (check, build-services and build)", got)
	}
	// Each build job must generate the client before it writes the App token to disk.
	for _, job := range strings.Split(ci, "\n\n  ")[1:] {
		if !strings.Contains(job, "just build-client") {
			continue
		}
		if strings.Index(job, "just build-client") > strings.Index(job, "generate-token") {
			t.Errorf("build-client runs after the App token step:\n%s", job)
		}
	}
}

func TestGoLambdaGenerateInCIWithoutOpenAPIAddsNothing(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Lambdas = []string{"api"}
	cfg.OpenAPI.GenerateInCI = true
	if err := cfg.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(goLambdaReadFile(t, "justfile"), "\ncheck:\n") || strings.Contains(goLambdaReadFile(t, ".github/workflows/ci.yaml"), "build-client") {
		t.Error("generateInCI without openapi.enable changed the output")
	}
}

func TestGoLambdaCheckJobWritesNetrcOnlyForPrivateModules(t *testing.T) {
	for name, tc := range map[string]struct {
		private string
		want    bool
	}{
		"private modules": {private: "github.com/example/kit", want: true},
		"none":            {private: "", want: false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
			cfg.Lambdas = []string{"api"}
			cfg.PrivateModules = tc.private
			cfg.Services = []string{"mcp"}
			cfg.Generate = []string{"echo hi"}
			cfg.OpenAPI.Enable = true
			cfg.OpenAPI.GenerateInCI = true
			if err := cfg.Render(); err != nil {
				t.Fatalf("Render: %v", err)
			}

			ci := goLambdaReadFile(t, ".github/workflows/ci.yaml")
			start := strings.Index(ci, "\n  check:\n")
			if start < 0 {
				t.Fatalf("check job not found:\n%s", ci)
			}
			check := ci[start+1:]
			if end := strings.Index(check, "\n\n  "); end >= 0 {
				check = check[:end]
			}

			if got := strings.Contains(check, "generate-token") && strings.Contains(check, "extractions/netrc") && strings.Contains(check, "rm -f ~/.netrc"); got != tc.want {
				t.Errorf("check job has netrc steps = %v, want %v:\n%s", got, tc.want, check)
			}
			if !tc.want {
				if !strings.Contains(check, "just check") || strings.Contains(check, "just lint") {
					t.Errorf("without private modules the check job runs just check only:\n%s", check)
				}
				return
			}

			// Everything that reads the repository's inputs runs before the token
			// is on disk; only the linter runs after it.
			generate, client := strings.Index(check, "just generate"), strings.Index(check, "just build-client")
			token, lint := strings.Index(check, "generate-token"), strings.Index(check, "just lint")
			if generate < 0 || client < generate || token < client || lint < token {
				t.Errorf("want generate, build-client, token, lint; got %d, %d, %d, %d:\n%s", generate, client, token, lint, check)
			}
			if strings.Contains(check, "just check") {
				t.Errorf("check job runs just check after the token, which re-runs generate and build-client:\n%s", check)
			}

			_, body := goLambdaJustfileRecipe(t, goLambdaReadFile(t, "justfile"), "lint")
			if !strings.Contains(strings.Join(body, "\n"), "golangci-lint run ./...") {
				t.Errorf("lint recipe does not run golangci-lint: %q", body)
			}
			if !strings.Contains(goLambdaReadFile(t, "justfile"), "\nlint:\n") {
				t.Error("lint recipe has dependencies; it must run no generator")
			}
		})
	}
}

func TestGoLambdaProgramGivesPrivateModuleBuildsASandboxCredential(t *testing.T) {
	for name, tc := range map[string]struct {
		private   string
		artifacts bool
		extension string
		want      bool
	}{
		"private modules":           {private: "github.com/example/kit", want: true},
		"private modules extension": {private: "github.com/example/kit", extension: "github.com/example/app/internal/vorpal", want: true},
		"artifacts only":            {private: "github.com/example/kit", artifacts: true},
		"none":                      {},
	} {
		t.Run(name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
			cfg.Lambdas = []string{"api"}
			cfg.Services = []string{"mcp"}
			cfg.PrivateModules = tc.private
			cfg.Vorpal.Extension = tc.extension
			if tc.artifacts {
				cfg.Lambdas, cfg.Services, cfg.Deploy = nil, nil, nil
				cfg.Artifacts = []string{"client-go"}
			}
			if err := cfg.Render(); err != nil {
				t.Fatalf("Render: %v", err)
			}

			src := goLambdaReadFile(t, "cmd/vorpal/main.go")
			if _, err := format.Source([]byte(src)); err != nil {
				t.Fatalf("cmd/vorpal/main.go does not parse: %v\n%s", err, src)
			}

			wired := strings.Contains(src, "newPrivateModuleSandbox")
			if wired != tc.want {
				t.Fatalf("sandbox wiring = %v, want %v:\n%s", wired, tc.want, src)
			}
			if !tc.want {
				for _, absent := range []string{"NETRC_", "GIT_SSL_CAINFO", ".netrc"} {
					if strings.Contains(src, absent) {
						t.Errorf("program without private modules mentions %q", absent)
					}
				}
				return
			}

			// Each lambda and service builder gets the sandbox settings, and none
			// is left with a bare environment list.
			if got := strings.Count(src, "WithSourceScript(sandbox.script)"); got != 2 {
				t.Errorf("%d builders get the sandbox script, want 2:\n%s", got, src)
			}
			if got := strings.Count(src, `sandbox.environments("CGO_ENABLED=0")`); got != 2 {
				t.Errorf("%d builders get the sandbox environment, want 2:\n%s", got, src)
			}
			if got := strings.Count(src, "WithSecrets(sandbox.secrets)"); got != 2 {
				t.Errorf("%d builders get the sandbox secrets, want 2:\n%s", got, src)
			}
			if strings.Contains(src, `WithEnvironments([]string{"CGO_ENABLED=0"})`) {
				t.Errorf("a builder keeps a bare environment list:\n%s", src)
			}
			for _, want := range []string{
				"GIT_SSL_CAINFO=/etc/ssl/certs/ca-certificates.crt",
				"GOPRIVATE=github.com/example/kit",
				`case "default":`,
				"umask 077",
				"HOME=$VORPAL_WORKSPACE",
				`chmod 600 "$VORPAL_WORKSPACE/.netrc"`,
				`strings.HasSuffix(target, "-linux")`,
			} {
				if !strings.Contains(src, want) {
					t.Errorf("program missing %q:\n%s", want, src)
				}
			}
		})
	}
}

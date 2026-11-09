package config

import (
	"encoding/json"
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
	if got := strings.Count(ci, "just build-client"); got != 2 {
		t.Errorf("CI runs build-client %d times, want 2 (build-services and build)", got)
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

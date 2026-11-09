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

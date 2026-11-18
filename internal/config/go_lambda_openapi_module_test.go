package config

import (
	"strings"
	"testing"
)

func goLambdaOpenAPIModuleConfig(module, output string, generateInCI bool) GoLambdaConfig {
	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Lambdas = []string{"api"}
	cfg.OpenAPI.Enable = true
	cfg.OpenAPI.Filename = "spec.yaml"
	cfg.OpenAPI.Output = output
	cfg.OpenAPI.PackageName = "client_example"
	cfg.OpenAPI.GenerateInCI = generateInCI
	cfg.OpenAPI.Module = module
	return cfg
}

func TestGoLambdaOpenAPIModuleWritesGoModAndCheckClient(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := goLambdaOpenAPIModuleConfig("github.com/example/svc/client/example", "client/example", false).Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	justfile := goLambdaReadFile(t, "justfile")
	_, body := goLambdaJustfileRecipe(t, justfile, "build-client")
	if want := `printf 'module %s\n\ngo %s\n' 'github.com/example/svc/client/example' '1.22' > client/example/go.mod`; !strings.Contains(strings.Join(body, "\n"), want) {
		t.Errorf("build-client does not write the module go.mod:\n%s", strings.Join(body, "\n"))
	}
	if !strings.Contains(justfile, "\ncheck-client: build-client\n") {
		t.Errorf("justfile has no check-client recipe depending on build-client")
	}
	wf := goLambdaReadFile(t, goLambdaWorkflowPath)
	_, check, ok := strings.Cut(wf, "  check:")
	check, _, found := strings.Cut(check, "generate-token")
	if !ok || !found {
		t.Fatalf("workflow has no check job with a token step:\n%s", wf)
	}
	if !strings.Contains(check, "just check-client") {
		t.Errorf("check job does not run check-client before the token:\n%s", check)
	}
}

func TestGoLambdaOpenAPIModuleRejected(t *testing.T) {
	for name, cfg := range map[string]GoLambdaConfig{
		"generateInCI":    goLambdaOpenAPIModuleConfig("github.com/example/svc/client/example", "client/example", true),
		"internal output": goLambdaOpenAPIModuleConfig("github.com/example/svc/internal/client", "internal/client", false),
		"path mismatch":   goLambdaOpenAPIModuleConfig("github.com/example/svc/other", "client/example", false),
		"shell quote":     goLambdaOpenAPIModuleConfig("github.com/x'/client/example", "client/example", false),
	} {
		t.Run(name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := cfg.Render(); err == nil {
				t.Fatal("Render succeeded, want an error")
			}
		})
	}
}

func TestGoLambdaOpenAPIModuleRequiresEnable(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := goLambdaOpenAPIModuleConfig("github.com/example/svc/client/example", "client/example", false)
	cfg.OpenAPI.Enable = false
	if err := cfg.Render(); err == nil {
		t.Fatal("Render succeeded with module set and enable false")
	}
}

func TestGoLambdaOpenAPIModuleCheckClientWithoutPrivateModules(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := goLambdaOpenAPIModuleConfig("github.com/example/svc/client/example", "client/example", false)
	cfg.PrivateModules = ""
	if err := cfg.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	wf := goLambdaReadFile(t, goLambdaWorkflowPath)
	_, check, ok := strings.Cut(wf, "  check:")
	if !ok {
		t.Fatalf("workflow has no check job:\n%s", wf)
	}
	if i, j := strings.Index(check, "just check-client"), strings.Index(check, "run: just check\n"); i < 0 || j < 0 || i > j {
		t.Errorf("check job does not run check-client before just check:\n%s", check[:min(len(check), 600)])
	}
}

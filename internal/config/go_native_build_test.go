package config

import (
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type nativeBuildStep struct {
	ID   string `yaml:"id"`
	If   string `yaml:"if"`
	Run  string `yaml:"run"`
	Uses string `yaml:"uses"`
}

type nativeBuildWorkflow struct {
	Jobs map[string]struct {
		Steps []nativeBuildStep `yaml:"steps"`
	} `yaml:"jobs"`
}

func nativeBuildRenderLambda(t *testing.T, mutate func(*GoLambdaConfig)) nativeBuildWorkflow {
	t.Helper()
	goLambdaRenderOptions(t, mutate)
	var wf nativeBuildWorkflow
	if err := yaml.Unmarshal([]byte(goLambdaReadFile(t, goLambdaWorkflowPath)), &wf); err != nil {
		t.Fatalf("workflow does not parse: %v", err)
	}
	return wf
}

func nativeBuildStepIndex(steps []nativeBuildStep, match func(nativeBuildStep) bool) int {
	return slices.IndexFunc(steps, match)
}

// With services and artifacts together, only services get the native build.
func TestGoLambdaBuildServicesGatesNativeBuildToServices(t *testing.T) {
	wf := nativeBuildRenderLambda(t, func(c *GoLambdaConfig) {
		c.Services = []string{"svc", "mcp"}
		c.Artifacts = []string{"client-go"}
	})
	steps := wf.Jobs["build-services"].Steps
	build := nativeBuildStepIndex(steps, func(s nativeBuildStep) bool { return s.Run == `just build "${{ matrix.service }}"` })
	pkg := nativeBuildStepIndex(steps, func(s nativeBuildStep) bool { return s.Run == `just package-artifact "${{ matrix.service }}"` })
	if build < 0 || pkg < 0 || build > pkg {
		t.Fatalf("build-services steps = %+v, want just build then just package-artifact", steps)
	}
	if want := `contains(fromJSON('["svc","mcp"]'), matrix.service)`; steps[build].If != want {
		t.Errorf("just build if = %q, want %q", steps[build].If, want)
	}
	if steps[pkg].If != "" {
		t.Errorf("just package-artifact if = %q, want none", steps[pkg].If)
	}
}

// Services alone need no gate on the native build.
func TestGoLambdaBuildServicesWithoutArtifactsHasNoGate(t *testing.T) {
	wf := nativeBuildRenderLambda(t, func(c *GoLambdaConfig) { c.Services = []string{"svc"} })
	for _, s := range wf.Jobs["build-services"].Steps {
		if s.If != "" && !strings.HasPrefix(s.Run, "rm -f") {
			t.Errorf("step %q has if %q", s.Run, s.If)
		}
	}
}

// Build jobs run the generators before the token is on disk, as check does.
func TestGoLambdaBuildJobsRunGenerateBeforeToken(t *testing.T) {
	wf := nativeBuildRenderLambda(t, func(c *GoLambdaConfig) {
		c.Generate = []string{"echo gen"}
		c.PrivateModules = "github.com/ALT-F4-LLC/quirk-service-kit"
		c.Services = []string{"svc"}
	})
	for _, job := range []string{"build", "build-services"} {
		steps := wf.Jobs[job].Steps
		gen := nativeBuildStepIndex(steps, func(s nativeBuildStep) bool { return s.Run == "just generate" })
		token := nativeBuildStepIndex(steps, func(s nativeBuildStep) bool { return s.ID == "generate-token" })
		build := nativeBuildStepIndex(steps, func(s nativeBuildStep) bool { return strings.HasPrefix(s.Run, "just build ") })
		if gen < 0 || token < 0 || build < 0 || gen > token || gen > build {
			t.Errorf("%s steps = %+v, want just generate before the token and just build", job, steps)
		}
	}
}

func TestGoLambdaBuildJobsOmitGenerateWhenUnset(t *testing.T) {
	wf := nativeBuildRenderLambda(t, func(c *GoLambdaConfig) { c.Services = []string{"svc"} })
	for _, job := range []string{"build", "build-services"} {
		for _, s := range wf.Jobs[job].Steps {
			if s.Run == "just generate" {
				t.Errorf("%s runs just generate with no generate config", job)
			}
		}
	}
}

func TestGoCobraCliCIPackagesAfterBuild(t *testing.T) {
	goCobraCliRenderTemp(t, nil)
	var wf nativeBuildWorkflow
	if err := yaml.Unmarshal([]byte(goCobraCliReadFile(t, ".github/workflows/ci.yaml")), &wf); err != nil {
		t.Fatalf("workflow does not parse: %v", err)
	}
	steps := wf.Jobs["build"].Steps
	build := nativeBuildStepIndex(steps, func(s nativeBuildStep) bool { return s.Run == "just build" })
	pkg := nativeBuildStepIndex(steps, func(s nativeBuildStep) bool { return s.Run == "just package" })
	if build < 0 || pkg < 0 || build > pkg {
		t.Errorf("build job steps = %+v, want just build then just package", steps)
	}
}

func TestGoCobraCliJustfileExportsGoprivate(t *testing.T) {
	t.Run("off", func(t *testing.T) {
		goCobraCliRenderTemp(t, nil)
		if got := goCobraCliReadFile(t, "justfile"); strings.Contains(got, "GOPRIVATE") {
			t.Errorf("justfile sets GOPRIVATE with no private modules:\n%s", got)
		}
	})
	t.Run("on", func(t *testing.T) {
		goCobraCliRenderTemp(t, map[string]any{"privateModules": "github.com/ALT-F4-LLC/*"})
		got := goCobraCliReadFile(t, "justfile")
		if want := "export GOPRIVATE := 'github.com/ALT-F4-LLC/*'\n"; !strings.Contains(got, want) {
			t.Errorf("justfile lacks %q:\n%s", want, got)
		}
	})
}

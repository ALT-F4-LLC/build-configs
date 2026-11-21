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

// The single build job packages every service and artifact with Vorpal, one
// step each, and runs no native build.
func TestGoLambdaBuildPackagesServicesWithoutBuild(t *testing.T) {
	for name, artifacts := range map[string][]string{"services only": nil, "services and artifacts": {"client-go"}} {
		t.Run(name, func(t *testing.T) {
			wf := nativeBuildRenderLambda(t, func(c *GoLambdaConfig) {
				c.Services = []string{"svc", "mcp"}
				c.Artifacts = artifacts
			})
			if _, ok := wf.Jobs["build-services"]; ok {
				t.Error("build-services job rendered")
			}
			steps := wf.Jobs["build"].Steps
			if nativeBuildStepIndex(steps, func(s nativeBuildStep) bool { return strings.HasPrefix(s.Run, "just build ") }) >= 0 {
				t.Errorf("build runs just build: %+v", steps)
			}
			for _, entry := range append([]string{"svc", "mcp"}, artifacts...) {
				if nativeBuildStepIndex(steps, func(s nativeBuildStep) bool { return s.Run == `just package-artifact "`+entry+`"` }) < 0 {
					t.Errorf("build steps = %+v, want just package-artifact %q", steps, entry)
				}
			}
			for _, s := range steps {
				if s.If != "" && !strings.HasPrefix(s.Run, "rm -f") {
					t.Errorf("step %q has if %q, want none", s.Run, s.If)
				}
			}
		})
	}
}

// Build jobs run the generators before the token is on disk, as check does.
func TestGoLambdaBuildJobsRunGenerateBeforeToken(t *testing.T) {
	wf := nativeBuildRenderLambda(t, func(c *GoLambdaConfig) {
		c.Generate = []string{"echo gen"}
		c.PrivateModules = "github.com/ALT-F4-LLC/quirk-service-kit"
		c.Services = []string{"svc"}
	})
	for _, job := range []string{"build"} {
		steps := wf.Jobs[job].Steps
		gen := nativeBuildStepIndex(steps, func(s nativeBuildStep) bool { return s.Run == "just generate" })
		token := nativeBuildStepIndex(steps, func(s nativeBuildStep) bool { return s.ID == "generate-token" })
		build := nativeBuildStepIndex(steps, func(s nativeBuildStep) bool {
			return strings.HasPrefix(s.Run, "just package ") || strings.HasPrefix(s.Run, "just package-artifact ")
		})
		if gen < 0 || token < 0 || build < 0 || gen > token || gen > build {
			t.Errorf("%s steps = %+v, want just generate before the token and the package step", job, steps)
		}
	}
}

func TestGoLambdaBuildJobsOmitGenerateWhenUnset(t *testing.T) {
	wf := nativeBuildRenderLambda(t, func(c *GoLambdaConfig) { c.Services = []string{"svc"} })
	for _, job := range []string{"build"} {
		for _, s := range wf.Jobs[job].Steps {
			if s.Run == "just generate" {
				t.Errorf("%s runs just generate with no generate config", job)
			}
		}
	}
}

func TestGoCobraCliCIPackagesWithoutBuild(t *testing.T) {
	goCobraCliRenderTemp(t, nil)
	var wf nativeBuildWorkflow
	if err := yaml.Unmarshal([]byte(goCobraCliReadFile(t, ".github/workflows/ci.yaml")), &wf); err != nil {
		t.Fatalf("workflow does not parse: %v", err)
	}
	steps := wf.Jobs["build"].Steps
	build := nativeBuildStepIndex(steps, func(s nativeBuildStep) bool { return s.Run == "just build" })
	pkg := nativeBuildStepIndex(steps, func(s nativeBuildStep) bool { return s.Run == "just package" })
	if build >= 0 || pkg < 0 {
		t.Errorf("build job steps = %+v, want just package without just build", steps)
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

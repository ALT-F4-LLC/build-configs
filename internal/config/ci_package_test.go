package config

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var ciPackageRecipe = regexp.MustCompile(`(?m)^([a-z][a-z0-9-]*)(?:[ :])`)

type ciPackageWorkflow struct {
	Jobs map[string]struct {
		Steps []struct {
			Run string `yaml:"run"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

// ciPackageRunsRecipe reports whether any command in run, a possibly
// multi-line script, invokes the just recipe name with or without arguments.
func ciPackageRunsRecipe(run, name string) bool {
	for _, field := range strings.FieldsFunc(run, func(r rune) bool { return r == '\n' || r == ';' || r == '&' || r == '|' }) {
		words := strings.Fields(field)
		if len(words) >= 2 && words[0] == "just" && words[1] == name {
			return true
		}
	}
	return false
}

// TestCIWorkflowTemplatesOnlyTerraformModuleRunsBuild scans the workflow
// template sources, so every conditional branch is covered.
func TestCIWorkflowTemplatesOnlyTerraformModuleRunsBuild(t *testing.T) {
	paths, err := filepath.Glob("../templates/templates/*/.github__workflows__*.yaml")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no workflow templates found: %v", err)
	}
	for _, path := range paths {
		src, err := os.ReadFile(path) //nolint:gosec // reads a template from this repository
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		allowsBuild := filepath.Base(filepath.Dir(path)) == TerraformModuleName
		for i, line := range strings.Split(string(src), "\n") {
			if run, ok := strings.CutPrefix(strings.TrimLeft(strings.TrimSpace(line), "- "), "run:"); ok &&
				ciPackageRunsRecipe(strings.TrimSpace(run), "build") && !allowsBuild {
				t.Errorf("%s:%d runs just build: %s", path, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// TestCIPackageAndBuildRecipesMatchWorkflows checks that package workflows
// avoid redundant builds, while terraform-module runs build for validation.
func TestCIPackageAndBuildRecipesMatchWorkflows(t *testing.T) {
	for _, tc := range []struct {
		name   string
		render func(t *testing.T)
		build  bool
	}{
		{name: GoCobraCliName, render: func(t *testing.T) { goCobraCliRenderTemp(t, goCobraCliPrivateModules) }},
		{name: GoLambdaName, render: func(t *testing.T) {
			t.Helper()
			t.Chdir(t.TempDir())
			cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
			cfg.Lambdas = []string{"api"}
			cfg.Services = []string{"mcp"}
			cfg.Artifacts = []string{"client-go"}
			if err := cfg.Render(); err != nil {
				t.Fatalf("Render: %v", err)
			}
		}},
		{name: GoServiceName, render: func(t *testing.T) {
			t.Helper()
			cfg := goServiceBotConfig()
			cfg.Artifacts = []string{"extra"}
			goServiceRender(t, cfg)
		}},
		{name: GoLibraryName},
		{name: TerraformName},
		{name: TerraformModuleName, build: true},
		{name: TypeScriptFrontendName},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.render != nil {
				tc.render(t)
			} else {
				t.Chdir(t.TempDir())
				tpl, err := Config{Name: "example", Template: tc.name}.GetTemplater()
				if err != nil {
					t.Fatalf("GetTemplater: %v", err)
				}
				if err := tpl.Render(); err != nil {
					t.Fatalf("Render: %v", err)
				}
			}

			var recipes []string
			for _, m := range ciPackageRecipe.FindAllStringSubmatch(goLambdaReadFile(t, "justfile"), -1) {
				recipes = append(recipes, m[1])
			}
			if !slices.Contains(recipes, "build") || !slices.Contains(recipes, "package") {
				t.Skipf("justfile recipes = %q: the rule applies only with both build and package", recipes)
			}

			workflows, err := filepath.Glob(".github/workflows/*.yaml")
			if err != nil || len(workflows) == 0 {
				t.Fatalf("no rendered workflows: %v", err)
			}
			for _, path := range workflows {
				src, err := os.ReadFile(path) //nolint:gosec // reads a file the test rendered into its own temp dir
				if err != nil {
					t.Fatalf("read %s: %v", path, err)
				}
				var wf ciPackageWorkflow
				if err := yaml.Unmarshal(src, &wf); err != nil {
					t.Fatalf("%s does not parse: %v", path, err)
				}
				hasBuild := false
				for _, job := range wf.Jobs {
					for _, step := range job.Steps {
						if ciPackageRunsRecipe(step.Run, "build") {
							hasBuild = true
						}
					}
				}
				if hasBuild != tc.build {
					t.Errorf("%s workflow runs just build = %t, want %t", path, hasBuild, tc.build)
				}
			}
		})
	}
}

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

// TestCIWorkflowTemplatesNeverRunBuild scans the workflow template sources, so
// every conditional branch is covered, not only the configs the render test
// below uses.
func TestCIWorkflowTemplatesNeverRunBuild(t *testing.T) {
	paths, err := filepath.Glob("../templates/templates/*/.github__workflows__*.yaml")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no workflow templates found: %v", err)
	}
	for _, path := range paths {
		src, err := os.ReadFile(path) //nolint:gosec // reads a template from this repository
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if run, ok := strings.CutPrefix(strings.TrimLeft(strings.TrimSpace(line), "- "), "run:"); ok && ciPackageRunsRecipe(strings.TrimSpace(run), "build") {
				t.Errorf("%s:%d runs just build: %s", path, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// TestCIRunsPackageWithoutBuild renders every template and checks that, when
// its justfile defines both build and package, no CI job runs `just build`:
// the package recipes compile through Vorpal on their own, so the native
// build would be repeated work.
func TestCIRunsPackageWithoutBuild(t *testing.T) {
	for _, tc := range []struct {
		name   string
		render func(t *testing.T)
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
		{name: TerraformModuleName},
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
				for name, job := range wf.Jobs {
					for _, step := range job.Steps {
						if ciPackageRunsRecipe(step.Run, "build") {
							t.Errorf("%s job %s runs %q; want the package recipes only", path, name, step.Run)
						}
					}
				}
			}
		})
	}
}

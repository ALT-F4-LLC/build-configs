package config

import (
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const ciNetrcCleanup = "rm -f ~/.netrc"

type ciNetrcStep struct {
	If   string `yaml:"if"`
	Run  string `yaml:"run"`
	Uses string `yaml:"uses"`
}

type ciNetrcWorkflow struct {
	Jobs map[string]struct {
		Steps []ciNetrcStep `yaml:"steps"`
	} `yaml:"jobs"`
}

// TestCINetrcCredentialStaysInHomeNetrc renders the go-cobra-cli, go-lambda and go-service
// workflows with private modules and checks that the GitHub App credential
// lives only in the ~/.netrc that extractions/netrc writes: no run step names
// a .netrc file except the cleanup, and each build job removes ~/.netrc on
// every outcome after writing it.
func TestCINetrcCredentialStaysInHomeNetrc(t *testing.T) {
	for _, tc := range []struct {
		name   string
		render func(t *testing.T)
		jobs   []string
	}{
		{
			name:   "go-cobra-cli",
			render: func(t *testing.T) { goCobraCliRenderTemp(t, goCobraCliPrivateModules) },
			jobs:   []string{"build", "check"},
		},
		{
			name: "go-lambda",
			render: func(t *testing.T) {
				t.Helper()
				t.Chdir(t.TempDir())
				cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
				cfg.Lambdas = []string{"api"}
				cfg.Services = []string{"mcp"}
				cfg.PrivateModules = "github.com/ALT-F4-LLC/quirk-service-kit"
				if err := cfg.Render(); err != nil {
					t.Fatalf("Render: %v", err)
				}
			},
			jobs: []string{"build"},
		},
		{
			name: "go-service",
			render: func(t *testing.T) {
				t.Helper()
				goServiceRender(t, goServiceBotConfig())
			},
			jobs: []string{"build", "check"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.render(t)
			src := goLambdaReadFile(t, ".github/workflows/ci.yaml")
			var wf ciNetrcWorkflow
			if err := yaml.Unmarshal([]byte(src), &wf); err != nil {
				t.Fatalf("workflow does not parse: %v\n%s", err, src)
			}

			cleanup := ciNetrcStep{If: "always()", Run: ciNetrcCleanup}
			writers := map[string]int{}
			for name, job := range wf.Jobs {
				for i, step := range job.Steps {
					if strings.Contains(step.Run, ".netrc") && step.Run != ciNetrcCleanup {
						t.Errorf("job %s step %q names a .netrc file other than %q", name, step.Run, ciNetrcCleanup)
					}
					if !strings.HasPrefix(step.Uses, "extractions/netrc@") {
						continue
					}
					writers[name]++
					if !slices.Contains(job.Steps[i+1:], cleanup) {
						t.Errorf("job %s step %d (%s) is not followed by %+v: %+v", name, i, step.Uses, cleanup, job.Steps)
					}
				}
			}

			for _, name := range tc.jobs {
				if _, ok := wf.Jobs[name]; !ok {
					t.Fatalf("job %s missing", name)
				}
				if writers[name] == 0 {
					t.Errorf("job %s has no extractions/netrc step: %+v", name, wf.Jobs[name].Steps)
				}
			}
		})
	}
}

// TestCINetrcTypeScriptFrontendWritesNoCredential renders the
// typescript-frontend workflow, whose dependencies come from the public npm
// registry, in its default and quirk-frontend-portal shapes, and checks that
// no job mints the GitHub App token, writes a .netrc or reads a secret.
func TestCINetrcTypeScriptFrontendWritesNoCredential(t *testing.T) {
	for name, cfg := range map[string]TypeScriptFrontendConfig{
		"default": NewTypeScriptFrontendConfig(Config{Name: "example", Template: TypeScriptFrontendName}),
		"portal":  typeScriptFrontendPortalConfig(),
	} {
		t.Run(name, func(t *testing.T) {
			typeScriptFrontendRender(t, cfg)
			src := goLambdaReadFile(t, ".github/workflows/ci.yaml")
			var wf ciNetrcWorkflow
			if err := yaml.Unmarshal([]byte(src), &wf); err != nil {
				t.Fatalf("workflow does not parse: %v\n%s", err, src)
			}
			for job, j := range wf.Jobs {
				for _, step := range j.Steps {
					if strings.HasPrefix(step.Uses, "extractions/netrc@") || strings.HasPrefix(step.Uses, "actions/create-github-app-token@") || strings.Contains(step.Run, ".netrc") {
						t.Errorf("job %s step %+v handles a credential", job, step)
					}
				}
			}
			if strings.Contains(src, "secrets.") {
				t.Errorf("workflow reads a secret:\n%s", src)
			}
		})
	}
}

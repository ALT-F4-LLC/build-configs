package config

import (
	"os"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type scanJobsStep struct {
	If   string         `yaml:"if"`
	Run  string         `yaml:"run"`
	Uses string         `yaml:"uses"`
	With map[string]any `yaml:"with"`
}

type scanJobsWorkflow struct {
	Jobs map[string]struct {
		Steps []scanJobsStep `yaml:"steps"`
	} `yaml:"jobs"`
}

// scanJobsTemplates lists every template whose justfile has a scan recipe,
// the workflow that runs them, and the dev tool each scan job needs.
var scanJobsTemplates = []struct {
	template string
	workflow string
	jobs     map[string]string
}{
	{GoCobraCliName, ".github/workflows/ci.yaml", map[string]string{"doc-validate": "rumdl", "secret-scan": "gitleaks", "vuln-scan": "govulncheck"}},
	{GoLambdaName, ".github/workflows/ci.yaml", map[string]string{"doc-validate": "rumdl", "secret-scan": "gitleaks", "vuln-scan": "govulncheck"}},
	{GoLibraryName, ".github/workflows/ci.yaml", map[string]string{"doc-validate": "rumdl", "secret-scan": "gitleaks", "vuln-scan": "govulncheck"}},
	{TerraformName, ".github/workflows/terraform.yaml", map[string]string{"secret-scan": "gitleaks"}},
	{TerraformModuleName, ".github/workflows/terraform.yaml", map[string]string{"secret-scan": "gitleaks"}},
}

func scanJobsRender(t *testing.T, template, workflow string, parameters map[string]any) scanJobsWorkflow {
	t.Helper()
	t.Chdir(t.TempDir())

	tpl, err := Config{Name: "demo", Template: template, Parameters: parameters}.GetTemplater()
	if err != nil {
		t.Fatalf("GetTemplater: %v", err)
	}
	if err := tpl.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	src, err := os.ReadFile(workflow) //nolint:gosec // reads a file the test rendered into its own temp dir
	if err != nil {
		t.Fatalf("%s was not written: %v", workflow, err)
	}
	var wf scanJobsWorkflow
	if err := yaml.Unmarshal(src, &wf); err != nil {
		t.Fatalf("%s does not parse as YAML: %v\n%s", workflow, err, src)
	}
	return wf
}

// Each scan job runs its own recipe, and the secret scan checks out the full
// history so gitleaks sees every commit.
func TestScanJobsRunTheirRecipes(t *testing.T) {
	for _, tc := range scanJobsTemplates {
		t.Run(tc.template, func(t *testing.T) {
			wf := scanJobsRender(t, tc.template, tc.workflow, nil)
			for job := range tc.jobs {
				steps := wf.Jobs[job].Steps
				if !slices.ContainsFunc(steps, func(s scanJobsStep) bool {
					return s.Run == "just "+job
				}) {
					t.Errorf("job %s does not run just %s: %+v", job, job, steps)
				}
			}
			for _, step := range wf.Jobs["secret-scan"].Steps {
				if strings.HasPrefix(step.Uses, "actions/checkout@") && step.With["fetch-depth"] != 0 {
					t.Errorf("secret-scan checkout fetch-depth = %v, want 0", step.With["fetch-depth"])
				}
			}
		})
	}
}

// A scan job renders only when the development environment carries its tool,
// at any version, so excluding a default does not leave a job that cannot run.
func TestScanJobsFollowDevTools(t *testing.T) {
	for _, tc := range scanJobsTemplates {
		t.Run(tc.template, func(t *testing.T) {
			var exclude []any
			for _, tool := range tc.jobs {
				exclude = append(exclude, tool)
			}
			wf := scanJobsRender(t, tc.template, tc.workflow, map[string]any{
				"vorpal": map[string]any{"excludeTools": exclude},
			})
			for job := range tc.jobs {
				if _, ok := wf.Jobs[job]; ok {
					t.Errorf("job %s renders with its tool excluded", job)
				}
			}

			var readd []any
			for _, tool := range tc.jobs {
				readd = append(readd, tool+":0.0.1")
			}
			wf = scanJobsRender(t, tc.template, tc.workflow, map[string]any{
				"vorpal": map[string]any{"excludeTools": exclude, "tools": readd},
			})
			for job := range tc.jobs {
				if _, ok := wf.Jobs[job]; !ok {
					t.Errorf("job %s is missing with its tool re-added at another version", job)
				}
			}
		})
	}
}

// With private modules, vuln-scan writes the App token before the scan and
// always removes it afterward.
func TestScanJobsVulnScanCleansUpPrivateModuleToken(t *testing.T) {
	for _, template := range []string{GoCobraCliName, GoLambdaName} {
		t.Run(template, func(t *testing.T) {
			wf := scanJobsRender(t, template, ".github/workflows/ci.yaml", map[string]any{
				"privateModules": "github.com/ALT-F4-LLC/private",
			})
			netrc, scan, cleanup := -1, -1, -1
			for i, step := range wf.Jobs["vuln-scan"].Steps {
				switch {
				case strings.HasPrefix(step.Uses, "extractions/netrc@"):
					netrc = i
				case step.Run == "just vuln-scan":
					scan = i
				case step.Run == "rm -f ~/.netrc" && step.If == "always()":
					cleanup = i
				}
			}
			if netrc < 0 || scan < netrc || cleanup < scan {
				t.Errorf("vuln-scan steps netrc=%d scan=%d cleanup=%d, want netrc < scan < cleanup: %+v",
					netrc, scan, cleanup, wf.Jobs["vuln-scan"].Steps)
			}
		})
	}
}

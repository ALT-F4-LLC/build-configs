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
		Needs scanJobsNeeds  `yaml:"needs"`
		Steps []scanJobsStep `yaml:"steps"`
	} `yaml:"jobs"`
}

// scanJobsNeeds accepts a job's needs as a single name or a list of names.
type scanJobsNeeds []string

func (n *scanJobsNeeds) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		*n = []string{node.Value}
		return nil
	}
	var list []string
	if err := node.Decode(&list); err != nil {
		return err
	}
	*n = list
	return nil
}

// scanJobsCheckNeeds fails when a job needs a job the workflow does not define.
func scanJobsCheckNeeds(t *testing.T, wf scanJobsWorkflow) {
	t.Helper()
	for name, job := range wf.Jobs {
		for _, need := range job.Needs {
			if _, ok := wf.Jobs[need]; !ok {
				t.Errorf("job %s needs %s, which does not render", name, need)
			}
		}
	}
}

// scanJobsTemplates lists every template whose justfile has a scan recipe,
// the workflow that runs them, and the dev tool each scan recipe needs.
var scanJobsTemplates = []struct {
	template string
	workflow string
	jobs     map[string]string
}{
	{GoCobraCliName, ".github/workflows/ci.yaml", map[string]string{"doc-validate": "rumdl", "secret-scan": "gitleaks", "vuln-scan": "govulncheck"}},
	{GoLambdaName, ".github/workflows/ci.yaml", map[string]string{"doc-validate": "rumdl", "secret-scan": "gitleaks", "vuln-scan": "govulncheck"}},
	{GoServiceName, ".github/workflows/ci.yaml", map[string]string{"doc-validate": "rumdl", "secret-scan": "gitleaks", "vuln-scan": "govulncheck"}},
	{GoLibraryName, ".github/workflows/ci.yaml", map[string]string{"doc-validate": "rumdl", "secret-scan": "gitleaks", "vuln-scan": "govulncheck"}},
	{TerraformName, ".github/workflows/terraform.yaml", map[string]string{"secret-scan": "gitleaks"}},
	{TerraformModuleName, ".github/workflows/terraform.yaml", map[string]string{"secret-scan": "gitleaks"}},
	{TypeScriptFrontendName, ".github/workflows/ci.yaml", map[string]string{"doc-validate": "rumdl", "secret-scan": "gitleaks"}},
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

// scanJobsCheckRuns returns the run commands of the check job.
func scanJobsCheckRuns(wf scanJobsWorkflow) []string {
	var runs []string
	for _, s := range wf.Jobs["check"].Steps {
		if s.Run != "" {
			runs = append(runs, s.Run)
		}
	}
	return runs
}

// Every check runs as a step of the one check job: no scan recipe has a job
// of its own, and no other job runs a scan recipe. The secret scan needs the
// full history, so the check job's checkout fetches it.
func TestScanJobsRunInTheCheckJob(t *testing.T) {
	for _, tc := range scanJobsTemplates {
		t.Run(tc.template, func(t *testing.T) {
			wf := scanJobsRender(t, tc.template, tc.workflow, nil)
			runs := scanJobsCheckRuns(wf)
			for recipe := range tc.jobs {
				if _, ok := wf.Jobs[recipe]; ok {
					t.Errorf("job %s renders; its recipe belongs in the check job", recipe)
				}
				if !slices.Contains(runs, "just "+recipe) {
					t.Errorf("check job does not run just %s: %q", recipe, runs)
				}
			}
			for name, job := range wf.Jobs {
				if name == "check" {
					continue
				}
				for _, s := range job.Steps {
					if _, ok := tc.jobs[strings.TrimPrefix(s.Run, "just ")]; ok || s.Run == "just check" || s.Run == "just lint" {
						t.Errorf("job %s runs %q, which belongs in the check job", name, s.Run)
					}
				}
			}
			for _, step := range wf.Jobs["check"].Steps {
				if strings.HasPrefix(step.Uses, "actions/checkout@") && step.With["fetch-depth"] != 0 {
					t.Errorf("check checkout fetch-depth = %v, want 0 for the secret scan", step.With["fetch-depth"])
				}
			}
		})
	}
}

// A scan step renders only when the development environment carries its
// tool, at any version, so excluding a default does not leave a step that
// cannot run.
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
			runs := scanJobsCheckRuns(wf)
			for recipe := range tc.jobs {
				if slices.Contains(runs, "just "+recipe) {
					t.Errorf("check runs just %s with its tool excluded", recipe)
				}
			}
			for _, step := range wf.Jobs["check"].Steps {
				if _, ok := step.With["fetch-depth"]; ok && strings.HasPrefix(step.Uses, "actions/checkout@") {
					t.Errorf("check checkout fetches full history without the secret scan: %+v", step)
				}
			}
			scanJobsCheckNeeds(t, wf)

			var readd []any
			for _, tool := range tc.jobs {
				readd = append(readd, tool+":0.0.1")
			}
			wf = scanJobsRender(t, tc.template, tc.workflow, map[string]any{
				"vorpal": map[string]any{"excludeTools": exclude, "tools": readd},
			})
			runs = scanJobsCheckRuns(wf)
			for recipe := range tc.jobs {
				if !slices.Contains(runs, "just "+recipe) {
					t.Errorf("check does not run just %s with its tool re-added at another version", recipe)
				}
			}
			scanJobsCheckNeeds(t, wf)
		})
	}
}

// With private modules, the check job runs the credential-free scans before
// it mints the App token, writes the token before the vuln scan, and always
// removes it afterward.
func TestScanJobsVulnScanCleansUpPrivateModuleToken(t *testing.T) {
	for _, template := range []string{GoCobraCliName, GoLambdaName, GoServiceName} {
		t.Run(template, func(t *testing.T) {
			wf := scanJobsRender(t, template, ".github/workflows/ci.yaml", map[string]any{
				"privateModules": "github.com/ALT-F4-LLC/private",
			})
			token, netrc, scan, cleanup := -1, -1, -1, -1
			var credentialFree []int
			for i, step := range wf.Jobs["check"].Steps {
				switch {
				case strings.HasPrefix(step.Uses, "actions/create-github-app-token@"):
					token = i
				case strings.HasPrefix(step.Uses, "extractions/netrc@"):
					netrc = i
				case step.Run == "just secret-scan" || step.Run == "just doc-validate":
					credentialFree = append(credentialFree, i)
				case step.Run == "just vuln-scan":
					scan = i
				case step.Run == "rm -f ~/.netrc" && step.If == "always()":
					cleanup = i
				}
			}
			if netrc < 0 || scan < netrc || cleanup < scan {
				t.Errorf("check steps netrc=%d scan=%d cleanup=%d, want netrc < scan < cleanup: %+v",
					netrc, scan, cleanup, wf.Jobs["check"].Steps)
			}
			if len(credentialFree) != 2 {
				t.Errorf("check runs %d of secret-scan and doc-validate, want 2", len(credentialFree))
			}
			for _, i := range credentialFree {
				if token < 0 || i > token {
					t.Errorf("step %q runs at %d, after the App token at %d", wf.Jobs["check"].Steps[i].Run, i, token)
				}
			}
		})
	}
}

// Without private modules, go-lambda's check job runs `just check`, whose
// recipe runs the generators, before the vuln scan that needs their output.
func TestScanJobsGoLambdaWithoutPrivateModulesOrdersChecks(t *testing.T) {
	wf := scanJobsRender(t, GoLambdaName, ".github/workflows/ci.yaml", map[string]any{
		"privateModules": "",
		"generate":       []any{"just gen"},
		"openapi":        map[string]any{"enable": true, "generateInCI": true},
	})
	want := []string{"just secret-scan", "just doc-validate", "just check", "just vuln-scan"}
	if got := scanJobsCheckRuns(wf); !slices.Equal(got, want) {
		t.Errorf("check runs %q, want %q", got, want)
	}
}

// The jobs that build, test, package, or plan wait on the check job, which
// runs every scan, so a failed scan stops the pipeline before it ships.
func TestScanJobsGateDownstreamJobs(t *testing.T) {
	cases := []struct {
		template   string
		workflow   string
		parameters map[string]any
		gated      []string
	}{
		{GoCobraCliName, ".github/workflows/ci.yaml", nil, []string{"build"}},
		{GoLambdaName, ".github/workflows/ci.yaml", nil, []string{"build"}},
		{GoLambdaName, ".github/workflows/ci.yaml", map[string]any{"services": []any{"mcp"}}, []string{"build"}},
		{GoServiceName, ".github/workflows/ci.yaml", nil, []string{"build"}},
		{GoServiceName, ".github/workflows/ci.yaml", map[string]any{"image": map[string]any{"repository": "quirk/x", "roleArn": "arn:aws:iam::677459762413:role/x"}}, []string{"build"}},
		{GoLibraryName, ".github/workflows/ci.yaml", nil, []string{"test"}},
		{TerraformName, ".github/workflows/terraform.yaml", nil, []string{"plan"}},
		{TerraformModuleName, ".github/workflows/terraform.yaml", nil, []string{"build"}},
		{TypeScriptFrontendName, ".github/workflows/ci.yaml", nil, []string{"build"}},
	}
	for _, tc := range cases {
		t.Run(tc.template+"/"+strings.Join(tc.gated, "+"), func(t *testing.T) {
			wf := scanJobsRender(t, tc.template, tc.workflow, tc.parameters)
			for _, name := range tc.gated {
				job, ok := wf.Jobs[name]
				if !ok {
					t.Fatalf("job %s does not render", name)
				}
				if !slices.Equal(job.Needs, scanJobsNeeds{"check"}) {
					t.Errorf("job %s needs = %q, want [check]", name, job.Needs)
				}
			}
			scanJobsCheckNeeds(t, wf)
		})
	}
}

// typescript-frontend's vuln-scan runs `bun audit`, and bun comes from the
// TypeScript development environment rather than a listed tool, so the scan
// stays in the check job after `just check` even with every default excluded.
func TestScanJobsTypeScriptFrontendAuditNeedsNoListedTool(t *testing.T) {
	wf := scanJobsRender(t, TypeScriptFrontendName, ".github/workflows/ci.yaml", map[string]any{
		"vorpal": map[string]any{"excludeTools": []any{"gitleaks", "jq", "just", "rumdl"}},
	})
	if got, want := scanJobsCheckRuns(wf), []string{"just check", "just vuln-scan"}; !slices.Equal(got, want) {
		t.Errorf("check runs %q, want %q", got, want)
	}
	scanJobsCheckNeeds(t, wf)
}

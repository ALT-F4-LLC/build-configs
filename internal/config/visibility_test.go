package config

import (
	"os"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// visibilityTemplates lists every registered template with the workflow it
// renders.
var visibilityTemplates = []struct {
	template string
	workflow string
}{
	{GoCobraCliName, ".github/workflows/ci.yaml"},
	{"go-lambda", ".github/workflows/ci.yaml"},
	{"go-library", ".github/workflows/ci.yaml"},
	{"go-service", ".github/workflows/ci.yaml"},
	{TerraformName, ".github/workflows/terraform.yaml"},
	{"terraform-module", ".github/workflows/terraform.yaml"},
	{TypeScriptFrontendName, ".github/workflows/ci.yaml"},
}

type visibilityStep struct {
	Uses string            `yaml:"uses"`
	With map[string]string `yaml:"with"`
	Env  map[string]string `yaml:"env"`
	Run  string            `yaml:"run"`
}

type visibilityJob struct {
	Permissions map[string]string `yaml:"permissions"`
	RunsOn      string            `yaml:"runs-on"`
	Steps       []visibilityStep  `yaml:"steps"`
}

type visibilityWorkflow struct {
	Jobs map[string]visibilityJob `yaml:"jobs"`
}

// visibilityRender renders template in a temp dir and parses its workflow.
func visibilityRender(t *testing.T, cfg Config, workflow string) visibilityWorkflow {
	t.Helper()
	t.Chdir(t.TempDir())

	tpl, err := cfg.GetTemplater()
	if err != nil {
		t.Fatalf("GetTemplater: %v", err)
	}
	if err := tpl.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	b, err := os.ReadFile(workflow) //nolint:gosec // reads a file the test rendered into its own temp dir
	if err != nil {
		t.Fatalf("reading rendered %s: %v", workflow, err)
	}
	var wf visibilityWorkflow
	if err := yaml.Unmarshal(b, &wf); err != nil {
		t.Fatalf("%s does not parse as YAML: %v\n%s", workflow, err, b)
	}
	if len(wf.Jobs) == 0 {
		t.Fatalf("%s has no jobs", workflow)
	}
	return wf
}

// publicVisibilityTemplates is visibilityTemplates without the terraform
// template, which rejects public.
func publicVisibilityTemplates() []struct {
	template string
	workflow string
} {
	var out []struct {
		template string
		workflow string
	}
	for _, tc := range visibilityTemplates {
		if tc.template != TerraformName {
			out = append(out, tc)
		}
	}
	return out
}

const (
	visibilitySetupAction = "ALT-F4-LLC/setup-vorpal-action@0.2.1"
	visibilityRoleAction  = "aws-actions/configure-aws-credentials@v6"
	visibilityDemoRole    = "arn:aws:iam::677459762413:role/altf4llc-gha-demo-vorpal"
)

// assertVorpalSetup checks the two steps a hosted runner starts with: the
// OIDC role session, then the setup action, which takes its AWS credentials
// from the job environment the first step exports. The job also needs
// id-token: write for the OIDC token.
func assertVorpalSetup(t *testing.T, name string, job visibilityJob) {
	t.Helper()
	if len(job.Steps) < 2 {
		t.Fatalf("job %s has %d steps, want the role and setup steps first", name, len(job.Steps))
	}
	role, setup := job.Steps[0], job.Steps[1]
	if role.Uses != visibilityRoleAction {
		t.Errorf("job %s first step uses %q, want %q", name, role.Uses, visibilityRoleAction)
	}
	for key, want := range map[string]string{
		"audience":       "sts.amazonaws.com",
		"aws-region":     "us-west-2",
		"role-to-assume": visibilityDemoRole,
	} {
		if got := role.With[key]; got != want {
			t.Errorf("job %s role with.%s = %q, want %q", name, key, got, want)
		}
	}
	if setup.Uses != visibilitySetupAction {
		t.Errorf("job %s second step uses %q, want %q", name, setup.Uses, visibilitySetupAction)
	}
	for key, want := range map[string]string{
		"version":                    "0.6.0",
		"registry-backend":           "s3",
		"registry-backend-s3-bucket": "altf4llc-vorpal-registry",
	} {
		if got := setup.With[key]; got != want {
			t.Errorf("job %s setup with.%s = %q, want %q", name, key, got, want)
		}
	}
	if len(setup.Env) != 0 {
		t.Errorf("job %s setup step sets env %v; its AWS credentials come from the role step", name, setup.Env)
	}
	if job.Permissions["id-token"] != "write" {
		t.Errorf("job %s permissions = %v, want id-token: write for the OIDC token", name, job.Permissions)
	}
}

// A repository with no visibility key is private, so every template keeps the
// altf4llc runner group and installs nothing, which keeps its existing output.
func TestVisibilityDefaultsToPrivateRunnerOnEveryTemplate(t *testing.T) {
	for _, visibility := range []string{"", VisibilityPrivate} {
		for _, tc := range visibilityTemplates {
			t.Run(tc.template+"/"+visibility, func(t *testing.T) {
				wf := visibilityRender(t, Config{Name: "demo", Template: tc.template, Visibility: visibility}, tc.workflow)

				for name, job := range wf.Jobs {
					if job.RunsOn != "altf4llc" {
						t.Errorf("job %s runs-on = %q, want altf4llc", name, job.RunsOn)
					}
					for _, step := range job.Steps {
						if strings.HasPrefix(step.Uses, "ALT-F4-LLC/setup-vorpal-action") {
							t.Errorf("job %s installs Vorpal on the altf4llc runner, which carries it", name)
						}
					}
				}
			})
		}
	}
}

// A public repository runs on a GitHub-hosted runner, which has no Vorpal, so
// every job installs it before any step that runs through the Vorpal shell.
func TestPublicVisibilityUsesHostedRunnerWithVorpalSetupOnEveryTemplate(t *testing.T) {
	for _, tc := range publicVisibilityTemplates() {
		t.Run(tc.template, func(t *testing.T) {
			wf := visibilityRender(t, Config{Name: "demo", Template: tc.template, Visibility: VisibilityPublic}, tc.workflow)

			for name, job := range wf.Jobs {
				if job.RunsOn != "ubuntu-latest" {
					t.Errorf("job %s runs-on = %q, want ubuntu-latest", name, job.RunsOn)
				}
				assertVorpalSetup(t, name, job)
			}
		})
	}
}

// Options add jobs the default render lacks: go-lambda adds a deploy job per
// environment, and go-service adds a push job with an image. Each job runs on
// the runner and installs Vorpal first, and the go-lambda deploy job reads the
// runner from outside its range scope.
func TestPublicVisibilityCoversJobsAddedByOptions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		template string
		params   map[string]any
		wantJobs []string
	}{
		{
			"go-lambda deploy environments",
			"go-lambda",
			map[string]any{
				"lambdas": []any{"api"},
				"deploy": []any{
					map[string]any{"environment": "dev", "region": "us-west-2", "roleArn": "arn:aws:iam::111111111111:role/dev"},
					map[string]any{"environment": "prod", "region": "us-west-2", "roleArn": "arn:aws:iam::111111111111:role/prod", "githubEnvironment": "prod"},
				},
			},
			[]string{"build", "check", "deploy-dev", "deploy-prod"},
		},
		{
			"go-service image push",
			"go-service",
			map[string]any{
				"image": map[string]any{"repository": "demo", "roleArn": "arn:aws:iam::111111111111:role/push"},
			},
			[]string{"build", "check", "push"},
		},
		{
			"go-lambda private modules",
			"go-lambda",
			map[string]any{"lambdas": []any{"api"}, "privateModules": "github.com/ALT-F4-LLC/quirk-service-kit"},
			[]string{"build", "check"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wf := visibilityRender(t, Config{Name: "demo", Template: tc.template, Visibility: VisibilityPublic, Parameters: tc.params}, ".github/workflows/ci.yaml")

			var got []string
			for name, job := range wf.Jobs {
				got = append(got, name)
				if job.RunsOn != PublicRunner {
					t.Errorf("job %s runs-on = %q, want %q", name, job.RunsOn, PublicRunner)
				}
				assertVorpalSetup(t, name, job)
			}
			slices.Sort(got)
			if !slices.Equal(got, tc.wantJobs) {
				t.Errorf("jobs = %q, want %q", got, tc.wantJobs)
			}
		})
	}
}

// The terraform plan artifact holds cleartext variable and state values, and a
// public repository's artifacts are open to any signed-in GitHub user, so the
// template refuses public. The other templates keep accepting it.
func TestTerraformRejectsPublicVisibility(t *testing.T) {
	_, err := Config{Name: "demo", Template: TerraformName, Visibility: VisibilityPublic}.GetTemplater()
	if err == nil {
		t.Fatal("terraform accepted visibility public")
	}
	for _, want := range []string{"terraform", "public", "cleartext"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
	for _, tc := range publicVisibilityTemplates() {
		if _, err := (Config{Name: "demo", Template: tc.template, Visibility: VisibilityPublic}).GetTemplater(); err != nil {
			t.Errorf("%s rejected visibility public: %v", tc.template, err)
		}
	}
}

func TestVisibilityRejectsUnknownValue(t *testing.T) {
	for _, tc := range visibilityTemplates {
		_, err := Config{Name: "demo", Template: tc.template, Visibility: "internal"}.GetTemplater()
		if err == nil {
			t.Errorf("%s accepted visibility internal", tc.template)
		}
	}
}

// The parameter unmarshal promotes embedded Config fields, so a visibility
// under parameters would silently override the top-level key.
func TestVisibilityRejectsParameterSpelling(t *testing.T) {
	for _, key := range []string{"visibility", "Visibility", "VISIBILITY"} {
		cfg := Config{
			Name:       "demo",
			Template:   TerraformName,
			Visibility: VisibilityPrivate,
			Parameters: map[string]any{key: VisibilityPublic},
		}
		if _, err := cfg.GetTemplater(); err == nil {
			t.Errorf("GetTemplater accepted parameters.%s", key)
		}
	}
}

func TestVisibilityLoadsFromYAML(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte("name: demo\ntemplate: go-library\nvisibility: public\n"), &cfg); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Visibility != VisibilityPublic {
		t.Errorf("Visibility = %q, want %q", cfg.Visibility, VisibilityPublic)
	}
	tpl, err := cfg.GetTemplater()
	if err != nil {
		t.Fatalf("GetTemplater: %v", err)
	}
	if got := tpl.(GoLibraryConfig).Runner(); got != PublicRunner {
		t.Errorf("Runner() = %q, want %q", got, PublicRunner)
	}
}

// github.runsOn overrides what visibility picks. Any runner but the altf4llc
// group lacks Vorpal, so the setup step follows the resolved runner.
func TestTerraformRunsOnOverridesVisibility(t *testing.T) {
	for _, tc := range []struct {
		name       string
		visibility string
		runsOn     string
		wantRunner string
		wantSetup  bool
	}{
		{"private custom runner", VisibilityPrivate, "ubuntu-latest", "ubuntu-latest", true},
		{"private custom self-hosted runner", VisibilityPrivate, "custom-runner", "custom-runner", true},
		{"private pinned to altf4llc", VisibilityPrivate, "altf4llc", "altf4llc", false},
		{"private default", VisibilityPrivate, "", "altf4llc", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := map[string]any{}
			if tc.runsOn != "" {
				params["github"] = map[string]any{"runsOn": tc.runsOn}
			}
			wf := visibilityRender(t, Config{Name: "demo", Template: TerraformName, Visibility: tc.visibility, Parameters: params}, ".github/workflows/terraform.yaml")

			for name, job := range wf.Jobs {
				if job.RunsOn != tc.wantRunner {
					t.Errorf("job %s runs-on = %q, want %q", name, job.RunsOn, tc.wantRunner)
				}
				hasSetup := slices.ContainsFunc(job.Steps, func(s visibilityStep) bool {
					return s.Uses == visibilitySetupAction
				})
				if hasSetup != tc.wantSetup {
					t.Errorf("job %s has setup step = %v, want %v", name, hasSetup, tc.wantSetup)
				}
			}
		})
	}
}

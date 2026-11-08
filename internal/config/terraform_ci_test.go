package config

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const (
	terraformCIWorkflowPath = ".github/workflows/terraform.yaml"
	terraformCIForkGuard    = "github.event_name != 'pull_request' || github.event.pull_request.head.repo.full_name == github.repository"
	terraformCIVorpalShell  = "bash " + vorpalShell + " {0}"
)

type terraformCIWorkflow struct {
	Jobs map[string]terraformCIWorkflowJob `yaml:"jobs"`
}

type terraformCIWorkflowJob struct {
	Env         map[string]string         `yaml:"env"`
	If          string                    `yaml:"if"`
	Permissions map[string]string         `yaml:"permissions"`
	RunsOn      string                    `yaml:"runs-on"`
	Steps       []terraformCIWorkflowStep `yaml:"steps"`
}

type terraformCIWorkflowStep struct {
	Env   map[string]string `yaml:"env"`
	ID    string            `yaml:"id"`
	If    string            `yaml:"if"`
	Run   string            `yaml:"run"`
	Shell string            `yaml:"shell"`
	Uses  string            `yaml:"uses"`
	With  map[string]string `yaml:"with"`
}

func terraformCIParseWorkflow(t *testing.T) terraformCIWorkflow {
	t.Helper()
	src := terraformVorpalReadFile(t, terraformCIWorkflowPath)
	var wf terraformCIWorkflow
	if err := yaml.Unmarshal([]byte(src), &wf); err != nil {
		t.Fatalf("%s does not parse as YAML: %v\n%s", terraformCIWorkflowPath, err, src)
	}
	return wf
}

// terraformCIStepIndex returns the index of the first step in job that uses
// action (an owner/name ref without its version tag), or -1.
func terraformCIStepIndex(job terraformCIWorkflowJob, action string) int {
	return slices.IndexFunc(job.Steps, func(s terraformCIWorkflowStep) bool {
		return strings.HasPrefix(s.Uses, action+"@")
	})
}

// terraformCIIsDeployKeyCleanup reports whether step is the always() step in
// the runner's bash that removes the key file job's ssh-key-action step names,
// and only when that step succeeded, so a key it did not write survives.
func terraformCIIsDeployKeyCleanup(job terraformCIWorkflowJob, step terraformCIWorkflowStep) bool {
	key := terraformCIStepIndex(job, "shimataro/ssh-key-action")
	if key < 0 {
		return false
	}
	keyStep := job.Steps[key]
	if keyStep.ID == "" || keyStep.With["name"] == "" {
		return false
	}
	wantRun := `if [ "$KEY_OUTCOME" = success ]; then rm -f "$HOME/.ssh/` + keyStep.With["name"] + `"; fi`
	wantOutcome := "${{ steps." + keyStep.ID + ".outcome }}"
	return step.If == "always()" && step.Shell == "bash" && step.Run == wantRun &&
		maps.Equal(step.Env, map[string]string{"KEY_OUTCOME": wantOutcome})
}

func terraformCIRenderDeployKey(t *testing.T) {
	t.Helper()
	terraformVorpalRender(t, map[string]interface{}{
		"github": map[string]interface{}{
			"action": map[string]interface{}{"setupDeployKey": true},
		},
	})
}

var terraformCIRenders = []struct {
	name   string
	render func(t *testing.T)
}{
	{"default", func(t *testing.T) { terraformVorpalRender(t, nil) }},
	{"deploy key", terraformCIRenderDeployKey},
}

func TestTerraformDefaultGitHubRunsOnIsAltf4llc(t *testing.T) {
	if got := NewTerraformConfig(Config{Name: "demo"}).GitHub.RunsOn; got != "altf4llc" {
		t.Errorf("default github.runsOn = %q, want %q", got, "altf4llc")
	}
}

func TestTerraformWorkflowJobsRunOnAltf4llc(t *testing.T) {
	terraformVorpalRender(t, nil)

	wf := terraformCIParseWorkflow(t)

	if got := slices.Sorted(maps.Keys(wf.Jobs)); !slices.Equal(got, []string{"apply", "check", "plan"}) {
		t.Fatalf("jobs = %q, want [apply check plan]", got)
	}
	for name, job := range wf.Jobs {
		if job.RunsOn != "altf4llc" {
			t.Errorf("job %s runs-on = %q, want %q", name, job.RunsOn, "altf4llc")
		}
	}
}

func TestTerraformWorkflowJobsRunOnConfiguredRunner(t *testing.T) {
	terraformVorpalRender(t, map[string]interface{}{
		"github": map[string]interface{}{
			"env":    map[string]interface{}{"TF_VAR_stage": "prod"},
			"runsOn": "custom-runner",
		},
		"schedule": "0 6 * * 1",
	})

	wf := terraformCIParseWorkflow(t)

	if got := slices.Sorted(maps.Keys(wf.Jobs)); !slices.Equal(got, []string{"apply", "check", "plan"}) {
		t.Fatalf("jobs = %q, want [apply check plan]", got)
	}
	for name, job := range wf.Jobs {
		if job.RunsOn != "custom-runner" {
			t.Errorf("job %s runs-on = %q, want %q", name, job.RunsOn, "custom-runner")
		}
	}
	wantEnv := map[string]string{"TF_VAR_stage": "prod"}
	for _, name := range []string{"plan", "apply"} {
		if got := wf.Jobs[name].Env; !maps.Equal(got, wantEnv) {
			t.Errorf("job %s env = %v, want %v", name, got, wantEnv)
		}
	}
	if got, want := wf.Jobs["apply"].If, "${{ ("+terraformCIForkGuard+") && (github.ref == 'refs/heads/main') }}"; got != want {
		t.Errorf("job apply if =\n  %q\nwant\n  %q", got, want)
	}
}

func TestTerraformWorkflowJobConditionsCarryForkGuard(t *testing.T) {
	terraformVorpalRender(t, nil)

	wf := terraformCIParseWorkflow(t)

	want := map[string]string{
		"check": "${{ " + terraformCIForkGuard + " }}",
		"plan":  "${{ " + terraformCIForkGuard + " }}",
		"apply": "${{ (" + terraformCIForkGuard + ") && (github.ref == 'refs/heads/main') }}",
	}
	for name, cond := range want {
		if got := wf.Jobs[name].If; got != cond {
			t.Errorf("job %s if =\n  %q\nwant\n  %q", name, got, cond)
		}
	}
}

func TestTerraformWorkflowJobPermissionsDoNotWiden(t *testing.T) {
	terraformVorpalRender(t, nil)

	wf := terraformCIParseWorkflow(t)

	readOnly := map[string]string{"contents": "read"}
	assumesRole := map[string]string{"contents": "read", "id-token": "write"}
	want := map[string]map[string]string{
		"check": readOnly,
		"plan":  assumesRole,
		"apply": assumesRole,
	}
	for name, perms := range want {
		if got := wf.Jobs[name].Permissions; !maps.Equal(got, perms) {
			t.Errorf("job %s permissions = %v, want %v", name, got, perms)
		}
	}
}

// Credential-cleanup steps run in the runner's default bash and are exempt
// from the Vorpal shell requirement.
func TestTerraformWorkflowRunsJustThroughVorpalShellAfterCheckout(t *testing.T) {
	for _, tc := range terraformCIRenders {
		t.Run(tc.name, func(t *testing.T) {
			tc.render(t)

			wf := terraformCIParseWorkflow(t)

			wantRuns := map[string][]string{
				"check": {"just check"},
				"plan":  {"aws sts get-caller-identity", "just init", "just validate", "just plan"},
				"apply": {"aws sts get-caller-identity", "just init", "just apply"},
			}
			for name, want := range wantRuns {
				job := wf.Jobs[name]
				checkout := terraformCIStepIndex(job, "actions/checkout")
				if checkout < 0 {
					t.Errorf("job %s has no actions/checkout step: %+v", name, job.Steps)
					continue
				}
				var runs []string
				for i, step := range job.Steps {
					if step.Run == "" || terraformCIIsDeployKeyCleanup(job, step) {
						continue
					}
					runs = append(runs, step.Run)
					if step.Shell != terraformCIVorpalShell {
						t.Errorf("job %s step %q shell = %q, want %q", name, step.Run, step.Shell, terraformCIVorpalShell)
					}
					if i < checkout {
						t.Errorf("job %s step %q runs before checkout", name, step.Run)
					}
				}
				if !slices.Equal(runs, want) {
					t.Errorf("job %s runs %q, want %q", name, runs, want)
				}
			}
		})
	}
}

// actions/checkout writes the job token into .git/config unless
// persist-credentials is false; on the persistent self-hosted runner that file
// outlives the job.
func TestTerraformWorkflowCheckoutDoesNotPersistToken(t *testing.T) {
	for _, tc := range terraformCIRenders {
		t.Run(tc.name, func(t *testing.T) {
			tc.render(t)

			wf := terraformCIParseWorkflow(t)

			for name, job := range wf.Jobs {
				for _, step := range job.Steps {
					if !strings.HasPrefix(step.Uses, "actions/checkout@") {
						continue
					}
					if got := step.With["persist-credentials"]; got != "false" {
						t.Errorf("job %s %s persist-credentials = %q, want %q", name, step.Uses, got, "false")
					}
				}
			}
		})
	}
}

func TestTerraformWorkflowKeepsCredentialAndArtifactSteps(t *testing.T) {
	terraformVorpalRender(t, nil)

	wf := terraformCIParseWorkflow(t)
	role := NewTerraformConfigRole("demo")

	for name, wantRole := range map[string]string{"plan": role.PlanARN, "apply": role.ApplyARN} {
		job := wf.Jobs[name]
		i := terraformCIStepIndex(job, "aws-actions/configure-aws-credentials")
		if i < 0 {
			t.Errorf("job %s has no configure-aws-credentials step", name)
			continue
		}
		want := map[string]string{
			"audience":       "sts.amazonaws.com",
			"aws-region":     "us-west-2",
			"role-to-assume": wantRole,
		}
		if got := job.Steps[i].With; !maps.Equal(got, want) {
			t.Errorf("job %s configure-aws-credentials with = %v, want %v", name, got, want)
		}
	}

	artifacts := map[string]string{"plan": "actions/upload-artifact", "apply": "actions/download-artifact"}
	for name, action := range artifacts {
		job := wf.Jobs[name]
		i := terraformCIStepIndex(job, action)
		if i < 0 {
			t.Errorf("job %s has no %s step", name, action)
			continue
		}
		if got := job.Steps[i].With["name"]; got != "tf-plan" {
			t.Errorf("job %s %s name = %q, want %q", name, action, got, "tf-plan")
		}
		if name == "plan" {
			if got := job.Steps[i].With["path"]; got != "terraform.tfplan" {
				t.Errorf("plan %s path = %q, want %q", action, got, "terraform.tfplan")
			}
		}
	}
}

func TestTerraformWorkflowRemovesDeployKeyItWrote(t *testing.T) {
	terraformCIRenderDeployKey(t)

	wf := terraformCIParseWorkflow(t)

	for _, name := range []string{"plan", "apply"} {
		job := wf.Jobs[name]
		key := terraformCIStepIndex(job, "shimataro/ssh-key-action")
		if key < 0 {
			t.Errorf("job %s has no ssh-key-action step", name)
			continue
		}
		if job.Steps[key].ID == "" || job.Steps[key].With["name"] == "" {
			t.Errorf("job %s ssh-key-action needs an id and a key file name: %+v", name, job.Steps[key])
			continue
		}
		cleanup := slices.IndexFunc(job.Steps, func(s terraformCIWorkflowStep) bool {
			return terraformCIIsDeployKeyCleanup(job, s)
		})
		if cleanup < 0 {
			t.Errorf("job %s has no `if: always()` bash step removing the key file only on ssh-key-action success: %+v", name, job.Steps)
			continue
		}
		if cleanup < key {
			t.Errorf("job %s removes the key at step %d, before ssh-key-action at step %d", name, cleanup, key)
		}
		for i, step := range job.Steps {
			if step.Shell == terraformCIVorpalShell && i > cleanup {
				t.Errorf("job %s step %q needs the key but runs after its cleanup", name, step.Run)
			}
		}
	}

	src := terraformVorpalReadFile(t, terraformCIWorkflowPath)
	if strings.Contains(src, "rm -rf") {
		t.Errorf("%s removes a directory recursively:\n%s", terraformCIWorkflowPath, src)
	}
}

func TestTerraformWorkflowOmitsDeployKeyByDefault(t *testing.T) {
	terraformVorpalRender(t, nil)

	src := terraformVorpalReadFile(t, terraformCIWorkflowPath)
	for _, unwanted := range []string{"ssh-key-action", ".ssh"} {
		if strings.Contains(src, unwanted) {
			t.Errorf("%s contains %q without setupDeployKey:\n%s", terraformCIWorkflowPath, unwanted, src)
		}
	}
}

func TestTerraformWorkflowOmitsNixCachixFlake(t *testing.T) {
	for _, tc := range []struct {
		name   string
		params map[string]interface{}
	}{
		{"default", nil},
		{"setupNix", map[string]interface{}{
			"github": map[string]interface{}{"action": map[string]interface{}{"setupNix": true}},
			"nix":    map[string]interface{}{"cachix": map[string]interface{}{"binaryCache": "legacy-cache"}},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			terraformVorpalRender(t, tc.params)

			got := strings.ToLower(terraformVorpalReadFile(t, terraformCIWorkflowPath))
			for _, banned := range []string{"nix", "cachix", "flake", "legacy-cache"} {
				if strings.Contains(got, banned) {
					t.Errorf("%s contains %q:\n%s", terraformCIWorkflowPath, banned, got)
				}
			}
		})
	}
}

func TestTerraformRendersExecutableVorpalShell(t *testing.T) {
	terraformVorpalRender(t, nil)

	info, err := os.Stat(vorpalShell)
	if err != nil {
		t.Fatalf("%s was not written: %v", vorpalShell, err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Errorf("%s mode = %#o, want %#o", vorpalShell, got, 0o755)
	}

	got := terraformVorpalReadFile(t, vorpalShell)
	for _, want := range []string{"vorpal build --path 'demo-dev'", "bin/activate"} {
		if !strings.Contains(got, want) {
			t.Errorf("%s does not contain %q:\n%s", vorpalShell, want, got)
		}
	}
}

func TestTerraformVorpalShellHonorsBcignore(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(".bcignore", []byte(vorpalShell+"\n"), 0o600); err != nil {
		t.Fatalf("write .bcignore: %v", err)
	}

	tpl, err := Config{Name: "demo", Template: TerraformName}.GetTemplater()
	if err != nil {
		t.Fatalf("GetTemplater: %v", err)
	}
	if err := tpl.Render(); err != nil {
		t.Fatalf("Render with %s ignored: %v", vorpalShell, err)
	}

	if _, err := os.Stat(vorpalShell); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("%s stat error = %v, want it absent", vorpalShell, err)
	}
}

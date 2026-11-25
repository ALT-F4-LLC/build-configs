package config

import (
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const (
	terraformPlanGroup = "${{ github.event_name == 'pull_request' && format('tf-plan-{0}', github.event.pull_request.number) || 'tf-lock' }}"
	terraformPlanRun   = "just plan ${{ github.event_name == 'pull_request' && '-lock=false' || '' }}"
)

type terraformPlanJob struct {
	Concurrency struct {
		Group            string `yaml:"group"`
		CancelInProgress bool   `yaml:"cancel-in-progress"`
	} `yaml:"concurrency"`
	Steps []terraformPlanStep `yaml:"steps"`
}

type terraformPlanStep struct {
	Env   map[string]string `yaml:"env"`
	ID    string            `yaml:"id"`
	If    string            `yaml:"if"`
	Name  string            `yaml:"name"`
	Run   string            `yaml:"run"`
	Shell string            `yaml:"shell"`
	Uses  string            `yaml:"uses"`
	With  map[string]string `yaml:"with"`
}

func terraformPlanRender(t *testing.T, params map[string]any) map[string]terraformPlanJob {
	t.Helper()
	terraformVorpalRender(t, params)

	var wf struct {
		Jobs map[string]terraformPlanJob `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(terraformVorpalReadFile(t, terraformCIWorkflowPath)), &wf); err != nil {
		t.Fatalf("workflow does not parse as YAML: %v", err)
	}
	return wf.Jobs
}

func terraformPlanFindStep(job terraformPlanJob, pred func(terraformPlanStep) bool) (int, terraformPlanStep) {
	for i, s := range job.Steps {
		if pred(s) {
			return i, s
		}
	}
	return -1, terraformPlanStep{}
}

// Plans on pull requests take one lock group per pull request, so they never
// queue behind another plan or an apply. Pushes and manual runs, and every
// apply, share tf-lock.
func TestTerraformConcurrencyIsolatesPullRequestPlans(t *testing.T) {
	jobs := terraformPlanRender(t, nil)

	if got := jobs["plan"].Concurrency.Group; got != terraformPlanGroup {
		t.Errorf("plan concurrency group =\n  %q\nwant\n  %q", got, terraformPlanGroup)
	}
	if got := jobs["apply"].Concurrency.Group; got != "tf-lock" {
		t.Errorf("apply concurrency group = %q, want tf-lock", got)
	}
	for name, job := range jobs {
		if job.Concurrency.CancelInProgress {
			t.Errorf("job %s cancels in-progress runs, which could interrupt a state lock", name)
		}
	}
}

func TestTerraformPlanSkipsStateLockOnPullRequestsOnly(t *testing.T) {
	jobs := terraformPlanRender(t, nil)

	_, plan := terraformPlanFindStep(jobs["plan"], func(s terraformPlanStep) bool {
		return strings.HasPrefix(s.Run, "just plan")
	})
	if plan.Run != terraformPlanRun {
		t.Errorf("plan step run = %q, want %q", plan.Run, terraformPlanRun)
	}
	if plan.Shell != terraformCIVorpalShell {
		t.Errorf("plan step shell = %q, want %q", plan.Shell, terraformCIVorpalShell)
	}
	_, apply := terraformPlanFindStep(jobs["apply"], func(s terraformPlanStep) bool {
		return strings.HasPrefix(s.Run, "just apply")
	})
	if apply.Run != "just apply" {
		t.Errorf("apply step run = %q, want just apply (the apply always locks)", apply.Run)
	}
}

func TestTerraformJustfilePlanForwardsArguments(t *testing.T) {
	terraformVorpalRender(t, nil)

	got := terraformVorpalReadFile(t, "justfile")
	want := "plan *ARGS:\n    terraform plan -out \"terraform.tfplan\" {{ ARGS }}\n"
	if !strings.Contains(got, want) {
		t.Errorf("justfile lacks the plan recipe\n%s\nin:\n%s", want, got)
	}
}

// The plan job posts the summary and the full plan on pull requests only, with
// jq (a default tool) rather than gawk, which no development environment has.
func TestTerraformPlanCommentsOnPullRequests(t *testing.T) {
	jobs := terraformPlanRender(t, nil)
	plan := jobs["plan"]

	render, renderStep := terraformPlanFindStep(plan, func(s terraformPlanStep) bool {
		return strings.HasPrefix(s.Run, "terraform show -no-color terraform.tfplan > terraform-plan.txt")
	})
	comment, commentStep := terraformPlanFindStep(plan, func(s terraformPlanStep) bool {
		return strings.HasPrefix(s.Uses, "actions/github-script@")
	})
	planRun, _ := terraformPlanFindStep(plan, func(s terraformPlanStep) bool {
		return strings.HasPrefix(s.Run, "just plan")
	})
	upload, _ := terraformPlanFindStep(plan, func(s terraformPlanStep) bool {
		return strings.HasPrefix(s.Uses, "actions/upload-artifact@")
	})
	if render < 0 || comment < 0 || planRun < 0 || upload < 0 {
		t.Fatalf("plan job lacks a step: plan %d render %d comment %d upload %d", planRun, render, comment, upload)
	}
	if planRun >= render || render >= comment {
		t.Errorf("plan job order: plan %d, render %d, comment %d, want plan < render < comment", planRun, render, comment)
	}

	for _, step := range []terraformPlanStep{renderStep, commentStep} {
		if step.If != "github.event_name == 'pull_request'" {
			t.Errorf("step %q if = %q, want it limited to pull requests", step.Name, step.If)
		}
	}
	if renderStep.Shell != terraformCIVorpalShell {
		t.Errorf("render step shell = %q, want %q", renderStep.Shell, terraformCIVorpalShell)
	}
	if !strings.Contains(renderStep.Run, "jq -r") {
		t.Errorf("render step does not use jq:\n%s", renderStep.Run)
	}
	if strings.Contains(renderStep.Run, "awk") {
		t.Errorf("render step uses awk, which the development environment lacks:\n%s", renderStep.Run)
	}

	script := commentStep.With["script"]
	for _, want := range []string{"Terraform Plan Summary", "listComments", "updateComment", "createComment", "paginate", "60000"} {
		if !strings.Contains(script, want) {
			t.Errorf("comment script lacks %q:\n%s", want, script)
		}
	}
	if hasEmoji := regexp.MustCompile(`[\x{1F300}-\x{1FAFF}\x{2600}-\x{27BF}]`); hasEmoji.MatchString(renderStep.Run + script) {
		t.Error("plan comment contains emoji")
	}
}

// pull-requests: write belongs to the plan job alone.
func TestTerraformApplyCannotWritePullRequests(t *testing.T) {
	terraformVorpalRender(t, nil)

	var wf struct {
		Jobs map[string]struct {
			Permissions map[string]string `yaml:"permissions"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(terraformVorpalReadFile(t, terraformCIWorkflowPath)), &wf); err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, name := range []string{"check", "apply"} {
		if got := wf.Jobs[name].Permissions["pull-requests"]; got != "" {
			t.Errorf("job %s pull-requests permission = %q, want none", name, got)
		}
	}
}

func TestTerraformAppTokenIsOffByDefault(t *testing.T) {
	jobs := terraformPlanRender(t, nil)

	src := terraformVorpalReadFile(t, terraformCIWorkflowPath)
	for _, unwanted := range []string{"create-github-app-token", "ALTF4LLC_GITHUB_APP", "GITHUB_TOKEN"} {
		if strings.Contains(src, unwanted) {
			t.Errorf("default workflow contains %q", unwanted)
		}
	}
	if len(jobs) != 3 {
		t.Errorf("jobs = %d, want 3", len(jobs))
	}
}

// The token step runs in the plan and apply jobs, after the role is assumed
// and before checkout. Only the apply step receives the token: a plan runs the
// pull request's own code, so it never sees an owner-wide token.
func TestTerraformAppTokenStepInPlanAndApply(t *testing.T) {
	jobs := terraformPlanRender(t, map[string]any{"github": map[string]any{"appToken": true}})

	for _, name := range []string{"plan", "apply"} {
		job := jobs[name]
		token, step := terraformPlanFindStep(job, func(s terraformPlanStep) bool {
			return strings.HasPrefix(s.Uses, "actions/create-github-app-token@")
		})
		if token < 0 {
			t.Errorf("job %s has no create-github-app-token step", name)
			continue
		}
		if step.ID != "generate-token" {
			t.Errorf("job %s token step id = %q, want generate-token", name, step.ID)
		}
		want := map[string]string{
			"app-id":      "${{ secrets.ALTF4LLC_GITHUB_APP_ID }}",
			"owner":       "${{ github.repository_owner }}",
			"private-key": "${{ secrets.ALTF4LLC_GITHUB_APP_PRIVATE_KEY }}",
		}
		for k, v := range want {
			if step.With[k] != v {
				t.Errorf("job %s token step with.%s = %q, want %q", name, k, step.With[k], v)
			}
		}
		role, _ := terraformPlanFindStep(job, func(s terraformPlanStep) bool {
			return strings.HasPrefix(s.Uses, "aws-actions/configure-aws-credentials@")
		})
		checkout, _ := terraformPlanFindStep(job, func(s terraformPlanStep) bool {
			return strings.HasPrefix(s.Uses, "actions/checkout@")
		})
		if role >= token || token >= checkout {
			t.Errorf("job %s order: role %d, token %d, checkout %d, want role < token < checkout", name, role, token, checkout)
		}
	}

	_, apply := terraformPlanFindStep(jobs["apply"], func(s terraformPlanStep) bool { return s.Run == "just apply" })
	if got := apply.Env["GITHUB_TOKEN"]; got != "${{ steps.generate-token.outputs.token }}" {
		t.Errorf("apply step GITHUB_TOKEN = %q, want the generated token", got)
	}
	for i, step := range jobs["plan"].Steps {
		if _, ok := step.Env["GITHUB_TOKEN"]; ok {
			t.Errorf("plan step %d (%q) receives GITHUB_TOKEN", i, step.Run)
		}
	}
}

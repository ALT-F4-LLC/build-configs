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
	goLambdaWorkflowPath    = ".github/workflows/ci.yaml"
	goLambdaVorpalShellPath = ".github/vorpal-shell.sh"
	goLambdaTestForkGuard   = "github.event_name != 'pull_request' || github.event.pull_request.head.repo.full_name == github.repository"
	goLambdaTestDeployIf    = "github.ref == 'refs/heads/main'"
)

type goLambdaWorkflow struct {
	Jobs map[string]goLambdaWorkflowJob `yaml:"jobs"`
}

type goLambdaWorkflowConcurrency struct {
	Group            string `yaml:"group"`
	CancelInProgress *bool  `yaml:"cancel-in-progress"`
}

type goLambdaWorkflowJob struct {
	Concurrency goLambdaWorkflowConcurrency `yaml:"concurrency"`
	If          string                      `yaml:"if"`
	Permissions map[string]string           `yaml:"permissions"`
	RunsOn      string                      `yaml:"runs-on"`
	Steps       []goLambdaWorkflowStep      `yaml:"steps"`
}

type goLambdaWorkflowStep struct {
	If    string `yaml:"if"`
	Run   string `yaml:"run"`
	Shell string `yaml:"shell"`
	Uses  string `yaml:"uses"`
}

// goLambdaRenderCI renders go-lambda into a fresh working directory with
// three deploy environments: a bare condition, a condition already wrapped
// in a GitHub expression, and no condition.
func goLambdaRenderCI(t *testing.T, privateModules string) {
	t.Helper()
	t.Chdir(t.TempDir())

	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Lambdas = []string{"api", "worker"}
	cfg.PrivateModules = privateModules
	cfg.Deploy = []DeployConfig{
		{Environment: "dev", Region: "us-west-2", RoleARN: "arn:aws:iam::111111111111:role/dev", If: goLambdaTestDeployIf},
		{Environment: "prod", Region: "us-west-2", RoleARN: "arn:aws:iam::111111111111:role/prod", If: "${{ " + goLambdaTestDeployIf + " }}"},
		{Environment: "staging", Region: "us-west-2", RoleARN: "arn:aws:iam::111111111111:role/staging"},
	}
	if err := cfg.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

func goLambdaReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // reads a file the test rendered into its own temp dir
	if err != nil {
		t.Fatalf("%s was not written: %v", path, err)
	}
	return string(b)
}

func goLambdaParseWorkflow(t *testing.T) goLambdaWorkflow {
	t.Helper()
	src := goLambdaReadFile(t, goLambdaWorkflowPath)
	var wf goLambdaWorkflow
	if err := yaml.Unmarshal([]byte(src), &wf); err != nil {
		t.Fatalf("%s does not parse as YAML: %v\n%s", goLambdaWorkflowPath, err, src)
	}
	return wf
}

// goLambdaIsCredentialStep reports whether a run step writes or removes the
// netrc credentials; those steps use the runner's default shell.
func goLambdaIsCredentialStep(step goLambdaWorkflowStep) bool {
	return strings.Contains(step.Run, ".netrc")
}

func TestGoLambdaRenderWritesCIWorkflowInsteadOfFlake(t *testing.T) {
	goLambdaRenderCI(t, "")

	goLambdaReadFile(t, goLambdaWorkflowPath)
	if _, err := os.Stat(".github/workflows/flake.yaml"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf(".github/workflows/flake.yaml stat error = %v, want it absent", err)
	}
}

func TestGoLambdaRendersExecutableVorpalShell(t *testing.T) {
	goLambdaRenderCI(t, "")

	info, err := os.Stat(goLambdaVorpalShellPath)
	if err != nil {
		t.Fatalf("%s was not written: %v", goLambdaVorpalShellPath, err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Errorf("%s mode = %#o, want %#o", goLambdaVorpalShellPath, got, 0o755)
	}
	got := goLambdaReadFile(t, goLambdaVorpalShellPath)
	for _, want := range []string{"vorpal build --path 'example-dev'", "bin/activate"} {
		if !strings.Contains(got, want) {
			t.Errorf("%s does not contain %q:\n%s", goLambdaVorpalShellPath, want, got)
		}
	}
}

func TestGoLambdaCIJobsRunOnAltf4llcThroughVorpalShell(t *testing.T) {
	for _, privateModules := range []string{"", "github.com/ALT-F4-LLC/quirk-service-kit"} {
		t.Run("privateModules="+privateModules, func(t *testing.T) {
			goLambdaRenderCI(t, privateModules)
			wf := goLambdaParseWorkflow(t)

			want := []string{"build", "check", "deploy-dev", "deploy-prod", "deploy-staging"}
			if got := slices.Sorted(maps.Keys(wf.Jobs)); !slices.Equal(got, want) {
				t.Fatalf("jobs = %q, want %q", got, want)
			}
			wantShell := "bash " + goLambdaVorpalShellPath + " {0}"
			for name, job := range wf.Jobs {
				if job.RunsOn != "altf4llc" {
					t.Errorf("job %s runs-on = %q, want %q", name, job.RunsOn, "altf4llc")
				}
				checkedOut := false
				justRuns := 0
				for _, step := range job.Steps {
					if strings.HasPrefix(step.Uses, "actions/checkout@") {
						checkedOut = true
					}
					if step.Run == "" {
						continue
					}
					if goLambdaIsCredentialStep(step) {
						if step.Shell != "" {
							t.Errorf("job %s credential step %q shell = %q, want the runner default", name, step.Run, step.Shell)
						}
						continue
					}
					if step.Shell != wantShell {
						t.Errorf("job %s step %q shell = %q, want %q", name, step.Run, step.Shell, wantShell)
					}
					if !checkedOut {
						t.Errorf("job %s runs %q before checking out the repository", name, step.Run)
					}
					if strings.HasPrefix(step.Run, "just ") {
						justRuns++
					}
				}
				if justRuns == 0 {
					t.Errorf("job %s runs no just recipe", name)
				}
			}
		})
	}
}

func TestGoLambdaCIJobIfJoinsForkGuard(t *testing.T) {
	goLambdaRenderCI(t, "")
	wf := goLambdaParseWorkflow(t)

	guardOnly := "${{ " + goLambdaTestForkGuard + " }}"
	joined := "${{ (" + goLambdaTestForkGuard + ") && (" + goLambdaTestDeployIf + ") }}"
	for name, want := range map[string]string{
		"check":          guardOnly,
		"build":          guardOnly,
		"deploy-dev":     joined,
		"deploy-prod":    joined,
		"deploy-staging": guardOnly,
	} {
		if got := wf.Jobs[name].If; got != want {
			t.Errorf("job %s if =\n  %q\nwant\n  %q", name, got, want)
		}
	}
}

func TestGoLambdaCIJobPermissions(t *testing.T) {
	goLambdaRenderCI(t, "github.com/ALT-F4-LLC/quirk-service-kit")
	wf := goLambdaParseWorkflow(t)

	readOnly := map[string]string{"contents": "read"}
	deploy := map[string]string{"id-token": "write", "contents": "read"}
	for name, want := range map[string]map[string]string{
		"check":          readOnly,
		"build":          readOnly,
		"deploy-dev":     deploy,
		"deploy-prod":    deploy,
		"deploy-staging": deploy,
	} {
		if got := wf.Jobs[name].Permissions; !maps.Equal(got, want) {
			t.Errorf("job %s permissions = %v, want %v", name, got, want)
		}
	}
}

func TestGoLambdaCIOmitsNixCachixFlake(t *testing.T) {
	goLambdaRenderCI(t, "github.com/ALT-F4-LLC/quirk-service-kit")

	got := strings.ToLower(goLambdaReadFile(t, goLambdaWorkflowPath))
	for _, banned := range []string{"nix", "cachix", "flake"} {
		if strings.Contains(got, banned) {
			t.Errorf("%s contains %q:\n%s", goLambdaWorkflowPath, banned, got)
		}
	}
}

func TestGoLambdaCICleansUpNetrcCredentials(t *testing.T) {
	goLambdaRenderCI(t, "github.com/ALT-F4-LLC/quirk-service-kit")
	steps := goLambdaParseWorkflow(t).Jobs["build"].Steps

	// Each credential writer maps to the only cleanup command allowed to
	// follow it.
	writers := map[int]string{}
	for i, step := range steps {
		switch {
		case strings.HasPrefix(step.Uses, "extractions/netrc@"):
			writers[i] = "rm -f ~/.netrc"
		case strings.HasPrefix(step.Run, "cp ") && strings.Contains(step.Run, "/tmp/.netrc"):
			writers[i] = "rm -f /tmp/.netrc"
		}
	}
	if len(writers) != 2 {
		t.Fatalf("build job has %d netrc-writing steps, want 2: %+v", len(writers), steps)
	}
	for i, cleanup := range writers {
		found := slices.ContainsFunc(steps[i+1:], func(s goLambdaWorkflowStep) bool {
			return s.Run == cleanup && s.If == "always()" && s.Shell == ""
		})
		if !found {
			t.Errorf("step %d (%+v) is not followed by %q with if: always() on the default shell", i, steps[i], cleanup)
		}
	}
	for _, step := range steps {
		if strings.HasPrefix(step.Run, "rm ") && !slices.Contains(slices.Collect(maps.Values(writers)), step.Run) {
			t.Errorf("unexpected removal step %q", step.Run)
		}
	}
}

func TestGoLambdaCIWithoutPrivateModulesWritesNoCredentials(t *testing.T) {
	goLambdaRenderCI(t, "")

	got := goLambdaReadFile(t, goLambdaWorkflowPath)
	for _, banned := range []string{"netrc", "create-github-app-token"} {
		if strings.Contains(got, banned) {
			t.Errorf("%s contains %q without private modules:\n%s", goLambdaWorkflowPath, banned, got)
		}
	}
}

func TestGoLambdaCIDeployJobsQueueInsteadOfCancel(t *testing.T) {
	goLambdaRenderCI(t, "")
	wf := goLambdaParseWorkflow(t)

	for _, env := range []string{"dev", "prod", "staging"} {
		job, ok := wf.Jobs["deploy-"+env]
		if !ok {
			t.Fatalf("job deploy-%s missing", env)
		}
		if want := "deploy-" + env + "-${{ matrix.profile }}"; job.Concurrency.Group != want {
			t.Errorf("deploy-%s concurrency.group = %q, want %q", env, job.Concurrency.Group, want)
		}
		if job.Concurrency.CancelInProgress == nil || *job.Concurrency.CancelInProgress {
			t.Errorf("deploy-%s concurrency.cancel-in-progress = %v, want false", env, job.Concurrency.CancelInProgress)
		}
	}
	for _, name := range []string{"build", "check"} {
		if group := wf.Jobs[name].Concurrency.Group; group != "" {
			t.Errorf("job %s concurrency.group = %q, want none", name, group)
		}
	}
}

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
	goCobraCliWorkflowPath    = ".github/workflows/ci.yaml"
	goCobraCliVorpalShellPath = ".github/vorpal-shell.sh"
	goCobraCliStepShell       = "bash " + goCobraCliVorpalShellPath + " {0}"
	goCobraCliForkGuard       = "github.event_name != 'pull_request' || github.event.pull_request.head.repo.full_name == github.repository"
)

type goCobraCliWorkflow struct {
	Jobs map[string]goCobraCliWorkflowJob `yaml:"jobs"`
}

type goCobraCliWorkflowJob struct {
	If          string                   `yaml:"if"`
	Permissions map[string]string        `yaml:"permissions"`
	RunsOn      string                   `yaml:"runs-on"`
	Steps       []goCobraCliWorkflowStep `yaml:"steps"`
}

type goCobraCliWorkflowStep struct {
	If    string `yaml:"if"`
	Run   string `yaml:"run"`
	Shell string `yaml:"shell"`
	Uses  string `yaml:"uses"`
}

// goCobraCliIsCredentialCleanup reports whether step is a post-job cleanup of
// a credential file, which runs on the runner's default shell.
func (s goCobraCliWorkflowStep) goCobraCliIsCredentialCleanup() bool {
	return s.If == "always()" && strings.HasPrefix(s.Run, "rm -f ")
}

// goCobraCliPrivateModules is the parameter set that renders the private-module
// netrc steps.
var goCobraCliPrivateModules = map[string]interface{}{"privateModules": "github.com/ALT-F4-LLC/*"}

func goCobraCliParseWorkflow(t *testing.T) goCobraCliWorkflow {
	t.Helper()
	src := goCobraCliReadFile(t, goCobraCliWorkflowPath)
	var wf goCobraCliWorkflow
	if err := yaml.Unmarshal([]byte(src), &wf); err != nil {
		t.Fatalf("%s does not parse as YAML: %v\n%s", goCobraCliWorkflowPath, err, src)
	}
	return wf
}

func TestGoCobraCliRendersCIWorkflowInPlaceOfFlakeWorkflow(t *testing.T) {
	goCobraCliRenderTemp(t, nil)

	if _, err := os.Stat(goCobraCliWorkflowPath); err != nil {
		t.Errorf("%s was not written: %v", goCobraCliWorkflowPath, err)
	}
	if _, err := os.Stat(".github/workflows/flake.yaml"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf(".github/workflows/flake.yaml stat error = %v, want it absent", err)
	}
}

func TestGoCobraCliRendersExecutableVorpalShellForDevEnvironment(t *testing.T) {
	goCobraCliRenderTemp(t, nil)

	info, err := os.Stat(goCobraCliVorpalShellPath)
	if err != nil {
		t.Fatalf("%s was not written: %v", goCobraCliVorpalShellPath, err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Errorf("%s mode = %#o, want %#o", goCobraCliVorpalShellPath, got, 0o755)
	}

	got := goCobraCliReadFile(t, goCobraCliVorpalShellPath)
	for _, want := range []string{`"$(vorpal build --path 'demo-dev')/bin/activate"`, `bash -e "$1"`} {
		if !strings.Contains(got, want) {
			t.Errorf("%s does not contain %s:\n%s", goCobraCliVorpalShellPath, want, got)
		}
	}
}

func TestGoCobraCliVorpalShellHonorsBcignore(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(".bcignore", []byte(goCobraCliVorpalShellPath+"\n"), 0o600); err != nil {
		t.Fatalf("write .bcignore: %v", err)
	}

	tpl, err := Config{Name: "demo", Template: GoCobraCliName}.GetTemplater()
	if err != nil {
		t.Fatalf("GetTemplater: %v", err)
	}
	if err := tpl.Render(); err != nil {
		t.Fatalf("Render with %s ignored: %v", goCobraCliVorpalShellPath, err)
	}

	if _, err := os.Stat(goCobraCliVorpalShellPath); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("%s stat error = %v, want it absent", goCobraCliVorpalShellPath, err)
	}
}

func TestGoCobraCliWorkflowJobsRunOnAltf4llcThroughVorpalShell(t *testing.T) {
	for name, parameters := range map[string]map[string]interface{}{
		"default":         nil,
		"private modules": goCobraCliPrivateModules,
	} {
		t.Run(name, func(t *testing.T) {
			goCobraCliRenderTemp(t, parameters)
			wf := goCobraCliParseWorkflow(t)

			if got := slices.Sorted(maps.Keys(wf.Jobs)); !slices.Equal(got, []string{"build", "check"}) {
				t.Fatalf("jobs = %q, want [build check]", got)
			}
			wantIf := "${{ " + goCobraCliForkGuard + " }}"
			for jobName, job := range wf.Jobs {
				if job.RunsOn != "altf4llc" {
					t.Errorf("job %s runs-on = %q, want %q", jobName, job.RunsOn, "altf4llc")
				}
				if job.If != wantIf {
					t.Errorf("job %s if = %q, want %q", jobName, job.If, wantIf)
				}
				if !maps.Equal(job.Permissions, map[string]string{"contents": "read"}) {
					t.Errorf("job %s permissions = %v, want only contents: read", jobName, job.Permissions)
				}

				checkedOut, runs := false, 0
				for _, step := range job.Steps {
					if step.Uses == "actions/checkout@v7" {
						checkedOut = true
					}
					if step.Run == "" {
						continue
					}
					runs++
					if step.goCobraCliIsCredentialCleanup() {
						if step.Shell != "" {
							t.Errorf("job %s cleanup %q shell = %q, want the runner default", jobName, step.Run, step.Shell)
						}
						continue
					}
					if step.Shell != goCobraCliStepShell {
						t.Errorf("job %s step %q shell = %q, want %q", jobName, step.Run, step.Shell, goCobraCliStepShell)
					}
					if !checkedOut {
						t.Errorf("job %s step %q uses %s before actions/checkout@v7", jobName, step.Run, goCobraCliVorpalShellPath)
					}
				}
				if runs == 0 {
					t.Errorf("job %s runs no command", jobName)
				}
			}

			for jobName, want := range map[string]string{"check": "just check", "build": "just build"} {
				if !slices.ContainsFunc(wf.Jobs[jobName].Steps, func(s goCobraCliWorkflowStep) bool { return s.Run == want }) {
					t.Errorf("job %s does not run %q: %+v", jobName, want, wf.Jobs[jobName].Steps)
				}
			}
		})
	}
}

func TestGoCobraCliWorkflowCleansUpEachCredentialFileItWrites(t *testing.T) {
	goCobraCliRenderTemp(t, goCobraCliPrivateModules)
	steps := goCobraCliParseWorkflow(t).Jobs["build"].Steps

	writers := map[string]func(goCobraCliWorkflowStep) bool{
		"~/.netrc": func(s goCobraCliWorkflowStep) bool {
			return strings.HasPrefix(s.Uses, "extractions/netrc@")
		},
		"/tmp/.netrc": func(s goCobraCliWorkflowStep) bool {
			return strings.HasPrefix(s.Run, "cp ") && strings.HasSuffix(s.Run, " /tmp/.netrc")
		},
	}
	for file, writes := range writers {
		at := slices.IndexFunc(steps, writes)
		if at < 0 {
			t.Errorf("build job has no step writing %s: %+v", file, steps)
			continue
		}
		cleanup := goCobraCliWorkflowStep{If: "always()", Run: "rm -f " + file}
		if !slices.Contains(steps[at+1:], cleanup) {
			t.Errorf("step writing %s is not followed by %+v: %+v", file, cleanup, steps)
		}
	}

	cleanups := 0
	for _, step := range steps {
		if strings.Contains(step.Run, ".ssh") {
			t.Errorf("step %q touches ~/.ssh", step.Run)
		}
		if step.goCobraCliIsCredentialCleanup() {
			cleanups++
		}
	}
	if cleanups != len(writers) {
		t.Errorf("build job has %d credential cleanups, want %d: %+v", cleanups, len(writers), steps)
	}
}

func TestGoCobraCliWorkflowRendersNoNetrcStepsWithoutPrivateModules(t *testing.T) {
	goCobraCliRenderTemp(t, nil)

	if got := goCobraCliReadFile(t, goCobraCliWorkflowPath); strings.Contains(got, "netrc") {
		t.Errorf("%s renders netrc steps without privateModules:\n%s", goCobraCliWorkflowPath, got)
	}
}

func TestGoCobraCliCIOmitsNixCachixFlake(t *testing.T) {
	for name, parameters := range map[string]map[string]interface{}{
		"default":         nil,
		"private modules": goCobraCliPrivateModules,
	} {
		t.Run(name, func(t *testing.T) {
			goCobraCliRenderTemp(t, parameters)

			got := strings.ToLower(goCobraCliReadFile(t, goCobraCliWorkflowPath))
			for _, banned := range []string{"nix", "cachix", "flake"} {
				if strings.Contains(got, banned) {
					t.Errorf("%s contains %q:\n%s", goCobraCliWorkflowPath, banned, got)
				}
			}
		})
	}
}

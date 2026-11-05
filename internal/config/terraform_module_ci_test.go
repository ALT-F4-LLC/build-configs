package config

import (
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const terraformModuleWorkflowPath = ".github/workflows/terraform.yaml"

type terraformModuleWorkflow struct {
	Jobs map[string]terraformModuleWorkflowJob `yaml:"jobs"`
}

type terraformModuleWorkflowJob struct {
	If          string                        `yaml:"if"`
	Permissions map[string]string             `yaml:"permissions"`
	RunsOn      string                        `yaml:"runs-on"`
	Steps       []terraformModuleWorkflowStep `yaml:"steps"`
}

type terraformModuleWorkflowStep struct {
	Run   string `yaml:"run"`
	Shell string `yaml:"shell"`
	Uses  string `yaml:"uses"`
}

func terraformModuleReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // reads a file the test rendered into its own temp dir
	if err != nil {
		t.Fatalf("%s was not written: %v", path, err)
	}
	return string(b)
}

func terraformModuleParseWorkflow(t *testing.T) terraformModuleWorkflow {
	t.Helper()
	src := terraformModuleReadFile(t, terraformModuleWorkflowPath)
	var wf terraformModuleWorkflow
	if err := yaml.Unmarshal([]byte(src), &wf); err != nil {
		t.Fatalf("%s does not parse as YAML: %v\n%s", terraformModuleWorkflowPath, err, src)
	}
	return wf
}

// terraformModuleJustfileRecipes maps each recipe name in a justfile to its
// body lines with indentation removed.
func terraformModuleJustfileRecipes(src string) map[string][]string {
	recipes := map[string][]string{}
	var current string
	for line := range strings.SplitSeq(src, "\n") {
		switch {
		case strings.TrimSpace(line) == "":
		case line[0] != ' ' && line[0] != '\t':
			current = strings.TrimSuffix(strings.TrimSpace(line), ":")
			recipes[current] = nil
		default:
			recipes[current] = append(recipes[current], strings.TrimSpace(line))
		}
	}
	return recipes
}

func TestTerraformModuleWorkflowJobsRunOnAltf4llcThroughVorpalShell(t *testing.T) {
	terraformModuleRenderTemp(t, "demo-module", nil)

	wf := terraformModuleParseWorkflow(t)

	if got := slices.Sorted(maps.Keys(wf.Jobs)); !slices.Equal(got, []string{"check", "package"}) {
		t.Fatalf("jobs = %q, want [check package]", got)
	}
	wantShell := "bash " + terraformModuleShellPath + " {0}"
	for name, job := range wf.Jobs {
		if job.RunsOn != "altf4llc" {
			t.Errorf("job %s runs-on = %q, want %q", name, job.RunsOn, "altf4llc")
		}
		if job.Permissions["contents"] != "read" || len(job.Permissions) != 1 {
			t.Errorf("job %s permissions = %v, want only contents: read", name, job.Permissions)
		}
		if !strings.HasPrefix(job.If, "${{ ") || !strings.HasSuffix(job.If, " }}") ||
			!strings.Contains(job.If, "github.event.pull_request.head.repo.full_name == github.repository") {
			t.Errorf("job %s if = %q, want a same-repository guard as a GitHub expression", name, job.If)
		}
		if len(job.Steps) == 0 || job.Steps[0].Uses != "actions/checkout@v7" {
			t.Errorf("job %s does not start with actions/checkout@v7: %+v", name, job.Steps)
		}
		runs := 0
		for _, step := range job.Steps {
			if step.Run == "" {
				continue
			}
			runs++
			if step.Shell != wantShell {
				t.Errorf("job %s step %q shell = %q, want %q", name, step.Run, step.Shell, wantShell)
			}
		}
		if runs == 0 {
			t.Errorf("job %s runs no command", name)
		}
	}
}

func TestTerraformModuleCIOmitsNixCachixFlake(t *testing.T) {
	terraformModuleRenderTemp(t, "demo-module", nil)

	for _, path := range []string{terraformModuleWorkflowPath, "justfile"} {
		got := strings.ToLower(terraformModuleReadFile(t, path))
		for _, banned := range []string{"nix", "cachix", "flake"} {
			if strings.Contains(got, banned) {
				t.Errorf("%s contains %q:\n%s", path, banned, got)
			}
		}
	}
}

func TestTerraformModuleJustfileRecipes(t *testing.T) {
	terraformModuleRenderTemp(t, "demo-module", nil)

	got := terraformModuleJustfileRecipes(terraformModuleReadFile(t, "justfile"))

	want := map[string][]string{
		"_default": {"just --list"},
		"check":    {"terraform fmt -check"},
		"docs":     {"terraform-docs markdown table --output-file README.md --output-mode inject ."},
		"init":     {"terraform init"},
		"package":  {"vorpal build --path 'dev'"},
		"validate": {"terraform validate"},
	}
	if !maps.EqualFunc(got, want, slices.Equal) {
		t.Errorf("justfile recipes = %q, want %q", got, want)
	}
}

func TestTerraformModuleRendersConfigWithLegacyNixKey(t *testing.T) {
	terraformModuleRenderTemp(t, "demo-module", map[string]any{
		"nix": map[string]any{"cachix": map[string]any{"binaryCache": "legacy-cache"}},
	})

	if got := terraformModuleReadFile(t, terraformModuleWorkflowPath); strings.Contains(got, "legacy-cache") {
		t.Errorf("%s still renders the legacy nix option:\n%s", terraformModuleWorkflowPath, got)
	}
}

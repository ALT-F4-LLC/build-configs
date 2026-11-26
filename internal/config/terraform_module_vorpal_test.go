package config

import (
	"os"
	"slices"
	"strings"
	"testing"
)

type terraformModuleVorpalProgram struct {
	environmentNames []string
	aliases          []string
}

// terraformModuleRenderTemp renders the terraform-module template named
// name with params into a fresh temporary working directory.
func terraformModuleRenderTemp(t *testing.T, name string, params map[string]any) {
	t.Helper()
	t.Chdir(t.TempDir())

	tpl, err := Config{Name: name, Template: TerraformModuleName, Parameters: params}.GetTemplater()
	if err != nil {
		t.Fatalf("GetTemplater: %v", err)
	}
	if err := tpl.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

// terraformModuleParseVorpalProgram reads the rendered src/vorpal.ts and
// returns the development environment name and the aliases in its tools list.
func terraformModuleParseVorpalProgram(t *testing.T) terraformModuleVorpalProgram {
	t.Helper()

	src, err := os.ReadFile("src/vorpal.ts")
	if err != nil {
		t.Fatalf("src/vorpal.ts was not written: %v", err)
	}
	envName, aliases := terraformVorpalProgramTools(t, string(src))
	return terraformModuleVorpalProgram{environmentNames: []string{envName}, aliases: aliases}
}

func TestTerraformModuleRendersVorpalToml(t *testing.T) {
	terraformModuleRenderTemp(t, "demo-module", nil)

	got, err := os.ReadFile("Vorpal.toml")
	if err != nil {
		t.Fatalf("Vorpal.toml was not written: %v", err)
	}

	for _, want := range []string{`language = "typescript"`, "includes = [\n    \"bun.lock\",\n    \"bunfig.toml\",\n    \"package.json\",\n    \"src/vorpal.ts\",\n    \"tsconfig.json\"\n]", "[source.typescript]\nentrypoint = \"src/vorpal.ts\""} {
		if !strings.Contains(string(got), want) {
			t.Errorf("Vorpal.toml missing %q:\n%s", want, got)
		}
	}
	for line := range strings.SplitSeq(string(got), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "script") {
			t.Errorf("Vorpal.toml has a script line %q, want none:\n%s", line, got)
		}
	}
	if _, err := os.Stat("cmd/vorpal/main.go"); err == nil {
		t.Error("cmd/vorpal/main.go rendered; the config program is TypeScript")
	}
}

func TestTerraformModuleVorpalProgramDefaults(t *testing.T) {
	terraformModuleRenderTemp(t, "demo-module", nil)

	program := terraformModuleParseVorpalProgram(t)

	if !slices.Equal(program.environmentNames, []string{"demo-module-dev"}) {
		t.Errorf("development environments = %q, want [\"demo-module-dev\"]", program.environmentNames)
	}
	for _, alias := range []string{"gitleaks:8.30.1", "jq:1.8.1", "just:1.45.0", "rumdl:0.3.2", "terraform:1.13.1"} {
		if !slices.Contains(program.aliases, alias) {
			t.Errorf("aliases %q do not include %q", program.aliases, alias)
		}
	}
}

func TestTerraformModuleVorpalTerraformVersionOverridesAlias(t *testing.T) {
	terraformModuleRenderTemp(t, "demo-module", map[string]any{
		"vorpal": map[string]any{"terraformVersion": "1.9.8"},
	})

	program := terraformModuleParseVorpalProgram(t)

	if !slices.Contains(program.aliases, "terraform:1.9.8") {
		t.Errorf("aliases %q do not include %q", program.aliases, "terraform:1.9.8")
	}
	if slices.Contains(program.aliases, "terraform:1.13.1") {
		t.Errorf("aliases %q still include the default %q", program.aliases, "terraform:1.13.1")
	}
}

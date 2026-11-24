package config

import (
	"encoding/json"
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"
)

func terraformVorpalRender(t *testing.T, parameters map[string]interface{}) {
	t.Helper()
	t.Chdir(t.TempDir())

	tpl, err := Config{Name: "demo", Template: TerraformName, Parameters: parameters}.GetTemplater()
	if err != nil {
		t.Fatalf("GetTemplater: %v", err)
	}
	if err := tpl.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

func terraformVorpalReadFile(t *testing.T, name string) string {
	t.Helper()
	b, err := fs.ReadFile(os.DirFS("."), name)
	if err != nil {
		t.Fatalf("reading rendered %s: %v", name, err)
	}
	return string(b)
}

// terraformVorpalProgramPath is the rendered Vorpal config program.
const terraformVorpalProgramPath = "src/vorpal.ts"

// terraformVorpalProgramTools decodes the `const tools: string[]` array the
// rendered program declares, and returns the development environment name it
// declares in `const devEnvironment`.
func terraformVorpalProgramTools(t *testing.T, src string) (string, []string) {
	t.Helper()

	var envName string
	for line := range strings.SplitSeq(src, "\n") {
		if value, ok := strings.CutPrefix(line, "const devEnvironment = "); ok {
			var name string
			if err := json.Unmarshal([]byte(strings.TrimSuffix(value, ";")), &name); err != nil {
				t.Fatalf("devEnvironment %q is not a JSON string: %v", value, err)
			}
			envName = name
		}
	}
	return envName, typeScriptFrontendArray(t, src, "tools")
}

func TestTerraformRenderWritesVorpalConfig(t *testing.T) {
	terraformVorpalRender(t, nil)

	got := terraformVorpalReadFile(t, "Vorpal.toml")
	want := hashNoticeLines + `language = "typescript"

[source]
includes = [
    "bun.lock",
    "bunfig.toml",
    "package.json",
    "src/vorpal.ts",
    "tsconfig.json"
]

[source.typescript]
entrypoint = "src/vorpal.ts"
`
	if got != want {
		t.Errorf("Vorpal.toml =\n%s\nwant\n%s", got, want)
	}
	if _, err := os.Stat("cmd/vorpal/main.go"); err == nil {
		t.Error("cmd/vorpal/main.go rendered; the config program is TypeScript")
	}
}

func TestTerraformRenderWritesVorpalDevEnvironment(t *testing.T) {
	terraformVorpalRender(t, nil)

	src := terraformVorpalReadFile(t, terraformVorpalProgramPath)
	envName, aliases := terraformVorpalProgramTools(t, src)
	if envName != "demo-dev" {
		t.Errorf("development environment name = %q, want %q", envName, "demo-dev")
	}
	for _, want := range []string{"gitleaks:8.30.1", "jq:1.8.1", "just:1.45.0", "rumdl:0.3.2", "terraform:1.13.1", "awscli2:2.33.1"} {
		if !slices.Contains(aliases, want) {
			t.Errorf("aliases %v missing %q", aliases, want)
		}
	}
	for _, want := range []string{
		`from "@altf4llc/vorpal-sdk";`,
		"new TypeScriptDevelopmentEnvironment(devEnvironment, SYSTEMS)",
		".withArtifacts(toolDigests)",
		"await context.run();",
		"void main();",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("%s does not contain %q", terraformVorpalProgramPath, want)
		}
	}
	if strings.Contains(src, "<no value>") {
		t.Errorf("%s has an unrendered value:\n%s", terraformVorpalProgramPath, src)
	}
}

func TestTerraformRenderVorpalTerraformVersionOverride(t *testing.T) {
	terraformVorpalRender(t, map[string]interface{}{
		"vorpal": map[string]interface{}{"terraformVersion": "1.12.0"},
	})

	_, aliases := terraformVorpalProgramTools(t, terraformVorpalReadFile(t, terraformVorpalProgramPath))
	if !slices.Contains(aliases, "terraform:1.12.0") {
		t.Errorf("aliases %v missing %q", aliases, "terraform:1.12.0")
	}
	if slices.Contains(aliases, "terraform:1.13.1") {
		t.Errorf("aliases %v still carry the default %q", aliases, "terraform:1.13.1")
	}
}

// Every value the config reaches the program through is a JSON string literal,
// so a quote or backslash in a tool alias or the name cannot end it.
func TestTerraformRenderVorpalProgramQuotesValues(t *testing.T) {
	terraformVorpalRender(t, map[string]interface{}{
		"vorpal": map[string]interface{}{"tools": []interface{}{`a"b:1`, `c\d:2`, "e`${f}`:3"}},
	})

	_, aliases := terraformVorpalProgramTools(t, terraformVorpalReadFile(t, terraformVorpalProgramPath))
	for _, want := range []string{`a"b:1`, `c\d:2`, "e`${f}`:3"} {
		if !slices.Contains(aliases, want) {
			t.Errorf("aliases %q missing %q", aliases, want)
		}
	}
}

// The ignore file keeps the Bun dependencies of the config program out of git.
func TestTerraformRenderIgnoresNodeModules(t *testing.T) {
	terraformVorpalRender(t, nil)

	if got := terraformVorpalReadFile(t, ".gitignore"); !strings.Contains(got, "\nnode_modules\n") {
		t.Errorf(".gitignore does not ignore node_modules:\n%s", got)
	}
}

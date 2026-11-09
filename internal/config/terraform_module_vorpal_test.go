package config

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strconv"
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

// terraformModuleParseVorpalProgram parses the rendered cmd/vorpal/main.go and
// collects the names passed to artifact.NewDevelopmentEnvironment and the
// aliases passed to FetchArtifactAlias.
func terraformModuleParseVorpalProgram(t *testing.T) terraformModuleVorpalProgram {
	t.Helper()

	src, err := os.ReadFile("cmd/vorpal/main.go")
	if err != nil {
		t.Fatalf("cmd/vorpal/main.go was not written: %v", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", src, 0)
	if err != nil {
		t.Fatalf("rendered program does not parse: %v\n%s", err, src)
	}

	var program terraformModuleVorpalProgram
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			t.Fatalf("unquote %s: %v", lit.Value, err)
		}
		switch sel.Sel.Name {
		case "NewDevelopmentEnvironment":
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "artifact" {
				program.environmentNames = append(program.environmentNames, value)
			}
		case "FetchArtifactAlias":
			program.aliases = append(program.aliases, value)
		}
		return true
	})
	return program
}

func TestTerraformModuleRendersVorpalToml(t *testing.T) {
	terraformModuleRenderTemp(t, "demo-module", nil)

	got, err := os.ReadFile("Vorpal.toml")
	if err != nil {
		t.Fatalf("Vorpal.toml was not written: %v", err)
	}

	for _, want := range []string{`language = "go"`, "includes = [\n    \"cmd/vorpal\",\n    \"go.mod\",\n    \"go.sum\"\n]", "[source.go]\ndirectory = \"cmd/vorpal\""} {
		if !strings.Contains(string(got), want) {
			t.Errorf("Vorpal.toml missing %q:\n%s", want, got)
		}
	}
	for line := range strings.SplitSeq(string(got), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "script") {
			t.Errorf("Vorpal.toml has a script line %q by default, want none:\n%s", line, got)
		}
	}
}

func TestTerraformModuleVorpalProgramDefaults(t *testing.T) {
	terraformModuleRenderTemp(t, "demo-module", nil)

	program := terraformModuleParseVorpalProgram(t)

	if !slices.Equal(program.environmentNames, []string{"demo-module-dev"}) {
		t.Errorf("development environments = %q, want [\"demo-module-dev\"]", program.environmentNames)
	}
	for _, alias := range []string{"just:1.45.0", "terraform:1.13.1"} {
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

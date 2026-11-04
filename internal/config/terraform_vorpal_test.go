package config

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"slices"
	"strconv"
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

// terraformVorpalDevEnvironment parses the rendered Vorpal program and returns
// the name passed to artifact.NewDevelopmentEnvironment and every alias passed
// to FetchArtifactAlias.
func terraformVorpalDevEnvironment(t *testing.T, src string) (string, []string) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "cmd/vorpal/main.go", src, 0)
	if err != nil {
		t.Fatalf("rendered cmd/vorpal/main.go does not parse: %v\n%s", err, src)
	}

	var envName string
	var aliases []string
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
			t.Fatalf("unquoting %s: %v", lit.Value, err)
		}
		switch sel.Sel.Name {
		case "NewDevelopmentEnvironment":
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "artifact" {
				envName = value
			}
		case "FetchArtifactAlias":
			aliases = append(aliases, value)
		}
		return true
	})
	return envName, aliases
}

func TestTerraformRenderWritesVorpalConfig(t *testing.T) {
	terraformVorpalRender(t, nil)

	got := terraformVorpalReadFile(t, "Vorpal.toml")
	for _, want := range []string{
		`language = "go"`,
		`"cmd/vorpal"`,
		`script = "go mod init github.com/ALT-F4-LLC/demo && go mod tidy"`,
		"[source.go]\ndirectory = \"cmd/vorpal\"",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Vorpal.toml missing %q:\n%s", want, got)
		}
	}
}

func TestTerraformRenderWritesVorpalDevEnvironment(t *testing.T) {
	terraformVorpalRender(t, nil)

	envName, aliases := terraformVorpalDevEnvironment(t, terraformVorpalReadFile(t, "cmd/vorpal/main.go"))
	if envName != "dev" {
		t.Errorf("development environment name = %q, want %q", envName, "dev")
	}
	for _, want := range []string{"just:1.45.0", "terraform:1.13.1"} {
		if !slices.Contains(aliases, want) {
			t.Errorf("aliases %v missing %q", aliases, want)
		}
	}
}

func TestTerraformRenderVorpalTerraformVersionOverride(t *testing.T) {
	terraformVorpalRender(t, map[string]interface{}{
		"vorpal": map[string]interface{}{"terraformVersion": "1.12.0"},
	})

	_, aliases := terraformVorpalDevEnvironment(t, terraformVorpalReadFile(t, "cmd/vorpal/main.go"))
	if !slices.Contains(aliases, "terraform:1.12.0") {
		t.Errorf("aliases %v missing %q", aliases, "terraform:1.12.0")
	}
	if slices.Contains(aliases, "terraform:1.13.1") {
		t.Errorf("aliases %v still carry the default %q", aliases, "terraform:1.13.1")
	}
}

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
// the name passed to artifact.NewDevelopmentEnvironment, every alias passed to
// FetchArtifactAlias, and the aliases whose fetched variables are passed to
// WithArtifacts.
func terraformVorpalDevEnvironment(t *testing.T, src string) (string, []string, []string) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "cmd/vorpal/main.go", src, 0)
	if err != nil {
		t.Fatalf("rendered cmd/vorpal/main.go does not parse: %v\n%s", err, src)
	}

	stringArg := func(call *ast.CallExpr) (string, bool) {
		if len(call.Args) == 0 {
			return "", false
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return "", false
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			t.Fatalf("unquoting %s: %v", lit.Value, err)
		}
		return value, true
	}

	var envName string
	var aliases []string
	aliasByVar := map[string]string{}
	var artifactVars []string
	ast.Inspect(file, func(n ast.Node) bool {
		if assign, ok := n.(*ast.AssignStmt); ok && len(assign.Lhs) > 0 && len(assign.Rhs) == 1 {
			ident, isIdent := assign.Lhs[0].(*ast.Ident)
			call, isCall := assign.Rhs[0].(*ast.CallExpr)
			if isIdent && isCall {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "FetchArtifactAlias" {
					if value, ok := stringArg(call); ok {
						aliasByVar[ident.Name] = value
					}
				}
			}
			return true
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch sel.Sel.Name {
		case "NewDevelopmentEnvironment":
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "artifact" {
				if value, ok := stringArg(call); ok {
					envName = value
				}
			}
		case "FetchArtifactAlias":
			if value, ok := stringArg(call); ok {
				aliases = append(aliases, value)
			}
		case "WithArtifacts":
			if len(call.Args) == 1 {
				if list, ok := call.Args[0].(*ast.CompositeLit); ok {
					for _, elt := range list.Elts {
						if ident, ok := elt.(*ast.Ident); ok {
							artifactVars = append(artifactVars, ident.Name)
						}
					}
				}
			}
		}
		return true
	})

	var envArtifacts []string
	for _, name := range artifactVars {
		if alias, ok := aliasByVar[name]; ok {
			envArtifacts = append(envArtifacts, alias)
		}
	}
	return envName, aliases, envArtifacts
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

	envName, aliases, envArtifacts := terraformVorpalDevEnvironment(t, terraformVorpalReadFile(t, "cmd/vorpal/main.go"))
	if envName != "dev" {
		t.Errorf("development environment name = %q, want %q", envName, "dev")
	}
	for _, want := range []string{"just:1.45.0", "terraform:1.13.1", "awscli2:2.33.1"} {
		if !slices.Contains(aliases, want) {
			t.Errorf("aliases %v missing %q", aliases, want)
		}
		if !slices.Contains(envArtifacts, want) {
			t.Errorf("dev environment artifacts %v missing %q", envArtifacts, want)
		}
	}
}

func TestTerraformRenderVorpalTerraformVersionOverride(t *testing.T) {
	terraformVorpalRender(t, map[string]interface{}{
		"vorpal": map[string]interface{}{"terraformVersion": "1.12.0"},
	})

	_, aliases, _ := terraformVorpalDevEnvironment(t, terraformVorpalReadFile(t, "cmd/vorpal/main.go"))
	if !slices.Contains(aliases, "terraform:1.12.0") {
		t.Errorf("aliases %v missing %q", aliases, "terraform:1.12.0")
	}
	if slices.Contains(aliases, "terraform:1.13.1") {
		t.Errorf("aliases %v still carry the default %q", aliases, "terraform:1.13.1")
	}
}

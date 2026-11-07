package config

import (
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// goCobraCliRenderTemp renders a go-cobra-cli config named demo with the given
// parameters into a fresh temporary directory, which becomes the working
// directory for the rest of the test.
func goCobraCliRenderTemp(t *testing.T, parameters map[string]interface{}) {
	t.Helper()
	t.Chdir(t.TempDir())

	tpl, err := Config{Name: "demo", Template: GoCobraCliName, Parameters: parameters}.GetTemplater()
	if err != nil {
		t.Fatalf("GetTemplater: %v", err)
	}
	if err := tpl.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

func goCobraCliReadFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name) //nolint:gosec // reads a file the test rendered into its own temp dir
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

// goCobraCliStrings returns every string literal under node, in source order.
func goCobraCliStrings(t *testing.T, node ast.Node) []string {
	t.Helper()
	var out []string
	ast.Inspect(node, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			t.Fatalf("unquote %s: %v", lit.Value, err)
		}
		out = append(out, s)
		return false
	})
	return out
}

func goCobraCliIsCallTo(n ast.Node, fn string) bool {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == fn
}

// goCobraCliCallStrings returns the string literal arguments of every call to
// a method named fn in file, flattening composite literals such as
// []string{...}.
func goCobraCliCallStrings(t *testing.T, file *ast.File, fn string) []string {
	t.Helper()
	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		if goCobraCliIsCallTo(n, fn) {
			for _, arg := range n.(*ast.CallExpr).Args {
				out = append(out, goCobraCliStrings(t, arg)...)
			}
		}
		return true
	})
	return out
}

// goCobraCliFetchedAliases returns the aliases the program passes to
// FetchArtifactAlias: the string literals of the range expression of the loop
// whose body makes that call.
func goCobraCliFetchedAliases(t *testing.T, file *ast.File) []string {
	t.Helper()
	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		loop, ok := n.(*ast.RangeStmt)
		if !ok {
			return true
		}
		fetches := false
		ast.Inspect(loop.Body, func(m ast.Node) bool {
			fetches = fetches || goCobraCliIsCallTo(m, "FetchArtifactAlias")
			return !fetches
		})
		if fetches {
			out = append(out, goCobraCliStrings(t, loop.X)...)
		}
		return true
	})
	return out
}

// goCobraCliParseProgram reads the rendered Vorpal program, requires it to be
// gofmt-clean, and returns its syntax tree.
func goCobraCliParseProgram(t *testing.T) *ast.File {
	t.Helper()
	src := goCobraCliReadFile(t, "cmd/vorpal/main.go")

	formatted, err := format.Source([]byte(src))
	if err != nil {
		t.Fatalf("gofmt cmd/vorpal/main.go: %v\n%s", err, src)
	}
	if string(formatted) != src {
		t.Fatalf("cmd/vorpal/main.go is not gofmt-clean:\n%s", src)
	}

	file, err := parser.ParseFile(token.NewFileSet(), "cmd/vorpal/main.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse cmd/vorpal/main.go: %v\n%s", err, src)
	}
	return file
}

func goCobraCliAssertStrings(t *testing.T, what string, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("%s = %q, want %q", what, got, want)
	}
}

func TestGoCobraCliRendersDefaultVorpalToml(t *testing.T) {
	goCobraCliRenderTemp(t, nil)

	want := `language = "go"

[source]
includes = [
    "cmd/vorpal",
    "go.mod",
    "go.sum",
]

[source.go]
directory = "cmd/vorpal"
`
	if got := goCobraCliReadFile(t, "Vorpal.toml"); got != want {
		t.Errorf("Vorpal.toml =\n%s\nwant\n%s", got, want)
	}
}

func TestGoCobraCliRendersDefaultVorpalProgram(t *testing.T) {
	goCobraCliRenderTemp(t, nil)
	file := goCobraCliParseProgram(t)

	goCobraCliAssertStrings(t, "FetchArtifactAlias aliases", goCobraCliFetchedAliases(t, file),
		[]string{"golangci-lint:2.11.4", "govulncheck:1.9.0", "gitleaks:8.30.1", "rumdl:0.3.2"})
	goCobraCliAssertStrings(t, "dev environment name", goCobraCliCallStrings(t, file, "NewGoDevelopmentEnvironment"),
		[]string{"demo-dev"})
	goCobraCliAssertStrings(t, "artifact name", goCobraCliCallStrings(t, file, "NewGo"),
		[]string{"demo"})
	goCobraCliAssertStrings(t, "build directory", goCobraCliCallStrings(t, file, "WithBuildDirectory"),
		[]string{"cmd/demo"})
	goCobraCliAssertStrings(t, "artifact includes", goCobraCliCallStrings(t, file, "WithIncludes"),
		[]string{"cmd/demo", "go.mod", "go.sum", "internal"})
}

func TestGoCobraCliVorpalOptionsChangeOutputIndependently(t *testing.T) {
	goCobraCliRenderTemp(t, map[string]interface{}{
		"vorpal": map[string]interface{}{
			"tools":          []string{"just:1.0.0"},
			"buildDirectory": "cmd/other",
			"includes":       []string{"cmd/other", "go.mod", "pkg"},
			"sourceIncludes": []string{"cmd/vorpal", "go.mod", "go.sum", "vendor"},
		},
	})

	wantToml := `language = "go"

[source]
includes = [
    "cmd/vorpal",
    "go.mod",
    "go.sum",
    "vendor",
]

[source.go]
directory = "cmd/vorpal"
`
	if got := goCobraCliReadFile(t, "Vorpal.toml"); got != wantToml {
		t.Errorf("Vorpal.toml =\n%s\nwant\n%s", got, wantToml)
	}

	file := goCobraCliParseProgram(t)
	goCobraCliAssertStrings(t, "FetchArtifactAlias aliases", goCobraCliFetchedAliases(t, file),
		[]string{"just:1.0.0"})
	goCobraCliAssertStrings(t, "build directory", goCobraCliCallStrings(t, file, "WithBuildDirectory"),
		[]string{"cmd/other"})
	goCobraCliAssertStrings(t, "artifact includes", goCobraCliCallStrings(t, file, "WithIncludes"),
		[]string{"cmd/other", "go.mod", "pkg"})
}

func TestGoCobraCliRendersEmptyVorpalLists(t *testing.T) {
	goCobraCliRenderTemp(t, map[string]interface{}{
		"vorpal": map[string]interface{}{
			"tools":    []string{},
			"includes": []string{},
		},
	})

	file := goCobraCliParseProgram(t)
	goCobraCliAssertStrings(t, "FetchArtifactAlias aliases", goCobraCliFetchedAliases(t, file), nil)
	goCobraCliAssertStrings(t, "artifact includes", goCobraCliCallStrings(t, file, "WithIncludes"), nil)
}

func TestGoCobraCliLintConfigOmitsGoKitExclusion(t *testing.T) {
	goCobraCliRenderTemp(t, nil)

	got := goCobraCliReadFile(t, ".golangci.yaml")
	golangciParse(t, got)
	if strings.Contains(got, "go-kit") {
		t.Errorf(".golangci.yaml has a go-kit exclusion:\n%s", got)
	}
}

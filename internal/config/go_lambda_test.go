package config

import (
	"bytes"
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

// goLambdaRenderTemp renders the go-lambda template with the given lambdas
// into a fresh temporary directory, which becomes the working directory.
func goLambdaRenderTemp(t *testing.T, lambdas []string) {
	t.Helper()
	t.Chdir(t.TempDir())

	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Lambdas = lambdas
	if err := cfg.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

// goLambdaParseVorpalProgram parses the rendered cmd/vorpal/main.go.
func goLambdaParseVorpalProgram(t *testing.T) *ast.File {
	t.Helper()
	src, err := os.ReadFile("cmd/vorpal/main.go")
	if err != nil {
		t.Fatalf("reading cmd/vorpal/main.go: %v", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", src, 0)
	if err != nil {
		t.Fatalf("cmd/vorpal/main.go does not parse: %v\n%s", err, src)
	}
	return file
}

// goLambdaCallStringArgs returns the leading string-literal argument of every
// call to pkg.fn in file, in source order.
func goLambdaCallStringArgs(t *testing.T, file *ast.File, pkg, fn string) []string {
	t.Helper()
	var args []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		if !goLambdaIsSelector(call.Fun, pkg, fn) {
			return true
		}
		args = append(args, goLambdaStringLit(t, call.Args[0]))
		return true
	})
	return args
}

// goLambdaMethodStringArgs returns the string-literal argument of every
// method call named method in file, in source order.
func goLambdaMethodStringArgs(t *testing.T, file *ast.File, method string) []string {
	t.Helper()
	var args []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != method {
			return true
		}
		args = append(args, goLambdaStringLit(t, call.Args[0]))
		return true
	})
	return args
}

func goLambdaIsSelector(expr ast.Expr, pkg, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	return ok && ident.Name == pkg
}

func goLambdaStringLit(t *testing.T, expr ast.Expr) string {
	t.Helper()
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		t.Fatalf("argument %#v is not a string literal", expr)
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		t.Fatalf("unquoting %s: %v", lit.Value, err)
	}
	return s
}

// goLambdaStringLits returns every string literal in file.
func goLambdaStringLits(t *testing.T, file *ast.File) []string {
	t.Helper()
	var lits []string
	ast.Inspect(file, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			lits = append(lits, goLambdaStringLit(t, lit))
		}
		return true
	})
	return lits
}

func TestGoLambdaRenderVorpalToml(t *testing.T) {
	goLambdaRenderTemp(t, []string{"api", "worker"})

	b, err := os.ReadFile("Vorpal.toml")
	if err != nil {
		t.Fatalf("reading Vorpal.toml: %v", err)
	}
	got := string(b)

	for _, want := range []string{
		`language = "go"`,
		"[source.go]\ndirectory = \"cmd/vorpal\"",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Vorpal.toml missing %q:\n%s", want, got)
		}
	}

	_, afterIncludes, found := strings.Cut(got, "includes = [")
	if !found {
		t.Fatalf("Vorpal.toml has no includes array:\n%s", got)
	}
	includes, _, _ := strings.Cut(afterIncludes, "]")
	for _, want := range []string{`"cmd/vorpal"`, `"go.mod"`, `"go.sum"`, `"internal/vorpal"`} {
		if !strings.Contains(includes, want) {
			t.Errorf("[source] includes %q missing %s", includes, want)
		}
	}
}

func TestGoLambdaRenderVorpalProgramDevEnvironment(t *testing.T) {
	goLambdaRenderTemp(t, []string{"api", "worker"})
	file := goLambdaParseVorpalProgram(t)

	devEnvs := goLambdaCallStringArgs(t, file, "language", "NewGoDevelopmentEnvironment")
	if !slices.Equal(devEnvs, []string{"example-dev"}) {
		t.Errorf("development environments = %q, want [\"example-dev\"]", devEnvs)
	}

	lits := goLambdaStringLits(t, file)
	for _, want := range []string{
		"golangci-lint:2.11.4",
		"just:1.45.0",
		"GOPRIVATE=github.com/ALT-F4-LLC/quirk-service-kit",
	} {
		if !slices.Contains(lits, want) {
			t.Errorf("program has no string literal %q", want)
		}
	}

	// Scoped to vorpal.extension unset: a set extension imports the repo's own
	// package, which may legitimately end in internal/vorpal.
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(path, "internal/vorpal") {
			t.Errorf("program imports %q", path)
		}
	}
}

func TestGoLambdaRenderVorpalProgramArtifactPerLambda(t *testing.T) {
	for _, tc := range []struct {
		name    string
		lambdas []string
	}{
		{name: "one", lambdas: []string{"api"}},
		{name: "three", lambdas: []string{"api", "worker", "scheduler"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			goLambdaRenderTemp(t, tc.lambdas)
			file := goLambdaParseVorpalProgram(t)

			artifacts := goLambdaCallStringArgs(t, file, "language", "NewGo")
			if !slices.Equal(artifacts, tc.lambdas) {
				t.Errorf("lambda artifacts = %q, want %q", artifacts, tc.lambdas)
			}

			var wantDirs []string
			for _, lambda := range tc.lambdas {
				wantDirs = append(wantDirs, "cmd/"+lambda)
			}
			dirs := goLambdaMethodStringArgs(t, file, "WithBuildDirectory")
			if !slices.Equal(dirs, wantDirs) {
				t.Errorf("build directories = %q, want %q", dirs, wantDirs)
			}

			flags := goLambdaMethodStringArgs(t, file, "WithBuildFlags")
			for _, f := range flags {
				if f != "-ldflags='-s -w' -tags='lambda.norpc'" {
					t.Errorf("build flags = %q", f)
				}
			}
			if len(flags) != len(tc.lambdas) {
				t.Errorf("got %d build-flag calls, want %d", len(flags), len(tc.lambdas))
			}
			cgo := 0
			for _, lit := range goLambdaStringLits(t, file) {
				if lit == "CGO_ENABLED=0" {
					cgo++
				}
			}
			if cgo != len(tc.lambdas) {
				t.Errorf("CGO_ENABLED=0 appears %d times, want %d", cgo, len(tc.lambdas))
			}
		})
	}
}

func TestGoLambdaRenderVorpalProgramGofmtCleanForAnyTools(t *testing.T) {
	for _, tc := range []struct {
		name    string
		tools   []string
		openAPI bool
	}{
		{name: "default", tools: NewGoLambdaConfig(Config{}).Vorpal.Tools},
		{name: "one", tools: []string{"just:1.45.0"}},
		{name: "empty", tools: []string{}},
		{name: "default with openapi", tools: NewGoLambdaConfig(Config{}).Vorpal.Tools, openAPI: true},
		{name: "empty with openapi", tools: []string{}, openAPI: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
			cfg.Lambdas = []string{"api"}
			cfg.Vorpal.Tools = tc.tools
			cfg.OpenAPI.Enable = tc.openAPI
			if err := cfg.Render(); err != nil {
				t.Fatalf("Render: %v", err)
			}

			src, err := os.ReadFile("cmd/vorpal/main.go")
			if err != nil {
				t.Fatalf("reading cmd/vorpal/main.go: %v", err)
			}
			formatted, err := format.Source(src)
			if err != nil {
				t.Fatalf("gofmt cmd/vorpal/main.go: %v\n%s", err, src)
			}
			if !bytes.Equal(formatted, src) {
				t.Errorf("cmd/vorpal/main.go is not gofmt-clean:\n%s", src)
			}
		})
	}
}

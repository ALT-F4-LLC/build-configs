package config

import (
	"bytes"
	"go/ast"
	"go/format"
	"os"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

// goLambdaSourceVariants are the rendered program shapes the builder source
// must hold in: vorpal.extension on and off, crossed with privateModules on
// and off.
var goLambdaSourceVariants = []struct {
	name           string
	extension      string
	privateModules string
}{
	{name: "plain"},
	{name: "privateModules", privateModules: "github.com/example/private-kit"},
	{name: "extension", extension: goLambdaTestExtension},
	{name: "extension privateModules", extension: goLambdaTestExtension, privateModules: "github.com/example/private-kit"},
}

// goLambdaRenderSource renders go-lambda with two lambdas and a service into a
// fresh temporary directory and returns the parsed, gofmt-checked program.
func goLambdaRenderSource(t *testing.T, extension, privateModules string, includes []string) *ast.File {
	t.Helper()
	t.Chdir(t.TempDir())

	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Lambdas = []string{"api", "worker"}
	cfg.Services = []string{"svc"}
	cfg.PrivateModules = privateModules
	cfg.Vorpal.Extension = extension
	cfg.Vorpal.Includes = includes
	if err := cfg.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	src, err := os.ReadFile("cmd/vorpal/main.go")
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := format.Source(src)
	if err != nil {
		t.Fatalf("gofmt: %v\n%s", err, src)
	}
	if !bytes.Equal(formatted, src) {
		t.Errorf("cmd/vorpal/main.go is not gofmt-clean:\n%s", src)
	}

	return goLambdaParseVorpalProgram(t)
}


// goLambdaSourceFilter returns the string lists the source helper passes to
// WithExcludes and WithIncludes; a nil list means the call is absent.
func goLambdaSourceFilter(t *testing.T, file *ast.File) (excludes, includes []string) {
	t.Helper()
	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		if d, ok := decl.(*ast.FuncDecl); ok && d.Name.Name == "source" && d.Recv == nil {
			fn = d
		}
	}
	if fn == nil {
		t.Fatalf("program declares no source helper")
	}

	var roots int
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if goLambdaIsSelector(call.Fun, "artifact", "NewArtifactSource") {
			roots++
			if id, ok := call.Args[0].(*ast.Ident); !ok || id.Name != "name" {
				t.Errorf("NewArtifactSource name argument is not the helper's name parameter")
			}
			if got := goLambdaStringLit(t, call.Args[1]); got != "." {
				t.Errorf("NewArtifactSource path = %q, want the project root %q", got, ".")
			}
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != "WithExcludes" && sel.Sel.Name != "WithIncludes") {
			return true
		}
		lit, ok := call.Args[0].(*ast.CompositeLit)
		if !ok {
			t.Fatalf("%s argument is %T, want a []string literal", sel.Sel.Name, call.Args[0])
		}
		list := []string{}
		for _, elt := range lit.Elts {
			list = append(list, goLambdaStringLit(t, elt))
		}
		if sel.Sel.Name == "WithExcludes" {
			excludes = list
		} else {
			includes = list
		}
		return true
	})
	if roots != 1 {
		t.Errorf("source helper calls artifact.NewArtifactSource %d times, want 1", roots)
	}
	return excludes, includes
}

func goLambdaAssertBuilderSources(t *testing.T, file *ast.File) {
	t.Helper()
	loops := goLambdaBuildLoops(t, file)
	if got, want := goLambdaLoopNames(loops), []string{"api", "worker", "svc"}; !slices.Equal(got, want) {
		t.Fatalf("build loop names = %q, want %q", got, want)
	}
	for _, loop := range loops {
		if got, want := loop.Calls["WithSource"], "source("+loop.Var+")"; got != want {
			t.Errorf("loop over %s: WithSource(%s), want WithSource(%s)", loop.Var, got, want)
		}
	}
}

func TestGoLambdaVorpalSourceExcludesDist(t *testing.T) {
	for _, tc := range goLambdaSourceVariants {
		t.Run(tc.name, func(t *testing.T) {
			file := goLambdaRenderSource(t, tc.extension, tc.privateModules, nil)
			goLambdaAssertBuilderSources(t, file)

			excludes, includes := goLambdaSourceFilter(t, file)
			if !slices.Equal(excludes, goLambdaSourceExcludes) {
				t.Errorf("source excludes = %q, want %q", excludes, goLambdaSourceExcludes)
			}
			if includes != nil {
				t.Errorf("source includes = %q with vorpal.includes unset, want no WithIncludes call", includes)
			}
		})
	}
}

func TestGoLambdaVorpalSourceIncludes(t *testing.T) {
	includes := []string{"cmd", "go.mod", "go.sum", "internal"}
	for _, tc := range goLambdaSourceVariants {
		t.Run(tc.name, func(t *testing.T) {
			file := goLambdaRenderSource(t, tc.extension, tc.privateModules, includes)
			goLambdaAssertBuilderSources(t, file)

			gotExcludes, gotIncludes := goLambdaSourceFilter(t, file)
			if !slices.Equal(gotExcludes, goLambdaSourceExcludes) {
				t.Errorf("source excludes = %q, want %q", gotExcludes, goLambdaSourceExcludes)
			}
			if !slices.Equal(gotIncludes, includes) {
				t.Errorf("source includes = %q, want %q", gotIncludes, includes)
			}
		})
	}
}

func TestGoLambdaVorpalSourceNoBuildersNoHelper(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Lambdas = nil
	cfg.Vorpal.Includes = []string{"cmd"}
	if err := cfg.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	src, err := os.ReadFile("cmd/vorpal/main.go")
	if err != nil {
		t.Fatal(err)
	}
	// Unused imports or an unused helper would not compile or would fail lint.
	for _, banned := range []string{"func source(", "pkg/api/artifact\"", "pkg/artifact\"", "WithSource("} {
		if bytes.Contains(src, []byte(banned)) {
			t.Errorf("program with nothing to build contains %q:\n%s", banned, src)
		}
	}
	goLambdaParseVorpalProgram(t)
}

func TestGoLambdaVorpalIncludesParameter(t *testing.T) {
	const params = `name: example
template: go-lambda
parameters:
  vorpal:
    includes: [cmd, go.mod, go.sum]
`
	var c Config
	if err := yaml.Unmarshal([]byte(params), &c); err != nil {
		t.Fatal(err)
	}
	tpl, err := c.GetTemplater()
	if err != nil {
		t.Fatalf("GetTemplater: %v", err)
	}
	got, ok := tpl.(GoLambdaConfig)
	if !ok {
		t.Fatalf("templater is %T, want GoLambdaConfig", tpl)
	}
	if want := []string{"cmd", "go.mod", "go.sum"}; !slices.Equal(got.Vorpal.Includes, want) {
		t.Errorf("Vorpal.Includes = %q, want %q", got.Vorpal.Includes, want)
	}
	if def := NewGoLambdaConfig(Config{Name: "example"}); def.Vorpal.Includes != nil {
		t.Errorf("default Vorpal.Includes = %q, want unset", def.Vorpal.Includes)
	}
}

// goLambdaSourceExcludes is the source exclude list: dist, plus the local
// secret and tool paths Vorpal would otherwise upload with the source.
var goLambdaSourceExcludes = []string{"dist", ".env", ".direnv", ".claude"}

package config

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
	"gopkg.in/yaml.v3"
)

// goLibraryRenderTemp renders a go-library config named demo with the given
// parameters into a fresh temporary directory, which becomes the working
// directory for the rest of the test.
func goLibraryRenderTemp(t *testing.T, parameters map[string]any) {
	t.Helper()
	t.Chdir(t.TempDir())

	tpl, err := Config{Name: "demo", Template: GoLibraryName, Parameters: parameters}.GetTemplater()
	if err != nil {
		t.Fatalf("GetTemplater: %v", err)
	}
	if err := tpl.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

// goLibraryRenderedFiles returns every regular file under the working
// directory as a sorted, slash-separated path.
func goLibraryRenderedFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		files = append(files, filepath.ToSlash(p))
		return nil
	})
	if err != nil {
		t.Fatalf("walk rendered files: %v", err)
	}
	slices.Sort(files)
	return files
}

func TestGoLibraryRendersExactlyTheLibraryFiles(t *testing.T) {
	goLibraryRenderTemp(t, nil)

	want := []string{
		".editorconfig",
		".envrc",
		".github/renovate.json",
		".github/vorpal-shell.sh",
		".github/workflows/ci.yaml",
		".golangci.yaml",
		"Vorpal.toml",
		"cmd/vorpal/main.go",
		"justfile",
	}
	got := goLibraryRenderedFiles(t)
	t.Logf("go-library renders:\n%s", strings.Join(got, "\n"))
	if !slices.Equal(got, want) {
		t.Errorf("rendered files = %q, want %q", got, want)
	}
}

func TestGoLibraryRegisteredAsTemplate(t *testing.T) {
	tpl, err := Config{Name: "demo", Template: "go-library"}.GetTemplater()
	if err != nil {
		t.Fatalf("GetTemplater(go-library): %v", err)
	}
	if _, ok := tpl.(GoLibraryConfig); !ok {
		t.Errorf("GetTemplater(go-library) = %T, want GoLibraryConfig", tpl)
	}
}

func TestGoLibraryRendersCheckAndTestRecipes(t *testing.T) {
	goLibraryRenderTemp(t, nil)

	want := hashNoticeLines + "check:\n    golangci-lint run ./...\n\ntest:\n    go test -count=1 ./...\n"
	if got := goCobraCliReadFile(t, "justfile"); got != want {
		t.Errorf("justfile =\n%s\nwant\n%s", got, want)
	}
}

func TestGoLibraryRendersVorpalSourceTable(t *testing.T) {
	goLibraryRenderTemp(t, nil)

	want := hashNoticeLines + `language = "go"

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

type goLibraryWorkflow struct {
	Jobs map[string]struct {
		If          string            `yaml:"if"`
		Needs       []string          `yaml:"needs"`
		Permissions map[string]string `yaml:"permissions"`
		RunsOn      string            `yaml:"runs-on"`
		Steps       []struct {
			Run   string `yaml:"run"`
			Shell string `yaml:"shell"`
			Uses  string `yaml:"uses"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

func TestGoLibraryCIRunsCheckThenTestOnVorpalShell(t *testing.T) {
	goLibraryRenderTemp(t, nil)

	var wf goLibraryWorkflow
	if err := yaml.Unmarshal([]byte(goCobraCliReadFile(t, goCobraCliWorkflowPath)), &wf); err != nil {
		t.Fatalf("%s does not parse as YAML: %v", goCobraCliWorkflowPath, err)
	}

	jobs := make([]string, 0, len(wf.Jobs))
	for name := range wf.Jobs {
		jobs = append(jobs, name)
	}
	slices.Sort(jobs)
	if want := []string{"check", "test"}; !slices.Equal(jobs, want) {
		t.Fatalf("jobs = %q, want %q (a library has no build job)", jobs, want)
	}

	for name, run := range map[string]string{"check": "just check", "test": "just test"} {
		job := wf.Jobs[name]
		if job.RunsOn != "altf4llc" {
			t.Errorf("job %s runs-on = %q, want altf4llc", name, job.RunsOn)
		}
		if want := "${{ " + goCobraCliForkGuard + " }}"; job.If != want {
			t.Errorf("job %s if = %q, want %q", name, job.If, want)
		}
		if job.Permissions["contents"] != "read" {
			t.Errorf("job %s permissions = %v, want contents: read", name, job.Permissions)
		}
		var ran bool
		for _, step := range job.Steps {
			if step.Run != run {
				continue
			}
			ran = true
			if step.Shell != goCobraCliStepShell {
				t.Errorf("job %s step %q shell = %q, want %q", name, run, step.Shell, goCobraCliStepShell)
			}
		}
		if !ran {
			t.Errorf("job %s has no step running %q", name, run)
		}
	}

	if got, want := wf.Jobs["test"].Needs, []string{"check"}; !slices.Equal(got, want) {
		t.Errorf("job test needs = %q, want %q", got, want)
	}
	if got := wf.Jobs["check"].Needs; len(got) != 0 {
		t.Errorf("job check needs = %q, want none", got)
	}
}

func TestGoLibraryRendersExecutableVorpalShell(t *testing.T) {
	goLibraryRenderTemp(t, nil)

	info, err := os.Stat(goCobraCliVorpalShellPath)
	if err != nil {
		t.Fatalf("%s was not written: %v", goCobraCliVorpalShellPath, err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Errorf("%s mode = %#o, want %#o", goCobraCliVorpalShellPath, got, 0o755)
	}
	if got := goCobraCliReadFile(t, goCobraCliVorpalShellPath); !strings.Contains(got, `vorpal build --path 'demo-dev'`) {
		t.Errorf("%s does not activate the demo-dev environment:\n%s", goCobraCliVorpalShellPath, got)
	}
}

func TestGoLibraryRendersSharedGolangciConfigUnchanged(t *testing.T) {
	goLibraryRenderTemp(t, nil)

	want, err := templates.RenderTemplate(templates.GoCommonTemplates, ".golangci.yaml", NewGoLibraryConfig(Config{Name: "demo"}))
	if err != nil {
		t.Fatalf("render shared .golangci.yaml: %v", err)
	}
	got := goCobraCliReadFile(t, ".golangci.yaml")
	if got != want {
		t.Errorf(".golangci.yaml differs from the shared Go template:\n%s\nwant\n%s", got, want)
	}
	if !strings.Contains(got, "default: standard") || strings.Contains(got, "rules:") {
		t.Errorf(".golangci.yaml should be the shared standard set with no exclusion rules by default:\n%s", got)
	}
}

func TestGoLibraryLintExcludeRendersIntoGolangciConfig(t *testing.T) {
	goLibraryRenderTemp(t, map[string]any{
		"lint": map[string]any{"exclude": []string{"Error return value of `x` is not checked"}},
	})

	if got := goCobraCliReadFile(t, ".golangci.yaml"); !strings.Contains(got, "Error return value of `x` is not checked") {
		t.Errorf(".golangci.yaml does not carry lint.exclude:\n%s", got)
	}
}

// goLibraryVorpalProgram parses the rendered cmd/vorpal/main.go and returns it
// with its source.
func goLibraryVorpalProgram(t *testing.T) (*ast.File, string) {
	t.Helper()
	src := goCobraCliReadFile(t, "cmd/vorpal/main.go")
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("cmd/vorpal/main.go does not parse: %v\n%s", err, src)
	}
	return file, src
}

func TestGoLibraryVorpalProgramBuildsOnlyTheDevEnvironment(t *testing.T) {
	goLibraryRenderTemp(t, nil)

	file, src := goLibraryVorpalProgram(t)

	var calls []string
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				calls = append(calls, sel.Sel.Name)
			}
		}
		return true
	})
	if !slices.Contains(calls, "NewGoDevelopmentEnvironment") {
		t.Errorf("cmd/vorpal/main.go never calls NewGoDevelopmentEnvironment:\n%s", src)
	}
	if slices.Contains(calls, "NewGo") {
		t.Errorf("cmd/vorpal/main.go builds a Go artifact, want only the dev environment:\n%s", src)
	}
	if want := `NewGoDevelopmentEnvironment("demo-dev", config.SYSTEMS)`; !strings.Contains(src, want) {
		t.Errorf("cmd/vorpal/main.go does not contain %s:\n%s", want, src)
	}
}

func TestGoLibraryVorpalProgramFetchesTheDefaultTools(t *testing.T) {
	goLibraryRenderTemp(t, nil)

	file, _ := goLibraryVorpalProgram(t)
	got := goCobraCliStrings(t, file)
	for _, tool := range NewGoCobraCliConfig(Config{Name: "demo"}).Vorpal.Tools {
		if !slices.Contains(got, tool) {
			t.Errorf("cmd/vorpal/main.go does not fetch the go-cobra-cli default tool %q", tool)
		}
	}
	if want := NewGoCobraCliConfig(Config{Name: "demo"}).Vorpal.Tools; !slices.Equal(NewGoLibraryConfig(Config{Name: "demo"}).Vorpal.Tools, want) {
		t.Errorf("go-library tools = %q, want go-cobra-cli's %q", NewGoLibraryConfig(Config{Name: "demo"}).Vorpal.Tools, want)
	}
}

func TestGoLibraryVorpalToolsReplaceTheDefaults(t *testing.T) {
	goLibraryRenderTemp(t, map[string]any{
		"vorpal": map[string]any{"tools": []string{"just:1.45.0"}},
	})

	file, src := goLibraryVorpalProgram(t)
	got := goCobraCliStrings(t, file)
	if !slices.Contains(got, "just:1.45.0") {
		t.Errorf("cmd/vorpal/main.go does not fetch the configured tool:\n%s", src)
	}
	if slices.Contains(got, "golangci-lint:2.11.4") {
		t.Errorf("cmd/vorpal/main.go still fetches a default tool after vorpal.tools was set:\n%s", src)
	}
}

func TestGoLibraryHonorsBcignore(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(".bcignore", []byte("justfile\n"), 0o600); err != nil {
		t.Fatalf("write .bcignore: %v", err)
	}
	if err := os.WriteFile("justfile", []byte("custom\n"), 0o600); err != nil {
		t.Fatalf("write justfile: %v", err)
	}

	tpl, err := Config{Name: "demo", Template: GoLibraryName}.GetTemplater()
	if err != nil {
		t.Fatalf("GetTemplater: %v", err)
	}
	if err := tpl.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	if got := goCobraCliReadFile(t, "justfile"); got != "custom\n" {
		t.Errorf("justfile listed in .bcignore was overwritten:\n%s", got)
	}
	if _, err := os.Stat("Vorpal.toml"); err != nil {
		t.Errorf("Vorpal.toml was not written next to an ignored justfile: %v", err)
	}
}

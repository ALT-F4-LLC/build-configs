package config

import (
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"
)

// goTemplates are tied to the Go language, so their Vorpal config program is Go.
var goTemplates = []string{GoCobraCliName, GoLambdaName, GoLibraryName, GoServiceName}

// typeScriptTemplates are every other template: the ones not tied to a
// programming language (terraform and terraform-module) and the TypeScript
// frontend. Their Vorpal config program is TypeScript on @altf4llc/vorpal-sdk.
var typeScriptTemplates = []string{TerraformName, TerraformModuleName, TypeScriptFrontendName}

// A new template directory must declare which side it is on, so a template
// that is not tied to a language cannot default to a Go config program.
func TestEveryTemplateDeclaresItsVorpalLanguage(t *testing.T) {
	entries, err := fs.ReadDir(os.DirFS(templatesRoot), ".")
	if err != nil {
		t.Fatalf("reading %s: %v", templatesRoot, err)
	}

	seen := 0
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "common" {
			continue
		}
		seen++
		if !slices.Contains(goTemplates, e.Name()) && !slices.Contains(typeScriptTemplates, e.Name()) {
			t.Errorf("template %q is in neither goTemplates nor typeScriptTemplates: a template not tied to a programming language must use the TypeScript SDK", e.Name())
		}
	}
	if want := len(goTemplates) + len(typeScriptTemplates); seen != want {
		t.Errorf("found %d template directories, the two lists name %d", seen, want)
	}
}

// Every template not tied to a Go program renders the TypeScript config and no
// Go file, so an adopting repository needs no Go toolchain.
func TestTypeScriptTemplatesRenderTypeScriptDefaults(t *testing.T) {
	for _, template := range typeScriptTemplates {
		t.Run(template, func(t *testing.T) {
			t.Chdir(t.TempDir())

			tpl, err := Config{Name: "demo", Template: template}.GetTemplater()
			if err != nil {
				t.Fatalf("GetTemplater: %v", err)
			}
			if err := tpl.Render(); err != nil {
				t.Fatalf("Render: %v", err)
			}

			toml, err := os.ReadFile("Vorpal.toml")
			if err != nil {
				t.Fatalf("Vorpal.toml was not written: %v", err)
			}
			if !strings.Contains(string(toml), `language = "typescript"`) || !strings.Contains(string(toml), "[source.typescript]") {
				t.Errorf("Vorpal.toml is not a TypeScript config:\n%s", toml)
			}
			program, err := os.ReadFile("src/vorpal.ts")
			if err != nil {
				t.Fatalf("src/vorpal.ts was not written: %v", err)
			}
			if !strings.Contains(string(program), `from "@altf4llc/vorpal-sdk"`) {
				t.Errorf("src/vorpal.ts does not import @altf4llc/vorpal-sdk:\n%s", program)
			}
			for _, goFile := range []string{"cmd/vorpal/main.go", "go.mod", "go.sum"} {
				if _, err := os.Stat(goFile); err == nil {
					t.Errorf("%s was rendered; the config program is TypeScript", goFile)
				}
			}
		})
	}
}

// terraform and terraform-module are not tied to a language, so they also
// scaffold the Bun project files the TypeScript config builds from.
func TestNonLanguageTemplatesScaffoldBunProject(t *testing.T) {
	for _, template := range []string{TerraformName, TerraformModuleName} {
		t.Run(template, func(t *testing.T) {
			t.Chdir(t.TempDir())
			terraformScaffoldRender(t, template, "demo")

			for _, name := range []string{"package.json", "bun.lock", "tsconfig.json"} {
				if _, err := os.Stat(name); err != nil {
					t.Errorf("%s was not scaffolded: %v", name, err)
				}
			}
		})
	}
}

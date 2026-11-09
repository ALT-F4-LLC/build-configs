package config

import (
	"io/fs"
	"os"
	"strings"
	"testing"
)

const templatesRoot = "../templates/templates"

var nixTerms = []string{"nix", "flake", "cachix"}

func TestTemplatesContainNoNixReferences(t *testing.T) {
	templates := os.DirFS(templatesRoot)
	walked := 0
	err := fs.WalkDir(templates, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if term, found := containsNixTerm(path); found {
			t.Errorf("template path %q contains %q", path, term)
		}

		if d.IsDir() {
			return nil
		}
		walked++

		content, err := fs.ReadFile(templates, path)
		if err != nil {
			return err
		}
		if term, found := containsNixTerm(string(content)); found {
			t.Errorf("template %q content contains %q", path, term)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", templatesRoot, err)
	}
	if walked == 0 {
		t.Fatalf("walk %s found no template files", templatesRoot)
	}
}

func containsNixTerm(s string) (string, bool) {
	lower := strings.ToLower(s)
	for _, term := range nixTerms {
		if strings.Contains(lower, term) {
			return term, true
		}
	}
	return "", false
}

package config

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// justfileRecipeHeader matches a top-level recipe header and captures its name.
var justfileRecipeHeader = regexp.MustCompile(`(?m)^@?([A-Za-z_][A-Za-z0-9_-]*)(?:[ \t][^:\n]*)?:(?:[^=\n]|$)`)

func justfileRecipeNames(src string) []string {
	var names []string
	for _, m := range justfileRecipeHeader.FindAllStringSubmatch(src, -1) {
		names = append(names, m[1])
	}
	return names
}

// TestGoTemplatesSupportRootJustfileRecipes checks that every Go template
// renders each recipe this repository's own justfile defines, apart from the
// recipes a template has no artifact for.
func TestGoTemplatesSupportRootJustfileRecipes(t *testing.T) {
	root, err := os.ReadFile(filepath.Join("..", "..", "justfile"))
	if err != nil {
		t.Fatalf("read root justfile: %v", err)
	}
	want := justfileRecipeNames(string(root))
	if len(want) == 0 {
		t.Fatal("root justfile has no recipes")
	}

	cases := []struct {
		name   string
		render func(t *testing.T)
		exempt []string
	}{
		{GoCobraCliName, func(t *testing.T) { goCobraCliRenderTemp(t, nil) }, nil},
		// A library has no main package to install; package builds its dev environment.
		{GoLibraryName, func(t *testing.T) { goLibraryRenderTemp(t, nil) }, []string{"install"}},
		// Lambda binaries are bootstraps, not tools to install.
		{GoLambdaName, func(t *testing.T) { goLambdaRenderTemp(t, []string{"api"}) }, []string{"install"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.render(t)
			got := justfileRecipeNames(goCobraCliReadFile(t, "justfile"))
			for _, recipe := range want {
				if slices.Contains(tc.exempt, recipe) {
					continue
				}
				if !slices.Contains(got, recipe) {
					t.Errorf("justfile lacks recipe %q; has %q", recipe, got)
				}
			}
		})
	}
}

func TestGoCobraCliInstallUsesBuildDirectory(t *testing.T) {
	goCobraCliRenderTemp(t, map[string]interface{}{"vorpal": map[string]interface{}{"buildDirectory": "cmd/other"}})
	got := goCobraCliReadFile(t, "justfile")
	if want := "install:\n    go install './cmd/other'\n"; !strings.Contains(got, want) {
		t.Errorf("justfile lacks %q:\n%s", want, got)
	}
}

func TestJustfileRecipeNames(t *testing.T) {
	src := "set shell := [\"bash\", \"-c\"]\nx := \"1\"\nexport FOO := \"a\"\nalias b := build\n" +
		"build profile:\n    vorpal build\n@quiet:\n    true\nfoo_bar:\n_private:\n" +
		"check: generate build-client\nbuild-target target=\"a:b\": dep\n"
	want := []string{"build", "quiet", "foo_bar", "_private", "check", "build-target"}
	if got := justfileRecipeNames(src); !slices.Equal(got, want) {
		t.Errorf("justfileRecipeNames = %q, want %q", got, want)
	}
}

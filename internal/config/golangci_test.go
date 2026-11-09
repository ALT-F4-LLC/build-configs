package config

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
	"gopkg.in/yaml.v3"
)

type golangciRenderContext struct {
	Lint GolangCILintConfig
}

func golangciRender(t *testing.T, name string, ctx golangciRenderContext) string {
	t.Helper()
	out, err := templates.RenderTemplate(templates.GoCommonTemplates, name, ctx)
	if err != nil {
		t.Fatalf("render %s: %v", name, err)
	}
	return out
}

func golangciParse(t *testing.T, out string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("rendered output is not valid YAML: %v\n%s", err, out)
	}
	return doc
}

func TestGolangciConfigWithoutExcludesMatchesRepositoryConfig(t *testing.T) {
	want, err := os.ReadFile("../../.golangci.yaml")
	if err != nil {
		t.Fatal(err)
	}

	got := golangciRender(t, ".golangci.yaml", golangciRenderContext{
		Lint: GolangCILintConfig{Exclude: []string{}},
	})

	// The repository copy may or may not carry the notice yet; compare the
	// configuration alone.
	if strings.Replace(got, hashNoticeLines, "", 1) != strings.Replace(string(want), hashNoticeLines, "", 1) {
		t.Errorf("rendered .golangci.yaml differs from repository .golangci.yaml\ngot:\n%s\nwant:\n%s", got, want)
	}
	if _, ok := golangciParse(t, got)["issues"]; ok {
		t.Errorf("rendered .golangci.yaml has a v1 issues key:\n%s", got)
	}
}

func TestGolangciConfigExcludesRenderAsTextRules(t *testing.T) {
	excludes := []string{
		"Error return value of `\\(github.com/go-kit/log.Logger\\).Log` is not checked",
		"G104: Errors unhandled",
	}
	extra := "exported .* should have comment: x # y"

	out := golangciRender(t, ".golangci.yaml", golangciRenderContext{
		Lint: GolangCILintConfig{Exclude: excludes, ExtraExclude: []string{extra}},
	})

	var doc struct {
		Version string         `yaml:"version"`
		Issues  map[string]any `yaml:"issues"`
		Linters struct {
			Exclusions struct {
				Presets []string `yaml:"presets"`
				Rules   []struct {
					Path string `yaml:"path"`
					Text string `yaml:"text"`
				} `yaml:"rules"`
			} `yaml:"exclusions"`
		} `yaml:"linters"`
	}
	if err := yaml.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("rendered output is not valid YAML: %v\n%s", err, out)
	}

	if doc.Version != "2" {
		t.Errorf("version = %q, want \"2\"", doc.Version)
	}
	if doc.Issues != nil || strings.Contains(out, "issues:") {
		t.Errorf("rendered .golangci.yaml has a v1 issues key:\n%s", out)
	}
	if !slices.Equal(doc.Linters.Exclusions.Presets, []string{"comments"}) {
		t.Errorf("presets = %q, want [comments]", doc.Linters.Exclusions.Presets)
	}

	var texts []string
	for _, rule := range doc.Linters.Exclusions.Rules {
		texts = append(texts, rule.Text)
		// golangci-lint v2 rejects an exclusion rule with fewer than two conditions.
		if rule.Path == "" {
			t.Errorf("rule for text %q has no path condition", rule.Text)
		}
	}
	want := append(slices.Clone(excludes), extra)
	if !slices.Equal(texts, want) {
		t.Errorf("exclusion rule texts = %q, want %q\n%s", texts, want, out)
	}
}

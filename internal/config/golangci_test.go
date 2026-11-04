package config

import (
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
	"gopkg.in/yaml.v3"
)

type golangciRenderContext struct {
	Lint           GolangCILintConfig
	PrivateModules string
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

	if got != string(want) {
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

func TestGolangciLintWorkflowUsesV2CapableAction(t *testing.T) {
	for _, privateModules := range []string{"", "github.com/example/private"} {
		t.Run("privateModules="+strconv.Quote(privateModules), func(t *testing.T) {
			out := golangciRender(t, ".github/workflows/golangci-lint.yaml", golangciRenderContext{
				PrivateModules: privateModules,
			})
			if strings.Contains(out, "skip-pkg-cache") {
				t.Errorf("workflow still sets skip-pkg-cache:\n%s", out)
			}

			var doc struct {
				Jobs map[string]struct {
					Steps []struct {
						Uses string            `yaml:"uses"`
						With map[string]string `yaml:"with"`
					} `yaml:"steps"`
				} `yaml:"jobs"`
			}
			if err := yaml.Unmarshal([]byte(out), &doc); err != nil {
				t.Fatalf("rendered workflow is not valid YAML: %v\n%s", err, out)
			}

			var found bool
			for _, step := range doc.Jobs["lint"].Steps {
				ref, ok := strings.CutPrefix(step.Uses, "golangci/golangci-lint-action@v")
				if !ok {
					continue
				}
				found = true
				major, err := strconv.Atoi(strings.SplitN(ref, ".", 2)[0])
				if err != nil || major < 7 {
					t.Errorf("golangci-lint action %q, want major v7 or later", step.Uses)
				}
				if got := step.With["args"]; got != "./... --timeout=5m" {
					t.Errorf("golangci-lint action args = %q, want %q", got, "./... --timeout=5m")
				}
			}
			if !found {
				t.Errorf("no golangci/golangci-lint-action step in lint job:\n%s", out)
			}
		})
	}
}

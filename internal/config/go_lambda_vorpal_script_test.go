package config

import (
	"os"
	"strings"
	"testing"
)

func goLambdaRenderVorpalScript(t *testing.T, script string) string {
	t.Helper()
	t.Chdir(t.TempDir())

	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Vorpal.Script = script
	if err := cfg.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	b, err := os.ReadFile("Vorpal.toml")
	if err != nil {
		t.Fatalf("reading Vorpal.toml: %v", err)
	}
	return string(b)
}

func TestGoLambdaRenderVorpalScriptOverride(t *testing.T) {
	got := goLambdaRenderVorpalScript(t, "make deps")

	if want := `script = "make deps"`; !strings.Contains(got, want) {
		t.Errorf("Vorpal.toml missing %q:\n%s", want, got)
	}
	if strings.Contains(got, "go mod init") {
		t.Errorf("Vorpal.toml still carries the default script:\n%s", got)
	}
}

func TestGoLambdaRenderVorpalScriptUnsetOmitsLine(t *testing.T) {
	goLambdaRenderTemp(t, []string{"api"})

	b, err := os.ReadFile("Vorpal.toml")
	if err != nil {
		t.Fatalf("reading Vorpal.toml: %v", err)
	}
	for line := range strings.SplitSeq(string(b), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "script") {
			t.Errorf("Vorpal.toml has a script line %q by default, want none:\n%s", line, b)
		}
	}
}

func TestGoLambdaRenderVorpalScriptExplicitEmptyOmitsLine(t *testing.T) {
	got := goLambdaRenderVorpalScript(t, "")

	for line := range strings.SplitSeq(got, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "script") {
			t.Errorf("Vorpal.toml has a script line %q, want none:\n%s", line, got)
		}
	}
}

func TestGoLambdaGetTemplaterVorpalScriptParameters(t *testing.T) {
	for name, tc := range map[string]struct {
		params map[string]any
		want   string
	}{
		"unset":    {nil, ""},
		"override": {map[string]any{"vorpal": map[string]any{"script": "make deps"}}, "make deps"},
		"empty":    {map[string]any{"vorpal": map[string]any{"script": ""}}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			tpl, err := Config{Name: "example", Template: GoLambdaName, Parameters: tc.params}.GetTemplater()
			if err != nil {
				t.Fatalf("GetTemplater: %v", err)
			}
			if got := tpl.(GoLambdaConfig).Vorpal.Script; got != tc.want {
				t.Errorf("Vorpal.Script = %q, want %q", got, tc.want)
			}
		})
	}
}

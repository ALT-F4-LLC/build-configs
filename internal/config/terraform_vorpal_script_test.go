package config

import (
	"strings"
	"testing"
)

func TestTerraformRenderVorpalScriptOverride(t *testing.T) {
	terraformVorpalRender(t, map[string]any{
		"vorpal": map[string]any{"script": "make deps"},
	})

	got := terraformVorpalReadFile(t, "Vorpal.toml")
	if want := `script = "make deps"`; !strings.Contains(got, want) {
		t.Errorf("Vorpal.toml missing %q:\n%s", want, got)
	}
	if strings.Contains(got, "go mod init") {
		t.Errorf("Vorpal.toml still carries the default script:\n%s", got)
	}
}

func TestTerraformRenderVorpalScriptUnsetOmitsLine(t *testing.T) {
	terraformVorpalRender(t, nil)

	got := terraformVorpalReadFile(t, "Vorpal.toml")
	for line := range strings.SplitSeq(got, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "script") {
			t.Errorf("Vorpal.toml has a script line %q by default, want none:\n%s", line, got)
		}
	}
}

func TestTerraformRenderVorpalScriptExplicitEmptyOmitsLine(t *testing.T) {
	terraformVorpalRender(t, map[string]any{
		"vorpal": map[string]any{"script": ""},
	})

	got := terraformVorpalReadFile(t, "Vorpal.toml")
	for line := range strings.SplitSeq(got, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "script") {
			t.Errorf("Vorpal.toml has a script line %q, want none:\n%s", line, got)
		}
	}
}

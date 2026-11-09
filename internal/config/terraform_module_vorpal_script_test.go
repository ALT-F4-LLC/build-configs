package config

import (
	"os"
	"strings"
	"testing"
)

func TestTerraformModuleRendersVorpalScriptOverride(t *testing.T) {
	terraformModuleRenderTemp(t, "demo-module", map[string]any{
		"vorpal": map[string]any{"script": "make deps"},
	})

	got, err := os.ReadFile("Vorpal.toml")
	if err != nil {
		t.Fatalf("Vorpal.toml was not written: %v", err)
	}
	if want := `script = "make deps"`; !strings.Contains(string(got), want) {
		t.Errorf("Vorpal.toml missing %q:\n%s", want, got)
	}
	if strings.Contains(string(got), "go mod init") {
		t.Errorf("Vorpal.toml still carries the default script:\n%s", got)
	}
}

func TestTerraformModuleRendersVorpalScriptExplicitEmptyOmitsLine(t *testing.T) {
	terraformModuleRenderTemp(t, "demo-module", map[string]any{
		"vorpal": map[string]any{"script": ""},
	})

	got, err := os.ReadFile("Vorpal.toml")
	if err != nil {
		t.Fatalf("Vorpal.toml was not written: %v", err)
	}
	for line := range strings.SplitSeq(string(got), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "script") {
			t.Errorf("Vorpal.toml has a script line %q, want none:\n%s", line, got)
		}
	}
}

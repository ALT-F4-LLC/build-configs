package config

import (
	"strings"
	"testing"
)

func TestTerraformRenderWritesVorpalEnvrc(t *testing.T) {
	terraformVorpalRender(t, nil)

	got := terraformVorpalReadFile(t, ".envrc")
	for _, want := range []string{
		"use_vorpal() {",
		`source "$(vorpal build --path 'demo-dev')/bin/activate"`,
		"\nuse vorpal\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf(".envrc missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "flake") {
		t.Errorf(".envrc still references flake:\n%s", got)
	}
}

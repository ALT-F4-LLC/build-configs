package config

import (
	"strings"
	"testing"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
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

// The render map is a Go map, so a file listed under two template sets is
// written by whichever set iterates last; listing .envrc once, under the
// terraform set, keeps the rendered file deterministic.
func TestTerraformRenderMapListsEnvrcOnlyUnderTerraform(t *testing.T) {
	for set, files := range NewTerraformConfig(Config{Name: "demo"}).renderMap() {
		count := 0
		for _, f := range files {
			if f == ".envrc" {
				count++
			}
		}
		want := 0
		if set == templates.TerraformTemplates {
			want = 1
		}
		if count != want {
			t.Errorf(".envrc listed %d times under template set %q, want %d", count, set.Name(), want)
		}
	}
}

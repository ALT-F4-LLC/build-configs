package config

import (
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

const terraformModuleEnvrcWant = `use_vorpal() {
    source "$(vorpal build --path 'dev')/bin/activate"
}

use vorpal
`

func TestTerraformModuleRendersVorpalEnvrc(t *testing.T) {
	terraformModuleRenderTemp(t, "demo-module", nil)

	if got := terraformModuleReadFile(t, ".envrc"); got != terraformModuleEnvrcWant {
		t.Errorf(".envrc =\n%s\nwant\n%s", got, terraformModuleEnvrcWant)
	}
}

// The render map is a Go map, so a file listed under two template sets is
// written by whichever set iterates last; .envrc must come only from the
// terraform-module set.
func TestTerraformModuleRenderMapListsEnvrcOnlyUnderTerraformModule(t *testing.T) {
	own, shared := 0, 0
	for set, files := range NewTerraformModuleConfig(Config{Name: "demo-module"}).renderMap() {
		for _, file := range files {
			if file != ".envrc" {
				continue
			}
			if set == templates.TerraformModuleTemplates {
				own++
			} else {
				shared++
			}
		}
	}
	if own != 1 || shared != 0 {
		t.Errorf(".envrc render entries: terraform-module set %d, shared sets %d; want 1 and 0", own, shared)
	}
}

func TestTerraformModuleRendersNoFlake(t *testing.T) {
	terraformModuleRenderTemp(t, "demo-module", nil)

	if _, err := os.Stat("flake.nix"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("flake.nix stat error = %v, want it absent", err)
	}
}

func TestTerraformModuleRendersConfigWithLegacyProvidersKey(t *testing.T) {
	terraformModuleRenderTemp(t, "demo-module", map[string]any{
		"providers": []any{"aws"},
	})

	if got := terraformModuleReadFile(t, ".envrc"); got != terraformModuleEnvrcWant {
		t.Errorf(".envrc with legacy providers key =\n%s\nwant\n%s", got, terraformModuleEnvrcWant)
	}
	if _, err := os.Stat("flake.nix"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("flake.nix stat error with legacy providers key = %v, want it absent", err)
	}
}

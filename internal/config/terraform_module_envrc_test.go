package config

import (
	"errors"
	"io/fs"
	"os"
	"testing"
)

const terraformModuleEnvrcWant = `use_vorpal() {
    source "$(vorpal build --path 'demo-module-dev')/bin/activate"
}

use vorpal
`

func TestTerraformModuleRendersVorpalEnvrc(t *testing.T) {
	terraformModuleRenderTemp(t, "demo-module", nil)

	if got := terraformModuleReadFile(t, ".envrc"); got != terraformModuleEnvrcWant {
		t.Errorf(".envrc =\n%s\nwant\n%s", got, terraformModuleEnvrcWant)
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

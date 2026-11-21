package config

import (
	"errors"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

func TestTerraformRendersNoFlake(t *testing.T) {
	terraformVorpalRender(t, nil)

	if _, err := os.Stat("flake.nix"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("flake.nix stat error = %v, want it absent", err)
	}
	if templates.TerraformTemplates.Lookup("flake.nix") != nil {
		t.Errorf("terraform template set still embeds flake.nix")
	}
}

func TestTerraformJustfileChecksFormattingWithoutNix(t *testing.T) {
	terraformVorpalRender(t, nil)

	got := terraformVorpalReadFile(t, "justfile")
	lower := strings.ToLower(got)
	for _, banned := range []string{"nix", "cachix", "flake"} {
		if strings.Contains(lower, banned) {
			t.Errorf("justfile contains %q:\n%s", banned, got)
		}
	}

	var body []string
	inCheck := false
	for line := range strings.SplitSeq(got, "\n") {
		switch {
		case line == "check:":
			inCheck = true
		case inCheck && strings.HasPrefix(line, "    "):
			body = append(body, strings.TrimSpace(line))
		case inCheck:
			inCheck = false
		}
	}
	if len(body) != 1 || body[0] != "terraform fmt -check" {
		t.Errorf("justfile check recipe = %q, want [\"terraform fmt -check\"]:\n%s", body, got)
	}
}

func TestTerraformConfigDropsNixSettings(t *testing.T) {
	for _, tc := range []struct {
		typ   reflect.Type
		field string
	}{
		{reflect.TypeFor[TerraformConfig](), "Nix"},
		{reflect.TypeFor[TerraformConfig](), "Providers"},
		{reflect.TypeFor[GitHubConfigAction](), "SetupNix"},
	} {
		if _, ok := tc.typ.FieldByName(tc.field); ok {
			t.Errorf("%s still has a %s field", tc.typ.Name(), tc.field)
		}
	}
}

// Config decoding ignores unknown keys, so a consumer config written for the
// Nix template still renders and its Nix settings leave no trace.
func TestTerraformRendersConfigWithLegacyNixKeys(t *testing.T) {
	terraformVorpalRender(t, map[string]interface{}{
		"github": map[string]interface{}{
			"action": map[string]interface{}{"setupNix": true},
		},
		"nix": map[string]interface{}{
			"cachix":        map[string]interface{}{"binaryCache": "legacy-binary-cache"},
			"nixpkgsBranch": "legacy-nixpkgs-branch",
			"systems":       []interface{}{"legacy-system"},
		},
		"providers": []interface{}{"legacy-provider"},
	})

	if _, err := os.Stat("flake.nix"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("flake.nix stat error with legacy keys = %v, want it absent", err)
	}

	rendered := 0
	err := fs.WalkDir(os.DirFS("."), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rendered++
		lower := strings.ToLower(terraformVorpalReadFile(t, path))
		for _, legacy := range []string{
			"setupnix",
			"legacy-binary-cache",
			"legacy-nixpkgs-branch",
			"legacy-system",
			"legacy-provider",
		} {
			if strings.Contains(lower, legacy) {
				t.Errorf("rendered %s contains legacy value %q", path, legacy)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking rendered files: %v", err)
	}
	if rendered == 0 {
		t.Fatal("render wrote no files")
	}
}

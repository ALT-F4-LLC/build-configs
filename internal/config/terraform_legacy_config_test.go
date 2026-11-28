package config

import (
	"errors"
	"fmt"
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
		{reflect.TypeFor[GitHubConfig](), "Action"},
	} {
		if _, ok := tc.typ.FieldByName(tc.field); ok {
			t.Errorf("%s still has a %s field", tc.typ.Name(), tc.field)
		}
	}
}

func TestTerraformRejectsLegacyNixKeys(t *testing.T) {
	cases := []struct {
		name       string
		key        string
		parameters map[string]interface{}
	}{
		{
			name: "nix",
			key:  "nix",
			parameters: map[string]interface{}{
				"nix": map[string]interface{}{"nixpkgsBranch": "legacy"},
			},
		},
		{
			name: "setupNix",
			key:  "setupNix",
			parameters: map[string]interface{}{
				"github": map[string]interface{}{
					"action": map[string]interface{}{"setupNix": true},
				},
			},
		},
		{
			name:       "installNix",
			key:        "installNix",
			parameters: map[string]interface{}{"installNix": true},
		},
		{
			name:       "cachix",
			key:        "cachix",
			parameters: map[string]interface{}{"cachix": map[string]interface{}{"binaryCache": "legacy"}},
		},
	}
	for _, template := range []string{TerraformName, TerraformModuleName} {
		for _, tc := range cases {
			t.Run(template+"/"+tc.name, func(t *testing.T) {
				cfg := Config{Name: "example", Template: template, Parameters: tc.parameters}
				if _, err := cfg.GetTemplater(); err == nil ||
					!strings.Contains(err.Error(), tc.key) ||
					!strings.Contains(err.Error(), "delete") ||
					!strings.Contains(err.Error(), "build-configs.yaml") {
					t.Errorf("GetTemplater error = %v, want an actionable error naming %q to delete", err, tc.key)
				}
			})
		}
	}
}

func TestTerraformRejectsRootLegacyNixKeys(t *testing.T) {
	keys := []string{"nix", "setupNix", "installNix", "cachix"}
	for _, template := range []string{TerraformName, TerraformModuleName} {
		for _, key := range keys {
			for _, format := range []string{"yaml", "json"} {
				t.Run(template+"/"+key+"/"+format, func(t *testing.T) {
					var (
						data   []byte
						loader func([]byte) (Config, error)
					)
					switch format {
					case "yaml":
						data = []byte(fmt.Sprintf("name: example\ntemplate: %s\n%s: true\n", template, key))
						loader = loadConfigYaml
					case "json":
						data = []byte(fmt.Sprintf(`{"name":"example","template":%q,%q:true}`, template, key))
						loader = loadConfigJSON
					}
					if _, err := loader(data); err == nil ||
						!strings.Contains(err.Error(), key) ||
						!strings.Contains(err.Error(), "delete") ||
						!strings.Contains(err.Error(), "build-configs.yaml") {
						t.Errorf("config loader error = %v, want an actionable error naming %q to delete", err, key)
					}
				})
			}
		}
	}
}

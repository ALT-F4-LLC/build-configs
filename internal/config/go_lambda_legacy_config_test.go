package config

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

const (
	goLambdaLegacyBinaryCache   = "legacy-lambda-binary-cache"
	goLambdaLegacyNixpkgsBranch = "legacy-lambda-nixpkgs-branch"
	goLambdaLegacyVendorHash    = "sha256-legacy-lambda-vendor-hash"
)

// goLambdaLegacyNixParameters returns a consumer config written for the Nix
// template, before the nix option was removed.
func goLambdaLegacyNixParameters(openAPI bool) map[string]interface{} {
	return map[string]interface{}{
		"nix": map[string]interface{}{
			"cachix":        map[string]interface{}{"binaryCache": goLambdaLegacyBinaryCache},
			"nixpkgsBranch": goLambdaLegacyNixpkgsBranch,
			"vendorHash":    goLambdaLegacyVendorHash,
			"buildGoModule": "buildGo122Module",
		},
		"openapi": map[string]interface{}{"enable": openAPI},
	}
}

// goLambdaRenderParameters renders go-lambda from a consumer config's
// parameters into a fresh temporary directory, which becomes the working
// directory.
func goLambdaRenderParameters(t *testing.T, parameters map[string]interface{}) {
	t.Helper()
	t.Chdir(t.TempDir())

	tpl, err := Config{Name: "example", Template: GoLambdaName, Parameters: parameters}.GetTemplater()
	if err != nil {
		t.Fatalf("GetTemplater: %v", err)
	}
	if err := tpl.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

// goLambdaRenderedFiles returns every regular file under the working
// directory with its content.
func goLambdaRenderedFiles(t *testing.T) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(path) //nolint:gosec // reads a file the test rendered into its own temp dir
		if err != nil {
			return err
		}
		files[filepath.ToSlash(path)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatalf("walk rendered files: %v", err)
	}
	return files
}

func TestGoLambdaConfigHasNoNixField(t *testing.T) {
	if _, ok := reflect.TypeFor[GoLambdaConfig]().FieldByName("Nix"); ok {
		t.Error("GoLambdaConfig has a Nix field")
	}
}

func TestGoLambdaRendersNoFlakeOrNixFiles(t *testing.T) {
	for _, tc := range []struct {
		name    string
		openAPI bool
	}{
		{name: "default", openAPI: false},
		{name: "openapi", openAPI: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			goLambdaRenderParameters(t, map[string]interface{}{
				"openapi": map[string]interface{}{"enable": tc.openAPI},
			})

			files := goLambdaRenderedFiles(t)
			if _, ok := files["justfile"]; !ok {
				t.Fatalf("rendered files %q lack justfile; the render did not run", slices.Sorted(maps.Keys(files)))
			}
			if _, ok := files["flake.nix"]; ok {
				t.Error("rendering go-lambda wrote flake.nix")
			}
			for path := range files {
				if strings.HasPrefix(path, "nix/") {
					t.Errorf("rendering go-lambda wrote %s", path)
				}
			}
		})
	}
}

func TestGoLambdaTemplateSetHasNoFlake(t *testing.T) {
	for _, name := range []string{"flake.nix", ".github__workflows__flake.yaml"} {
		if templates.GoLambdaTemplates.Lookup(name) != nil {
			t.Errorf("go-lambda template set still embeds %s", name)
		}
	}
}

func TestGoLambdaRendersConfigWithLegacyNixKey(t *testing.T) {
	for _, tc := range []struct {
		name    string
		openAPI bool
	}{
		{name: "default", openAPI: false},
		{name: "openapi", openAPI: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			goLambdaRenderParameters(t, goLambdaLegacyNixParameters(tc.openAPI))

			files := goLambdaRenderedFiles(t)
			if !strings.Contains(files["justfile"], "golangci-lint") {
				t.Fatalf("rendered justfile lacks golangci-lint; the scan cannot detect the legacy values:\n%s", files["justfile"])
			}
			for path, content := range files {
				for _, legacy := range []string{
					goLambdaLegacyBinaryCache,
					goLambdaLegacyNixpkgsBranch,
					goLambdaLegacyVendorHash,
					"buildGo122Module",
				} {
					if strings.Contains(content, legacy) {
						t.Errorf("%s contains the legacy nix value %q", path, legacy)
					}
				}
			}
		})
	}
}

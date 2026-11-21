package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const goCobraCliLegacyBinaryCache = "legacy-binary-cache-value"

// goCobraCliLegacyNixParameters is a consumer config written for the Nix
// template, before the nix option was removed.
var goCobraCliLegacyNixParameters = map[string]interface{}{
	"nix": map[string]interface{}{
		"cachix":        map[string]interface{}{"binaryCache": goCobraCliLegacyBinaryCache},
		"nixpkgsBranch": "nixos-24.05",
		"vendorHash":    "sha256-legacy",
	},
}

// goCobraCliFilesContaining returns every regular file under the working
// directory whose content contains needle.
func goCobraCliFilesContaining(t *testing.T, needle string) []string {
	t.Helper()
	var hits []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(path) //nolint:gosec // reads a file the test rendered into its own temp dir
		if err != nil {
			return err
		}
		if strings.Contains(string(b), needle) {
			hits = append(hits, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk rendered files: %v", err)
	}
	return hits
}

func TestGoCobraCliConfigHasNoNixField(t *testing.T) {
	if _, ok := reflect.TypeOf(GoCobraCliConfig{}).FieldByName("Nix"); ok {
		t.Error("GoCobraCliConfig has a Nix field")
	}
}

func TestGoCobraCliRendersConfigWithLegacyNixKey(t *testing.T) {
	goCobraCliRenderTemp(t, goCobraCliLegacyNixParameters)

	if _, err := os.Stat("flake.nix"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("flake.nix stat error with legacy nix key = %v, want it absent", err)
	}
	if hits := goCobraCliFilesContaining(t, "golangci-lint run ./..."); len(hits) == 0 {
		t.Fatal("scan found no rendered file with the check command; it cannot detect the legacy value")
	}
	if hits := goCobraCliFilesContaining(t, goCobraCliLegacyBinaryCache); len(hits) != 0 {
		t.Errorf("rendered files contain the legacy nix.cachix.binaryCache value: %q", hits)
	}
}

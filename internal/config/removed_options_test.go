package config

import (
	"reflect"
	"strings"
	"testing"
)

// The options below were read by the Nix flake or never read at all, so no
// template uses them. A consumer config that still sets one must keep
// rendering, because config decoding ignores unknown keys.
func TestRemovedOptionsAreNotDeclared(t *testing.T) {
	for _, tc := range []struct {
		config any
		field  string
	}{
		{GoCobraCliConfig{}, "CgoEnabled"},
		{GoCobraCliConfig{}, "GoVersion"},
		{GoCobraCliConfig{}, "Version"},
		{TerraformConfig{}, "Region"},
	} {
		typ := reflect.TypeOf(tc.config)
		if _, ok := typ.FieldByName(tc.field); ok {
			t.Errorf("%s still declares %s", typ.Name(), tc.field)
		}
	}
}

func TestRendersConfigWithRemovedOptions(t *testing.T) {
	t.Run("go-cobra-cli", func(t *testing.T) {
		goCobraCliRenderTemp(t, map[string]any{"cgoEnabled": true, "goVersion": "1.99", "version": "9.9.9"})
		if hits := goCobraCliFilesContaining(t, "9.9.9"); len(hits) != 0 {
			t.Errorf("rendered files contain the removed version value: %q", hits)
		}
	})
	t.Run("terraform", func(t *testing.T) {
		terraformVorpalRender(t, map[string]any{"region": "eu-west-1"})
		if src := terraformVorpalReadFile(t, terraformCIWorkflowPath); strings.Contains(src, "eu-west-1") {
			t.Errorf("workflow contains the removed region value:\n%s", src)
		}
	})
}

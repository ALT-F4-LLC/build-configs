package config

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
)

const terraformModuleShellPath = ".github/vorpal-shell.sh"

func TestTerraformModuleRendersExecutableVorpalShell(t *testing.T) {
	terraformModuleRenderTemp(t, "demo-module", nil)

	info, err := os.Stat(terraformModuleShellPath)
	if err != nil {
		t.Fatalf("%s was not written: %v", terraformModuleShellPath, err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Errorf("%s mode = %#o, want %#o", terraformModuleShellPath, got, 0o755)
	}

	got, err := os.ReadFile(terraformModuleShellPath)
	if err != nil {
		t.Fatalf("read %s: %v", terraformModuleShellPath, err)
	}
	for _, want := range []string{"vorpal build --path 'dev'", "bin/activate"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("%s does not contain %q:\n%s", terraformModuleShellPath, want, got)
		}
	}
}

func TestTerraformModuleVorpalShellHonorsBcignore(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(".bcignore", []byte(terraformModuleShellPath+"\n"), 0o600); err != nil {
		t.Fatalf("write .bcignore: %v", err)
	}

	tpl, err := Config{Name: "demo-module", Template: TerraformModuleName}.GetTemplater()
	if err != nil {
		t.Fatalf("GetTemplater: %v", err)
	}
	if err := tpl.Render(); err != nil {
		t.Fatalf("Render with %s ignored: %v", terraformModuleShellPath, err)
	}

	if _, err := os.Stat(terraformModuleShellPath); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("%s stat error = %v, want it absent", terraformModuleShellPath, err)
	}
}

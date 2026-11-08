package config

import (
	"os"
	"path/filepath"
	"testing"
)

// The go-cobra-cli snapshot pins every rendered file for the default config.
// It reuses the -update flag and the snapshot helpers from
// terraform_module_snapshot_test.go.
func TestGoCobraCliSnapshot(t *testing.T) {
	golden, err := filepath.Abs(filepath.Join("testdata", "go-cobra-cli-default.golden"))
	if err != nil {
		t.Fatalf("resolve golden path: %v", err)
	}

	root := t.TempDir()
	out := filepath.Join(root, "out")
	if err := os.Mkdir(out, 0o750); err != nil {
		t.Fatalf("create render dir: %v", err)
	}
	t.Chdir(out)

	tpl, err := Config{Name: "example", Template: GoCobraCliName}.GetTemplater()
	if err != nil {
		t.Fatalf("GetTemplater: %v", err)
	}
	if err := tpl.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	terraformModuleCompareGolden(t, golden, terraformModuleSnapshot(t, root))
}

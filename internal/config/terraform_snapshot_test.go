package config

import (
	"os"
	"path/filepath"
	"testing"
)

// The terraform snapshots pin every rendered file. They reuse the -update flag
// and the snapshot helpers from terraform_module_snapshot_test.go.
func TestTerraformSnapshot(t *testing.T) {
	schedule := "0 6 * * 1"

	for _, tc := range []struct {
		name   string
		golden string
		config func() TerraformConfig
	}{
		{
			name:   "default",
			golden: "terraform-default.golden",
			config: func() TerraformConfig {
				return NewTerraformConfig(Config{Name: "example", Template: TerraformName})
			},
		},
		{
			name:   "schedule script env deploy key",
			golden: "terraform-full.golden",
			config: func() TerraformConfig {
				cfg := NewTerraformConfig(Config{Name: "example", Template: TerraformName})
				cfg.Schedule = &schedule
				cfg.Region = "us-east-1"
				cfg.GitHub.Action.SetupDeployKey = true
				cfg.GitHub.Env = map[string]string{"TF_VAR_example": "value"}
				cfg.Vorpal.Script = "just plan"
				return cfg
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			golden, err := filepath.Abs(filepath.Join("testdata", tc.golden))
			if err != nil {
				t.Fatalf("resolve golden path: %v", err)
			}

			root := t.TempDir()
			out := filepath.Join(root, "out")
			if err := os.Mkdir(out, 0o750); err != nil {
				t.Fatalf("create render dir: %v", err)
			}
			t.Chdir(out)

			if err := tc.config().Render(); err != nil {
				t.Fatalf("Render: %v", err)
			}

			terraformModuleCompareGolden(t, golden, terraformModuleSnapshot(t, root))
		})
	}
}

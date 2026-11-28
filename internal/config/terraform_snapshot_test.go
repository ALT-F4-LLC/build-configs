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
			name:   "schedule env",
			golden: "terraform-full.golden",
			config: func() TerraformConfig {
				cfg := NewTerraformConfig(Config{Name: "example", Template: TerraformName})
				cfg.Schedule = &schedule
				cfg.GitHub.Env = map[string]string{"TF_VAR_example": "value"}
				return cfg
			},
		},
		{
			name:   "terraform-aws-gha-iam",
			golden: "terraform-aws-gha-iam.golden",
			config: func() TerraformConfig {
				cfg := NewTerraformConfig(Config{Name: "terraform-aws-gha-iam", Template: TerraformName})
				cfg.GitHub.AppToken = true
				cfg.GitHub.RunsOn = "altf4llc"
				return cfg
			},
		},
		{
			name:   "custom OIDC and secrets",
			golden: "terraform-custom-config.golden",
			config: func() TerraformConfig {
				cfg := NewTerraformConfig(Config{Name: "terraform-custom-config", Template: TerraformName})
				cfg.GitHub.RunsOn = "altf4llc"
				cfg.GitHub.Env = map[string]string{
					"TF_VAR_deployment_token": "${{ secrets.DEPLOYMENT_TOKEN }}", // #nosec G101 -- GitHub secret expression, not a literal credential
					"TF_VAR_region":           "us-east-1",
				}
				cfg.Role.PlanARN = "arn:aws:iam::111122223333:role/custom-plan"
				cfg.Role.ApplyARN = "arn:aws:iam::111122223333:role/custom-apply"
				cfg.Role.Region = "eu-west-1"
				cfg.Vorpal.Tools = []string{"terraform-docs:0.20.0"}
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

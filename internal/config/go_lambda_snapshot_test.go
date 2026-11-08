package config

import (
	"os"
	"path/filepath"
	"testing"
)

// The go-lambda snapshots pin every rendered file for configs that use
// neither services nor vorpal.extension. They reuse the -update flag and the
// snapshot helpers from terraform_module_snapshot_test.go.
func TestGoLambdaSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name   string
		golden string
		config func() GoLambdaConfig
	}{
		{
			name:   "default",
			golden: "go-lambda-default.golden",
			config: func() GoLambdaConfig {
				return NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
			},
		},
		{
			name:   "lambdas deploy privateModules",
			golden: "go-lambda-full.golden",
			config: func() GoLambdaConfig {
				cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
				cfg.Lambdas = []string{"api", "worker"}
				cfg.PrivateModules = "github.com/example/private-kit"
				cfg.Deploy = []DeployConfig{
					{Environment: "dev", Region: "us-west-2", RoleARN: "arn:aws:iam::111111111111:role/dev", If: goLambdaTestDeployIf},
					{Environment: "prod", Region: "us-east-1", RoleARN: "arn:aws:iam::111111111111:role/prod"},
				}
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

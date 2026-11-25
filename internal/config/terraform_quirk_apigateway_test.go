package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// terraformQuirkAPIGatewayYAML is the build-configs.yaml for
// terraform-quirk-apigateway on the terraform template. It keeps the platform
// directory token from the Nix-era file and drops the deploy key, setupNix,
// providers and runsOn: nixos. None of those exist on the template, and the
// jobs run on the default altf4llc runner.
const terraformQuirkAPIGatewayYAML = `---
name: terraform-quirk-apigateway
template: terraform
parameters:
  github:
    env:
      TF_VAR_PLATFORM_DIRECTORY_TOKEN: ${{ secrets.TF_VAR_PLATFORM_DIRECTORY_TOKEN }}
`

// The roles the repository's Nix-era workflow assumed. The template derives
// the same ARNs from the name, so the migrated workflow keeps its identity.
const (
	terraformQuirkAPIGatewayPlanARN  = "arn:aws:iam::677459762413:role/altf4llc-gha-terraform-quirk-apigateway-plan"
	terraformQuirkAPIGatewayApplyARN = "arn:aws:iam::677459762413:role/altf4llc-gha-terraform-quirk-apigateway-apply"
	terraformQuirkAPIGatewayEnvValue = "${{ secrets.TF_VAR_PLATFORM_DIRECTORY_TOKEN }}"
)

func terraformQuirkAPIGatewayConfig(t *testing.T) TerraformConfig {
	t.Helper()

	var cfg Config
	if err := yaml.Unmarshal([]byte(terraformQuirkAPIGatewayYAML), &cfg); err != nil {
		t.Fatalf("parse build-configs.yaml: %v", err)
	}

	tpl, err := cfg.GetTemplater()
	if err != nil {
		t.Fatalf("GetTemplater: %v", err)
	}
	tf, ok := tpl.(TerraformConfig)
	if !ok {
		t.Fatalf("GetTemplater returned %T, want TerraformConfig", tpl)
	}
	return tf
}

// Config decoding ignores unknown keys, so a render alone does not show that
// the template declares every key of the file. A strict decode of the
// parameters does.
func TestTerraformQuirkAPIGatewayDeclaresEveryKey(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte(terraformQuirkAPIGatewayYAML), &cfg); err != nil {
		t.Fatalf("parse build-configs.yaml: %v", err)
	}

	b, err := json.Marshal(cfg.Parameters)
	if err != nil {
		t.Fatalf("marshal parameters: %v", err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&TerraformConfig{}); err != nil {
		t.Errorf("TerraformConfig does not declare every parameter: %v", err)
	}
}

func TestTerraformQuirkAPIGatewayConfigValues(t *testing.T) {
	cfg := terraformQuirkAPIGatewayConfig(t)

	if got := cfg.GitHub.Env["TF_VAR_PLATFORM_DIRECTORY_TOKEN"]; got != terraformQuirkAPIGatewayEnvValue {
		t.Errorf("GitHub.Env token = %q, want %q", got, terraformQuirkAPIGatewayEnvValue)
	}
	if cfg.GitHub.RunsOn != "" {
		t.Errorf("GitHub.RunsOn = %q, want unset", cfg.GitHub.RunsOn)
	}
	if got := cfg.Runner(); got != "altf4llc" {
		t.Errorf("Runner() = %q, want altf4llc", got)
	}
	if cfg.Role.PlanARN != terraformQuirkAPIGatewayPlanARN {
		t.Errorf("Role.PlanARN = %q, want %q", cfg.Role.PlanARN, terraformQuirkAPIGatewayPlanARN)
	}
	if cfg.Role.ApplyARN != terraformQuirkAPIGatewayApplyARN {
		t.Errorf("Role.ApplyARN = %q, want %q", cfg.Role.ApplyARN, terraformQuirkAPIGatewayApplyARN)
	}
}

// The workflow keeps what the Nix-era one did: the token on the plan and
// apply jobs only, the same roles, no deploy key and no Nix runner.
func TestTerraformQuirkAPIGatewayWorkflow(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := terraformQuirkAPIGatewayConfig(t).Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	wf := terraformCIParseWorkflow(t)

	for name, job := range wf.Jobs {
		if job.RunsOn != "altf4llc" {
			t.Errorf("job %s runs-on = %q, want altf4llc", name, job.RunsOn)
		}
	}
	if src := terraformVorpalReadFile(t, terraformCIWorkflowPath); strings.Contains(src, "nix") {
		t.Errorf("rendered workflow mentions nix:\n%s", src)
	}

	if len(wf.Jobs["check"].Env) != 0 {
		t.Errorf("check job env = %v, want none", wf.Jobs["check"].Env)
	}

	for _, tc := range []struct {
		job string
		arn string
	}{
		{"plan", terraformQuirkAPIGatewayPlanARN},
		{"apply", terraformQuirkAPIGatewayApplyARN},
	} {
		job := wf.Jobs[tc.job]

		if got := job.Env["TF_VAR_PLATFORM_DIRECTORY_TOKEN"]; got != terraformQuirkAPIGatewayEnvValue {
			t.Errorf("job %s token env = %q, want %q", tc.job, got, terraformQuirkAPIGatewayEnvValue)
		}

		if key := terraformCIStepIndex(job, "shimataro/ssh-key-action"); key >= 0 {
			t.Errorf("job %s has a deploy key step at %d", tc.job, key)
		}
		aws := terraformCIStepIndex(job, "aws-actions/configure-aws-credentials")
		if aws < 0 {
			t.Errorf("job %s has no AWS credentials step", tc.job)
			continue
		}
		if got := job.Steps[aws].With["role-to-assume"]; got != tc.arn {
			t.Errorf("job %s role-to-assume = %q, want %q", tc.job, got, tc.arn)
		}
	}
}

// The snapshot pins every rendered file for the repository's configuration.
// It reuses the -update flag and the snapshot helpers from
// terraform_module_snapshot_test.go.
func TestTerraformQuirkAPIGatewaySnapshot(t *testing.T) {
	golden, err := filepath.Abs(filepath.Join("testdata", "terraform-quirk-apigateway.golden"))
	if err != nil {
		t.Fatalf("resolve golden path: %v", err)
	}

	root := t.TempDir()
	out := filepath.Join(root, "out")
	if err := os.Mkdir(out, 0o750); err != nil {
		t.Fatalf("create render dir: %v", err)
	}
	t.Chdir(out)

	if err := terraformQuirkAPIGatewayConfig(t).Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	terraformModuleCompareGolden(t, golden, terraformModuleSnapshot(t, root))
}

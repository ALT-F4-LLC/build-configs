package config

import (
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type goLambdaBuildJobStep struct {
	ID   string            `yaml:"id"`
	If   string            `yaml:"if"`
	Run  string            `yaml:"run"`
	Uses string            `yaml:"uses"`
	With map[string]string `yaml:"with"`
}

type goLambdaBuildJobWorkflow struct {
	Jobs map[string]struct {
		Strategy struct {
			Matrix map[string][]string `yaml:"matrix"`
		} `yaml:"strategy"`
		Steps []goLambdaBuildJobStep `yaml:"steps"`
	} `yaml:"jobs"`
}

// TestGoLambdaBuildJobPackagesEverythingInOneJob renders lambdas, services,
// artifacts and private modules, and pins the single build job's order:
// the token, then artifacts before services before lambdas (so no artifact
// source sees the lambda zips in dist/), then the token cleanup, then one
// upload per lambda that the deploy job downloads.
func TestGoLambdaBuildJobPackagesEverythingInOneJob(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Lambdas = []string{"api", "worker"}
	cfg.Services = []string{"mcp"}
	cfg.Artifacts = []string{"client-go"}
	cfg.PrivateModules = "github.com/example/kit"
	cfg.Deploy = []DeployConfig{{Environment: "prod", Region: "us-west-2", RoleARN: "arn:aws:iam::111111111111:role/prod"}}
	if err := cfg.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	var wf goLambdaBuildJobWorkflow
	if err := yaml.Unmarshal([]byte(goLambdaReadFile(t, goLambdaWorkflowPath)), &wf); err != nil {
		t.Fatalf("workflow does not parse: %v", err)
	}

	var builds []string
	for name, job := range wf.Jobs {
		if strings.HasPrefix(name, "build") {
			builds = append(builds, name)
			if len(job.Strategy.Matrix) != 0 {
				t.Errorf("job %s has a matrix %v, want none", name, job.Strategy.Matrix)
			}
		}
	}
	if !slices.Equal(builds, []string{"build"}) {
		t.Fatalf("build jobs = %q, want only build", builds)
	}

	var order []string
	for _, s := range wf.Jobs["build"].Steps {
		switch {
		case strings.HasPrefix(s.Uses, "extractions/netrc@"):
			order = append(order, "netrc")
		case strings.HasPrefix(s.Run, "just package"):
			order = append(order, s.Run)
		case s.Run == "rm -f ~/.netrc" && s.If == "always()":
			order = append(order, "cleanup")
		case strings.HasPrefix(s.Uses, "actions/upload-artifact@"):
			order = append(order, "upload "+s.With["name"]+" "+s.With["path"])
		}
	}
	want := []string{
		"netrc",
		`just package-artifact "client-go"`,
		`just package-artifact "mcp"`,
		`just package "api"`,
		`just package "worker"`,
		"cleanup",
		"upload zip-api dist/api/lambda.zip",
		"upload zip-worker dist/worker/lambda.zip",
	}
	if !slices.Equal(order, want) {
		t.Errorf("build job order =\n  %q\nwant\n  %q", order, want)
	}

	// The deploy job is one job per environment as well: it downloads every
	// zip the build job uploaded, then deploys each lambda.
	deploy := wf.Jobs["deploy-prod"]
	if len(deploy.Strategy.Matrix) != 0 {
		t.Errorf("deploy-prod has a matrix %v, want none", deploy.Strategy.Matrix)
	}
	var deploySteps []string
	for _, s := range deploy.Steps {
		switch {
		case strings.HasPrefix(s.Uses, "actions/download-artifact@"):
			deploySteps = append(deploySteps, "download "+s.With["name"]+" "+s.With["path"])
		case strings.HasPrefix(s.Run, "just deploy"):
			deploySteps = append(deploySteps, s.Run)
		}
	}
	wantDeploy := []string{
		"download zip-api dist/api",
		"download zip-worker dist/worker",
		`just deploy prod "api"`,
		`just deploy prod "worker"`,
	}
	if !slices.Equal(deploySteps, wantDeploy) {
		t.Errorf("deploy-prod steps =\n  %q\nwant\n  %q", deploySteps, wantDeploy)
	}
}

// No job in any rendered go-lambda workflow fans out over a matrix.
func TestGoLambdaWorkflowHasNoMatrix(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Lambdas = []string{"api", "worker"}
	cfg.Services = []string{"mcp"}
	cfg.Deploy = []DeployConfig{
		{Environment: "dev", Region: "us-west-2", RoleARN: "arn:aws:iam::111111111111:role/dev"},
		{Environment: "prod", Region: "us-west-2", RoleARN: "arn:aws:iam::111111111111:role/prod"},
	}
	if err := cfg.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	src := goLambdaReadFile(t, goLambdaWorkflowPath)
	var wf goLambdaBuildJobWorkflow
	if err := yaml.Unmarshal([]byte(src), &wf); err != nil {
		t.Fatalf("workflow does not parse: %v", err)
	}
	for name, job := range wf.Jobs {
		if len(job.Strategy.Matrix) != 0 {
			t.Errorf("job %s has a matrix %v, want none", name, job.Strategy.Matrix)
		}
	}
	if strings.Contains(src, "matrix.") {
		t.Errorf("workflow still references a matrix:\n%s", src)
	}
}

// Without lambdas there is no zip to deploy, so deploy is an error whether
// the project builds services, artifacts or nothing.
func TestGoLambdaDeployWithoutLambdasIsAnError(t *testing.T) {
	for name, mutate := range map[string]func(*GoLambdaConfig){
		"services":  func(c *GoLambdaConfig) { c.Services = []string{"mcp"} },
		"artifacts": func(c *GoLambdaConfig) { c.Artifacts = []string{"client-go"} },
		"nothing":   func(*GoLambdaConfig) {},
	} {
		t.Run(name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
			cfg.Lambdas = nil
			cfg.Deploy = []DeployConfig{{Environment: "prod", Region: "us-west-2", RoleARN: "arn:aws:iam::111111111111:role/prod"}}
			mutate(&cfg)
			err := cfg.Render()
			if err == nil || !strings.Contains(err.Error(), "lambdas is empty") {
				t.Errorf("Render error = %v, want the empty-lambdas deploy error", err)
			}
		})
	}
}

// A config with nothing to build renders no build or deploy job.
func TestGoLambdaNothingToBuildRendersNoBuildJob(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Lambdas = nil
	if err := cfg.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	var wf goLambdaBuildJobWorkflow
	if err := yaml.Unmarshal([]byte(goLambdaReadFile(t, goLambdaWorkflowPath)), &wf); err != nil {
		t.Fatalf("workflow does not parse: %v", err)
	}
	for name := range wf.Jobs {
		if strings.HasPrefix(name, "build") || strings.HasPrefix(name, "deploy") {
			t.Errorf("config with nothing to build renders job %s", name)
		}
	}
}

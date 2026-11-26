package config

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type goLambdaOptionsWorkflow struct {
	Jobs map[string]struct {
		Environment string `yaml:"environment"`
		Strategy    struct {
			Matrix map[string][]string `yaml:"matrix"`
		} `yaml:"strategy"`
		Steps []struct {
			Run  string            `yaml:"run"`
			Uses string            `yaml:"uses"`
			With map[string]string `yaml:"with"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

// goLambdaRenderOptions renders go-lambda into a fresh directory after mutate
// adjusts a config with one lambda, "api", and two deploy environments.
func goLambdaRenderOptions(t *testing.T, mutate func(*GoLambdaConfig)) (justfile string, wf goLambdaOptionsWorkflow) {
	t.Helper()
	t.Chdir(t.TempDir())

	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Lambdas = []string{"api"}
	cfg.Deploy = []DeployConfig{
		{Environment: "dev", Region: "us-west-2", RoleARN: "arn:aws:iam::111111111111:role/dev"},
		{Environment: "prod", Region: "us-west-2", RoleARN: "arn:aws:iam::111111111111:role/prod"},
	}
	mutate(&cfg)
	if err := cfg.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	src := goLambdaReadFile(t, goLambdaWorkflowPath)
	if err := yaml.Unmarshal([]byte(src), &wf); err != nil {
		t.Fatalf("workflow does not parse: %v\n%s", err, src)
	}
	return goLambdaReadFile(t, "justfile"), wf
}

func TestGoLambdaGenerate(t *testing.T) {
	t.Run("off", func(t *testing.T) {
		just, _ := goLambdaRenderOptions(t, func(*GoLambdaConfig) {})
		if strings.Contains(strings.TrimPrefix(just, hashNoticeLines), "generate") {
			t.Errorf("justfile mentions generate with it unset:\n%s", just)
		}
		if !strings.Contains(just, "\ncheck:\n    golangci-lint run ./...\n") {
			t.Errorf("check recipe changed:\n%s", just)
		}
	})

	t.Run("on", func(t *testing.T) {
		just, _ := goLambdaRenderOptions(t, func(c *GoLambdaConfig) {
			c.Generate = []string{
				`OUT=$(vorpal build --path "data")`,
				`install -m 0644 "$OUT"/*.json cmd/{{ "x" }}/`,
			}
		})
		want := "\ngenerate:\n" +
			"    #!/usr/bin/env bash\n" +
			"    set -euxo pipefail\n" +
			"    OUT=$(vorpal build --path \"data\")\n" +
			"    install -m 0644 \"$OUT\"/*.json cmd/{{ \"x\" }}/\n" +
			"\ncheck: generate\n    golangci-lint run ./...\n"
		if !strings.Contains(just, want) {
			t.Errorf("justfile lacks the generate recipe and check dependency:\n%s", just)
		}
		if strings.Index(just, "generate:") > strings.Index(just, "check: generate") {
			t.Errorf("generate recipe follows check:\n%s", just)
		}
	})

	t.Run("newline is an error", func(t *testing.T) {
		t.Chdir(t.TempDir())
		cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
		cfg.Generate = []string{"one\ntwo"}
		if err := cfg.Render(); err == nil || !strings.Contains(err.Error(), "generate") {
			t.Fatalf("Render error = %v, want one about generate", err)
		}
	})
}

func TestGoLambdaFunctionName(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		just, _ := goLambdaRenderOptions(t, func(*GoLambdaConfig) {})
		want := `--function-name "quirk-{{ environment }}-example-{{ lambda }}" \`
		if !strings.Contains(just, want) {
			t.Errorf("justfile lacks %q:\n%s", want, just)
		}
	})

	t.Run("custom passes just interpolation through", func(t *testing.T) {
		just, _ := goLambdaRenderOptions(t, func(c *GoLambdaConfig) { c.FunctionName = "altf4llc-{{ lambda }}" })
		want := `--function-name "altf4llc-{{ lambda }}" \`
		if !strings.Contains(just, want) {
			t.Errorf("justfile lacks %q:\n%s", want, just)
		}
		if strings.Contains(just, "quirk-") {
			t.Errorf("justfile still holds the default function name:\n%s", just)
		}
	})
}

// goLambdaBuildRuns returns the run commands of the single build job.
func goLambdaBuildRuns(t *testing.T, wf goLambdaOptionsWorkflow) []string {
	t.Helper()
	job, ok := wf.Jobs["build"]
	if !ok {
		t.Fatal("no build job")
	}
	if len(job.Strategy.Matrix) != 0 {
		t.Errorf("build job has a matrix %v, want one job for every entry", job.Strategy.Matrix)
	}
	var runs []string
	for _, step := range job.Steps {
		if step.Run != "" && !strings.HasPrefix(step.Run, "rm -f") {
			runs = append(runs, step.Run)
		}
	}
	return runs
}

func TestGoLambdaArtifacts(t *testing.T) {
	t.Run("off", func(t *testing.T) {
		_, wf := goLambdaRenderOptions(t, func(*GoLambdaConfig) {})
		if _, ok := wf.Jobs["build-services"]; ok {
			t.Error("build-services rendered")
		}
		if got, want := goLambdaBuildRuns(t, wf), []string{`just package "api"`}; !slices.Equal(got, want) {
			t.Errorf("build runs %q, want %q", got, want)
		}
	})

	t.Run("on without services", func(t *testing.T) {
		_, wf := goLambdaRenderOptions(t, func(c *GoLambdaConfig) { c.Artifacts = []string{"client-go", "docs"} })
		// Artifacts are Vorpal artifacts, not Go packages: no native build.
		want := []string{`just package-artifact "client-go"`, `just package-artifact "docs"`, `just package "api"`}
		if got := goLambdaBuildRuns(t, wf); !slices.Equal(got, want) {
			t.Errorf("build runs %q, want %q", got, want)
		}
	})

	t.Run("on with services", func(t *testing.T) {
		_, wf := goLambdaRenderOptions(t, func(c *GoLambdaConfig) {
			c.Services = []string{"svc"}
			c.Artifacts = []string{"client-go"}
		})
		want := []string{`just package-artifact "client-go"`, `just package-artifact "svc"`, `just package "api"`}
		if got := goLambdaBuildRuns(t, wf); !slices.Equal(got, want) {
			t.Errorf("build runs %q, want %q", got, want)
		}
	})

	t.Run("on with no lambdas packages only the artifacts", func(t *testing.T) {
		_, wf := goLambdaRenderOptions(t, func(c *GoLambdaConfig) {
			c.Lambdas = nil
			c.Deploy = nil
			c.Artifacts = []string{"client-go"}
		})
		if got, want := goLambdaBuildRuns(t, wf), []string{`just package-artifact "client-go"`}; !slices.Equal(got, want) {
			t.Errorf("build runs %q, want %q", got, want)
		}
		for _, step := range wf.Jobs["build"].Steps {
			if strings.HasPrefix(step.Uses, "actions/upload-artifact") {
				t.Errorf("artifacts-only build uploads %+v", step)
			}
		}
	})
}

func TestGoLambdaGithubEnvironment(t *testing.T) {
	t.Run("off", func(t *testing.T) {
		_, wf := goLambdaRenderOptions(t, func(*GoLambdaConfig) {})
		for name, job := range wf.Jobs {
			if job.Environment != "" {
				t.Errorf("job %s has environment %q with none set", name, job.Environment)
			}
		}
	})

	t.Run("on for one environment", func(t *testing.T) {
		_, wf := goLambdaRenderOptions(t, func(c *GoLambdaConfig) { c.Deploy[1].GithubEnvironment = "prod" })
		if got := wf.Jobs["deploy-prod"].Environment; got != "prod" {
			t.Errorf("deploy-prod environment = %q, want prod", got)
		}
		for _, name := range []string{"check", "build", "deploy-dev"} {
			if got := wf.Jobs[name].Environment; got != "" {
				t.Errorf("job %s environment = %q, want none", name, got)
			}
		}
	})
}

func TestGoLambdaDeployDownloadPath(t *testing.T) {
	_, wf := goLambdaRenderOptions(t, func(*GoLambdaConfig) {})
	for _, name := range []string{"deploy-dev", "deploy-prod"} {
		var paths []string
		for _, step := range wf.Jobs[name].Steps {
			if strings.HasPrefix(step.Uses, "actions/download-artifact") {
				paths = append(paths, step.With["path"])
			}
		}
		if want := []string{"dist/api"}; !slices.Equal(paths, want) {
			t.Errorf("%s download paths = %q, want %q", name, paths, want)
		}
	}
}

func TestGoLambdaOptionsRoundTrip(t *testing.T) {
	const (
		yamlParams = `name: example
template: go-lambda
parameters:
  generate:
    - just gen
  functionName: "altf4llc-{{ lambda }}"
  artifacts: [client-go]
  deploy:
    - environment: prod
      githubEnvironment: prod
`
		jsonParams = `{"name":"example","template":"go-lambda","parameters":{"generate":["just gen"],"functionName":"altf4llc-{{ lambda }}","artifacts":["client-go"],"deploy":[{"environment":"prod","githubEnvironment":"prod"}]}}`
	)

	for _, tc := range []struct {
		name  string
		parse func(t *testing.T) Config
	}{
		{"yaml", func(t *testing.T) Config {
			var c Config
			if err := yaml.Unmarshal([]byte(yamlParams), &c); err != nil {
				t.Fatal(err)
			}
			return c
		}},
		{"json", func(t *testing.T) Config {
			var c Config
			if err := json.Unmarshal([]byte(jsonParams), &c); err != nil {
				t.Fatal(err)
			}
			return c
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tpl, err := tc.parse(t).GetTemplater()
			if err != nil {
				t.Fatalf("GetTemplater: %v", err)
			}
			got, ok := tpl.(GoLambdaConfig)
			if !ok {
				t.Fatalf("templater is %T, want GoLambdaConfig", tpl)
			}
			if !slices.Equal(got.Generate, []string{"just gen"}) {
				t.Errorf("Generate = %q", got.Generate)
			}
			if got.FunctionName != "altf4llc-{{ lambda }}" {
				t.Errorf("FunctionName = %q", got.FunctionName)
			}
			if !slices.Equal(got.Artifacts, []string{"client-go"}) {
				t.Errorf("Artifacts = %q", got.Artifacts)
			}
			if len(got.Deploy) != 1 || got.Deploy[0].GithubEnvironment != "prod" {
				t.Errorf("Deploy = %+v, want one environment with githubEnvironment prod", got.Deploy)
			}
		})
	}
}

func TestGoLambdaOptionsDefaultEmpty(t *testing.T) {
	cfg := NewGoLambdaConfig(Config{Name: "example"})
	if len(cfg.Generate) != 0 || cfg.FunctionName != "" || len(cfg.Artifacts) != 0 {
		t.Errorf("defaults: generate = %q, functionName = %q, artifacts = %q, want all empty", cfg.Generate, cfg.FunctionName, cfg.Artifacts)
	}
}

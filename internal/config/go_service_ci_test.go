package config

import (
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const (
	goServiceTestPushCondition = "github.event_name == 'push' && github.ref == 'refs/heads/main'"
	goServiceTestPushIf        = "${{ " + goServiceTestPushCondition + " }}"
	goServiceRegistryLogin     = "docker/login-action@"
	goServiceAWSAction         = "aws-actions/configure-aws-credentials@"
)

type goServiceWorkflow struct {
	Env  map[string]string               `yaml:"env"`
	On   map[string]any                  `yaml:"on"`
	Jobs map[string]goServiceWorkflowJob `yaml:"jobs"`
}

type goServiceWorkflowJob struct {
	Concurrency map[string]any          `yaml:"concurrency"`
	Env         map[string]string       `yaml:"env"`
	Environment string                  `yaml:"environment"`
	If          string                  `yaml:"if"`
	Needs       []string                `yaml:"needs"`
	Permissions map[string]string       `yaml:"permissions"`
	RunsOn      string                  `yaml:"runs-on"`
	Steps       []goServiceWorkflowStep `yaml:"steps"`
}

type goServiceWorkflowStep struct {
	Env   map[string]string `yaml:"env"`
	ID    string            `yaml:"id"`
	If    string            `yaml:"if"`
	Run   string            `yaml:"run"`
	Shell string            `yaml:"shell"`
	Uses  string            `yaml:"uses"`
	With  map[string]string `yaml:"with"`
}

// goServiceCIConfigs are the configs every CI rule below must hold for.
func goServiceCIConfigs() map[string]func() GoServiceConfig {
	return map[string]func() GoServiceConfig{
		"default": func() GoServiceConfig { return NewGoServiceConfig(Config{Name: "demo", Template: GoServiceName}) },
		"bot":     goServiceBotConfig,
		"no private modules with image": func() GoServiceConfig {
			cfg := goServiceBotConfig()
			cfg.PrivateModules = ""
			return cfg
		},
		"services artifacts generate": func() GoServiceConfig {
			cfg := goServiceBotConfig()
			cfg.Services = []string{"api", "worker"}
			cfg.Artifacts = []string{"extra"}
			cfg.Generate = []string{"just gen"}
			return cfg
		},
	}
}

func goServiceParseWorkflow(t *testing.T) goServiceWorkflow {
	t.Helper()
	src := goLambdaReadFile(t, goLambdaWorkflowPath)
	var wf goServiceWorkflow
	if err := yaml.Unmarshal([]byte(src), &wf); err != nil {
		t.Fatalf("%s does not parse as YAML: %v\n%s", goLambdaWorkflowPath, err, src)
	}
	return wf
}

// Every job carries the fork guard, runs on the self-hosted runner, and runs
// its just recipes through the Vorpal shell. check and build read the
// repository only; with an image, the push job alone may request an OIDC
// token, and only for pushes to main.
func TestGoServiceCIJobsForkGuardAndPermissions(t *testing.T) {
	for name, cfg := range goServiceCIConfigs() {
		t.Run(name, func(t *testing.T) {
			c := cfg()
			goServiceRender(t, c)
			wf := goServiceParseWorkflow(t)

			wantJobs := []string{"build", "check"}
			if c.Image.Repository != "" {
				wantJobs = []string{"build", "check", "push"}
			}
			if got := slices.Sorted(maps.Keys(wf.Jobs)); !slices.Equal(got, wantJobs) {
				t.Fatalf("jobs = %q, want %q", got, wantJobs)
			}
			if !slices.Equal(wf.Jobs["build"].Needs, []string{"check"}) {
				t.Errorf("build needs = %q, want [check]", wf.Jobs["build"].Needs)
			}
			guard := "${{ " + goLambdaTestForkGuard + " }}"
			for job, j := range wf.Jobs {
				wantIf, wantPermissions := guard, map[string]string{"contents": "read"}
				if job == "push" {
					wantIf = "${{ (" + goLambdaTestForkGuard + ") && (" + goServiceTestPushCondition + ") }}"
					wantPermissions = map[string]string{"contents": "read", "id-token": "write"}
				}
				if j.If != wantIf {
					t.Errorf("job %s if = %q, want %q", job, j.If, wantIf)
				}
				if !maps.Equal(j.Permissions, wantPermissions) {
					t.Errorf("job %s permissions = %v, want %v", job, j.Permissions, wantPermissions)
				}
				if job != "push" && j.Environment != "" {
					t.Errorf("job %s environment = %q, want none", job, j.Environment)
				}
				if j.RunsOn != "altf4llc" {
					t.Errorf("job %s runs-on = %q, want altf4llc", job, j.RunsOn)
				}
				for _, step := range j.Steps {
					if strings.HasPrefix(step.Run, "just ") && step.Shell != "bash .github/vorpal-shell.sh {0}" {
						t.Errorf("job %s step %q shell = %q, want the Vorpal shell", job, step.Run, step.Shell)
					}
				}
			}
		})
	}
}

// goServiceSecretExpr matches a GitHub expression that reads a secret, a
// token, or another step's output.
var goServiceSecretExpr = regexp.MustCompile(`\$\{\{[^}]*\b(secrets\.|steps\.[^}]*outputs|github\.token)`)

// No run step carries a GitHub expression at all, so no secret, token or
// registry credential can land on a command line or in a shell script.
// Secrets reach only the `with` inputs of the App token and netrc actions;
// the registry login takes no static credential.
func TestGoServiceCIKeepsSecretsOutOfRunLines(t *testing.T) {
	for name, cfg := range goServiceCIConfigs() {
		t.Run(name, func(t *testing.T) {
			goServiceRender(t, cfg())
			wf := goServiceParseWorkflow(t)
			if src := goLambdaReadFile(t, goLambdaWorkflowPath); strings.Contains(src, "ALTF4LLC_DOCKER_REGISTRY") {
				t.Errorf("workflow still reads a static registry credential")
			}
			allowed := []string{"actions/create-github-app-token@", "extractions/netrc@"}
			if len(wf.Env) > 0 {
				t.Errorf("workflow env = %v; a workflow-level env reaches every run step", wf.Env)
			}
			for job, j := range wf.Jobs {
				if len(j.Env) > 0 {
					t.Errorf("job %s env = %v; a job-level env reaches every run step", job, j.Env)
				}
				for _, step := range j.Steps {
					if strings.Contains(step.Run, "${{") {
						t.Errorf("job %s run %q holds a GitHub expression; pass values through env", job, step.Run)
					}
					for key, value := range step.Env {
						if goServiceSecretExpr.MatchString(value) {
							t.Errorf("job %s step %q env %s = %q reads a secret", job, step.Run, key, value)
						}
					}
					usesSecret := false
					for _, value := range step.With {
						usesSecret = usesSecret || goServiceSecretExpr.MatchString(value)
					}
					if usesSecret && !slices.ContainsFunc(allowed, func(p string) bool { return strings.HasPrefix(step.Uses, p) }) {
						t.Errorf("job %s step %s passes a secret to an action outside %q", job, step.Uses, allowed)
					}
				}
			}
		})
	}
}

// The build job uploads the image tarball only for pushes to main, after the
// netrc cleanup, and touches neither AWS nor Docker. The push job runs only
// for pushes to main and needs build. It builds the Vorpal shell, then
// assumes the configured role through OIDC for 15 minutes, logs in with no
// static credential to the computed ECR registry from a per-job
// DOCKER_CONFIG, pushes, and always logs out. It holds no App token, netrc,
// generator or service build, and pushes per ref queue one at a time.
func TestGoServiceCIPushesImageFromMainOnly(t *testing.T) {
	for name, cfg := range goServiceCIConfigs() {
		c := cfg()
		if c.Image.Repository == "" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			goServiceRender(t, c)
			wf := goServiceParseWorkflow(t)
			goServiceCheckPushGating(t, wf, c)
		})
	}
}

func goServiceCheckPushGating(t *testing.T, wf goServiceWorkflow, c GoServiceConfig) {
	t.Helper()
	if got := slices.Sorted(maps.Keys(wf.On)); !slices.Equal(got, []string{"pull_request", "push"}) {
		t.Fatalf("on = %q, want pull_request and push", got)
	}
	if push, ok := wf.On["push"].(map[string]any); !ok || !slices.Equal(goServiceStrings(push["branches"]), []string{"main"}) {
		t.Errorf("on.push = %v, want branches [main]", wf.On["push"])
	}
	registry := c.Image.Account + ".dkr.ecr." + c.Image.Region + ".amazonaws.com"

	build := wf.Jobs["build"].Steps
	index := slices.IndexFunc[[]goServiceWorkflowStep]
	image := index(build, func(s goServiceWorkflowStep) bool { return s.Run == "just package-image" })
	netrcCleanup := index(build, func(s goServiceWorkflowStep) bool { return s.Run == ciNetrcCleanup && s.If == "always()" })
	upload := index(build, func(s goServiceWorkflowStep) bool { return strings.HasPrefix(s.Uses, "actions/upload-artifact@") })
	after := image
	if c.PrivateModules != "" {
		after = netrcCleanup
	}
	if image < 0 || after < image || upload < after || upload != len(build)-1 {
		t.Fatalf("build steps image=%d netrc-cleanup=%d upload=%d, want the upload last: %+v", image, netrcCleanup, upload, build)
	}
	if build[image].If != "" {
		t.Errorf("package-image if = %q; pull requests must still build the image", build[image].If)
	}
	if build[upload].If != goServiceTestPushIf {
		t.Errorf("upload if = %q, want %q", build[upload].If, goServiceTestPushIf)
	}
	if got := build[upload].With["path"]; got != "dist/"+c.ImageArtifact()+"/image.tar" {
		t.Errorf("upload path = %q", got)
	}
	if build[upload].With["retention-days"] != "1" {
		t.Errorf("upload retention-days = %q, want 1", build[upload].With["retention-days"])
	}
	if build[upload].With["if-no-files-found"] != "error" {
		t.Errorf("upload if-no-files-found = %q, want error", build[upload].With["if-no-files-found"])
	}
	for i, step := range build {
		if strings.Contains(step.Run, "docker") || strings.Contains(step.Run, "just push") ||
			strings.HasPrefix(step.Uses, "docker/") || strings.HasPrefix(step.Uses, "aws-actions/") {
			t.Errorf("build step %d %q%q touches AWS or the registry", i, step.Run, step.Uses)
		}
	}
	for _, job := range []string{"check", "build"} {
		if len(wf.Jobs[job].Concurrency) > 0 {
			t.Errorf("%s concurrency = %v, want none", job, wf.Jobs[job].Concurrency)
		}
		for _, step := range wf.Jobs[job].Steps {
			if strings.HasPrefix(step.Uses, "aws-actions/") || strings.HasPrefix(step.Uses, "docker/") {
				t.Errorf("%s step %q uses AWS or Docker", job, step.Uses)
			}
		}
	}

	push := wf.Jobs["push"]
	if !slices.Equal(push.Needs, []string{"build"}) {
		t.Errorf("push needs = %q, want [build]", push.Needs)
	}
	if push.Concurrency["group"] != "push-${{ github.ref }}" || push.Concurrency["cancel-in-progress"] != false || push.Concurrency["queue"] != "max" {
		t.Errorf("push concurrency = %v, want group push-${{ github.ref }}, no cancel-in-progress, queue max", push.Concurrency)
	}
	if push.Environment != c.Image.GithubEnvironment {
		t.Errorf("push environment = %q, want %q", push.Environment, c.Image.GithubEnvironment)
	}
	steps := push.Steps
	want := []string{"actions/checkout@", "actions/download-artifact@", "just --version", goServiceAWSAction, goServiceRegistryLogin, `just push "$REGISTRY" "$TAG"`, `docker logout "$REGISTRY"`}
	if len(steps) != len(want) {
		t.Fatalf("push steps = %+v, want %q", steps, want)
	}
	for i, w := range want {
		if !strings.HasPrefix(steps[i].Uses, w) && steps[i].Run != w {
			t.Errorf("push step %d = %s%s, want %s", i, steps[i].Uses, steps[i].Run, w)
		}
	}
	if steps[1].With["name"] != build[upload].With["name"] || steps[1].With["path"] != "dist/"+c.ImageArtifact() {
		t.Errorf("download with = %v, want the uploaded artifact in dist/%s", steps[1].With, c.ImageArtifact())
	}
	creds := steps[3].With
	wantCreds := map[string]string{"aws-region": c.Image.Region, "role-duration-seconds": "900", "role-to-assume": c.Image.RoleARN}
	if !maps.Equal(creds, wantCreds) {
		t.Errorf("configure-aws-credentials with = %v, want %v", creds, wantCreds)
	}
	if !maps.Equal(steps[4].With, map[string]string{"logout": "true", "registry": registry}) {
		t.Errorf("registry login with = %v, want logout and registry %s only", steps[4].With, registry)
	}
	for _, i := range []int{4, 5, 6} {
		if got := steps[i].Env["DOCKER_CONFIG"]; got != "${{ runner.temp }}/docker" {
			t.Errorf("push step %d DOCKER_CONFIG = %q, want ${{ runner.temp }}/docker", i, got)
		}
	}
	for _, i := range []int{5, 6} {
		if got := steps[i].Env["REGISTRY"]; got != registry {
			t.Errorf("push step %d REGISTRY = %q, want %s", i, got, registry)
		}
	}
	if got := steps[5].Env["TAG"]; got != "${{ github.sha }}" {
		t.Errorf("push env TAG = %q", got)
	}
	if steps[6].If != "always()" {
		t.Errorf("logout if = %q, want always()", steps[6].If)
	}
	for i, step := range steps {
		if i != 6 && step.If != "" {
			t.Errorf("push step %d if = %q; the job if gates the push", i, step.If)
		}
	}
}

// Without image.repository no job builds, logs in to, or pushes an image, and
// no job may request an OIDC token.
func TestGoServiceCIWithoutImageNeverTouchesRegistry(t *testing.T) {
	goServiceRender(t, NewGoServiceConfig(Config{Name: "demo", Template: GoServiceName}))
	src := goLambdaReadFile(t, goLambdaWorkflowPath)
	for _, absent := range []string{"docker", "REGISTRY", "package-image", "just push", "id-token", "aws-actions/", "dkr.ecr", "upload-artifact"} {
		if strings.Contains(src, absent) {
			t.Errorf("workflow without image.repository contains %q", absent)
		}
	}
}

// A GitHub environment applies to the push job alone.
func TestGoServiceCIPushEnvironment(t *testing.T) {
	cfg := goServiceBotConfig()
	cfg.Image.GithubEnvironment = "production"
	goServiceRender(t, cfg)
	wf := goServiceParseWorkflow(t)
	goServiceCheckPushGating(t, wf, cfg)
}

// The check job runs the generators before it writes the App token; the build
// job runs the generators, then packages every artifact, then every service,
// then the image, while the token is on disk.
func TestGoServiceCIOrdersGenerateAndPackages(t *testing.T) {
	cfg := goServiceBotConfig()
	cfg.Services = []string{"api", "worker"}
	cfg.Artifacts = []string{"extra"}
	cfg.Generate = []string{"just gen"}
	goServiceRender(t, cfg)
	wf := goServiceParseWorkflow(t)

	runs := func(job string) []string {
		var out []string
		for _, s := range wf.Jobs[job].Steps {
			if s.Run != "" {
				out = append(out, s.Run)
			}
		}
		return out
	}
	wantCheck := []string{"just secret-scan", "just doc-validate", "just generate", "just lint", "just vuln-scan", ciNetrcCleanup}
	if got := runs("check"); !slices.Equal(got, wantCheck) {
		t.Errorf("check runs %q, want %q", got, wantCheck)
	}
	token := slices.IndexFunc(wf.Jobs["check"].Steps, func(s goServiceWorkflowStep) bool {
		return strings.HasPrefix(s.Uses, "actions/create-github-app-token@")
	})
	generate := slices.IndexFunc(wf.Jobs["check"].Steps, func(s goServiceWorkflowStep) bool { return s.Run == "just generate" })
	if token < 0 || generate < 0 || generate > token {
		t.Errorf("check runs just generate at %d, after the App token at %d", generate, token)
	}

	wantBuild := []string{
		"just generate", `just package "extra"`, `just package "api"`, `just package "worker"`, "just package-image",
		ciNetrcCleanup,
	}
	if got := runs("build"); !slices.Equal(got, wantBuild) {
		t.Errorf("build runs %q, want %q", got, wantBuild)
	}
}

func goServiceStrings(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

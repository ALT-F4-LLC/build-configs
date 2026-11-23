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
	goServiceTestPushIf       = "${{ github.event_name == 'push' && github.ref == 'refs/heads/main' }}"
	goServiceTestPushIfAlways = "${{ always() && github.event_name == 'push' && github.ref == 'refs/heads/main' }}"
	goServiceRegistryLogin    = "docker/login-action@"
)

type goServiceWorkflow struct {
	Env  map[string]string               `yaml:"env"`
	On   map[string]any                  `yaml:"on"`
	Jobs map[string]goServiceWorkflowJob `yaml:"jobs"`
}

type goServiceWorkflowJob struct {
	Concurrency map[string]any          `yaml:"concurrency"`
	Env         map[string]string       `yaml:"env"`
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

// Every job carries the fork guard, reads the repository only, runs on the
// self-hosted runner, and runs its just recipes through the Vorpal shell.
func TestGoServiceCIJobsForkGuardAndPermissions(t *testing.T) {
	for name, cfg := range goServiceCIConfigs() {
		t.Run(name, func(t *testing.T) {
			goServiceRender(t, cfg())
			wf := goServiceParseWorkflow(t)

			if got := slices.Sorted(maps.Keys(wf.Jobs)); !slices.Equal(got, []string{"build", "check"}) {
				t.Fatalf("jobs = %q, want [build check]", got)
			}
			if !slices.Equal(wf.Jobs["build"].Needs, []string{"check"}) {
				t.Errorf("build needs = %q, want [check]", wf.Jobs["build"].Needs)
			}
			wantIf := "${{ " + goLambdaTestForkGuard + " }}"
			for job, j := range wf.Jobs {
				if j.If != wantIf {
					t.Errorf("job %s if = %q, want the fork guard %q", job, j.If, wantIf)
				}
				if !maps.Equal(j.Permissions, map[string]string{"contents": "read"}) {
					t.Errorf("job %s permissions = %v, want contents: read only", job, j.Permissions)
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
// registry credential, including the registry login's, can land on a command
// line or in a shell script. Secrets reach only the `with` inputs of the App
// token, netrc and registry login actions.
func TestGoServiceCIKeepsSecretsOutOfRunLines(t *testing.T) {
	for name, cfg := range goServiceCIConfigs() {
		t.Run(name, func(t *testing.T) {
			goServiceRender(t, cfg())
			wf := goServiceParseWorkflow(t)
			allowed := []string{"actions/create-github-app-token@", "extractions/netrc@", goServiceRegistryLogin}
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

// The registry login and push run only for pushes to main, never for a pull
// request; they follow the netrc cleanup, so the GitHub App token and the
// registry credential are never on the runner together; the credential lives
// in a per-job DOCKER_CONFIG and is removed on every outcome; and one build
// per ref runs at a time.
func TestGoServiceCIPushesImageFromMainOnly(t *testing.T) {
	for name, cfg := range goServiceCIConfigs() {
		c := cfg()
		if c.Image.Repository == "" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			goServiceRender(t, c)
			wf := goServiceParseWorkflow(t)
			goServiceCheckPushGating(t, wf, c.PrivateModules != "")
		})
	}
}

func goServiceCheckPushGating(t *testing.T, wf goServiceWorkflow, privateModules bool) {
	t.Helper()
	if got := slices.Sorted(maps.Keys(wf.On)); !slices.Equal(got, []string{"pull_request", "push"}) {
		t.Fatalf("on = %q, want pull_request and push", got)
	}
	if push, ok := wf.On["push"].(map[string]any); !ok || !slices.Equal(goServiceStrings(push["branches"]), []string{"main"}) {
		t.Errorf("on.push = %v, want branches [main]", wf.On["push"])
	}
	build := wf.Jobs["build"]
	if build.Concurrency["group"] != "build-${{ github.ref }}" || build.Concurrency["cancel-in-progress"] != false {
		t.Errorf("build concurrency = %v, want group build-${{ github.ref }} without cancel-in-progress", build.Concurrency)
	}

	steps := build.Steps
	index := func(match func(goServiceWorkflowStep) bool) int { return slices.IndexFunc(steps, match) }
	image := index(func(s goServiceWorkflowStep) bool { return s.Run == "just package-image" })
	netrcCleanup := index(func(s goServiceWorkflowStep) bool { return s.Run == ciNetrcCleanup && s.If == "always()" })
	login := index(func(s goServiceWorkflowStep) bool { return strings.HasPrefix(s.Uses, goServiceRegistryLogin) })
	push := index(func(s goServiceWorkflowStep) bool { return s.Run == `just push "$REGISTRY" "$TAG"` })
	logout := index(func(s goServiceWorkflowStep) bool { return s.Run == `docker logout "$REGISTRY"` })

	beforeLogin := image
	if privateModules {
		if netrcCleanup < image {
			t.Fatalf("build steps image=%d netrc-cleanup=%d, want the cleanup after the image: %+v", image, netrcCleanup, steps)
		}
		beforeLogin = netrcCleanup
	} else if netrcCleanup >= 0 {
		t.Errorf("build removes ~/.netrc without privateModules")
	}
	if image < 0 || login < beforeLogin || push < login || logout < push {
		t.Fatalf("build steps image=%d netrc-cleanup=%d login=%d push=%d logout=%d, want them in that order: %+v",
			image, netrcCleanup, login, push, logout, steps)
	}
	if steps[image].If != "" {
		t.Errorf("package-image if = %q; pull requests must still build the image", steps[image].If)
	}
	for _, i := range []int{login, push} {
		if steps[i].If != goServiceTestPushIf {
			t.Errorf("step %d (%s%s) if = %q, want %q", i, steps[i].Uses, steps[i].Run, steps[i].If, goServiceTestPushIf)
		}
	}
	if steps[logout].If != goServiceTestPushIfAlways {
		t.Errorf("logout if = %q, want %q", steps[logout].If, goServiceTestPushIfAlways)
	}
	for _, i := range []int{login, push, logout} {
		if got := steps[i].Env["DOCKER_CONFIG"]; got != "${{ runner.temp }}/docker" {
			t.Errorf("step %d (%s%s) DOCKER_CONFIG = %q, want ${{ runner.temp }}/docker", i, steps[i].Uses, steps[i].Run, got)
		}
	}
	if steps[login].With["logout"] != "true" {
		t.Errorf("registry login logout = %q, want true", steps[login].With["logout"])
	}
	if got := steps[login].With["password"]; got != "${{ secrets.ALTF4LLC_DOCKER_REGISTRY_PASSWORD }}" {
		t.Errorf("registry login password = %q", got)
	}
	if got := steps[push].Env["REGISTRY"]; got != "${{ vars.ALTF4LLC_DOCKER_REGISTRY_URL }}" {
		t.Errorf("push env REGISTRY = %q", got)
	}
	if got := steps[push].Env["TAG"]; got != "${{ github.sha }}" {
		t.Errorf("push env TAG = %q", got)
	}

	// No other step touches the registry.
	for i, step := range steps {
		if i == login || i == push || i == logout {
			continue
		}
		if strings.Contains(step.Run, "docker") || strings.Contains(step.Run, "just push") || strings.HasPrefix(step.Uses, "docker/") {
			t.Errorf("build step %d %q%q touches the registry outside the gated steps", i, step.Run, step.Uses)
		}
	}
	for _, step := range wf.Jobs["check"].Steps {
		if strings.Contains(step.Run, "docker") || strings.Contains(step.Run, "push") || strings.HasPrefix(step.Uses, "docker/") {
			t.Errorf("check step %q%q touches the registry", step.Run, step.Uses)
		}
	}
	if len(wf.Jobs["check"].Concurrency) > 0 {
		t.Errorf("check concurrency = %v, want none", wf.Jobs["check"].Concurrency)
	}
}

// Without image.repository no step builds, logs in to, or pushes an image.
func TestGoServiceCIWithoutImageNeverTouchesRegistry(t *testing.T) {
	goServiceRender(t, NewGoServiceConfig(Config{Name: "demo", Template: GoServiceName}))
	src := goLambdaReadFile(t, goLambdaWorkflowPath)
	for _, absent := range []string{"docker", "REGISTRY", "package-image", "just push"} {
		if strings.Contains(src, absent) {
			t.Errorf("workflow without image.repository contains %q", absent)
		}
	}
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
		ciNetrcCleanup, `just push "$REGISTRY" "$TAG"`, `docker logout "$REGISTRY"`,
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

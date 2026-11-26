package config

import (
	"go/format"
	"strings"
	"testing"
)

const goCobraCliSandboxMarker = "// privateModuleSandbox holds"

// TestGoCobraCliVorpalProgramWiresPrivateModuleSandbox checks that, with
// privateModules set, the CLI build gets the same sandbox settings and
// ~/.netrc credential as a go-lambda build, keyed to the artifact being
// built, and that without it the program carries none of them.
func TestGoCobraCliVorpalProgramWiresPrivateModuleSandbox(t *testing.T) {
	t.Run("off", func(t *testing.T) {
		goCobraCliRenderTemp(t, nil)
		src := goCobraCliReadFile(t, "cmd/vorpal/main.go")
		if _, err := format.Source([]byte(src)); err != nil {
			t.Fatalf("cmd/vorpal/main.go does not parse: %v\n%s", err, src)
		}
		for _, absent := range []string{"sandbox", "NETRC_", "GIT_SSL_CAINFO", ".netrc", "GOPRIVATE"} {
			if strings.Contains(src, absent) {
				t.Errorf("program without private modules mentions %q", absent)
			}
		}
	})

	t.Run("on", func(t *testing.T) {
		goCobraCliRenderTemp(t, goCobraCliPrivateModules)
		src := goCobraCliReadFile(t, "cmd/vorpal/main.go")
		if _, err := format.Source([]byte(src)); err != nil {
			t.Fatalf("cmd/vorpal/main.go does not parse: %v\n%s", err, src)
		}
		for _, want := range []string{
			`WithEnvironments([]string{"GOPRIVATE=github.com/ALT-F4-LLC/*"}).`,
			"sandbox := newPrivateModuleSandbox(ctx.GetTargetStr(), ctx.GetArtifactName())",
			`WithEnvironments(sandbox.environments("demo")).`,
			`WithSecrets(sandbox.secretsFor("demo")).`,
			`WithSourceScript(sandbox.scriptFor("demo")).`,
			`base:      []string{"GOPRIVATE=github.com/ALT-F4-LLC/*"},`,
		} {
			if !strings.Contains(src, want) {
				t.Errorf("program missing %q:\n%s", want, src)
			}
		}
		// Only the CLI builder takes secrets; the development environment
		// never does.
		if got := strings.Count(src, "WithSecrets("); got != 1 {
			t.Errorf("program calls WithSecrets %d times, want 1 (the CLI build only)", got)
		}
		for _, absent := range []string{"$NETRC_", "${NETRC_"} {
			if strings.Contains(src, absent) {
				t.Errorf("program references a secret by name with %q", absent)
			}
		}

		// The sandbox helpers are a copy of go-lambda's; keep them identical so
		// a fix to one reaches the other.
		cobra := goCobraCliSandboxHelpers(t, src)
		t.Chdir(t.TempDir())
		cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
		cfg.Lambdas = []string{"api"}
		cfg.PrivateModules = "github.com/ALT-F4-LLC/*"
		if err := cfg.Render(); err != nil {
			t.Fatalf("Render go-lambda: %v", err)
		}
		if lambda := goCobraCliSandboxHelpers(t, goLambdaReadFile(t, "cmd/vorpal/main.go")); cobra != lambda {
			t.Errorf("go-cobra-cli sandbox helpers differ from go-lambda's:\n--- go-cobra-cli\n%s\n--- go-lambda\n%s", cobra, lambda)
		}
	})
}

func goCobraCliSandboxHelpers(t *testing.T, src string) string {
	t.Helper()
	at := strings.Index(src, goCobraCliSandboxMarker)
	if at < 0 {
		t.Fatalf("program has no %q block:\n%s", goCobraCliSandboxMarker, src)
	}
	return strings.TrimSpace(src[at:])
}

package config

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const goLambdaOpenAPIGeneratorAlias = "openapi-generator-cli:7.18.0"

// goLambdaRenderOpenAPI renders go-lambda with OpenAPI client generation
// enabled or disabled into a fresh working directory.
func goLambdaRenderOpenAPI(t *testing.T, enable bool) {
	t.Helper()
	t.Chdir(t.TempDir())

	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Lambdas = []string{"api"}
	cfg.OpenAPI.Enable = enable
	cfg.OpenAPI.Filename = "spec.yaml"
	if err := cfg.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

func TestGoLambdaOpenAPIEnabledVorpalProgramFetchesGenerator(t *testing.T) {
	goLambdaRenderOpenAPI(t, true)

	if lits := goLambdaStringLits(t, goLambdaParseVorpalProgram(t)); !slices.Contains(lits, goLambdaOpenAPIGeneratorAlias) {
		t.Errorf("rendered Vorpal program has no tool alias %q", goLambdaOpenAPIGeneratorAlias)
	}
}

func TestGoLambdaOpenAPIEnabledDoesNotWriteNixClient(t *testing.T) {
	goLambdaRenderOpenAPI(t, true)

	if _, err := os.Stat("nix/client.nix"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("nix/client.nix was written (stat error %v)", err)
	}
}

func TestGoLambdaOpenAPIEnabledBuildClientOmitsNixCachixFlake(t *testing.T) {
	goLambdaRenderOpenAPI(t, true)

	header, body := goLambdaJustfileRecipe(t, goLambdaReadFile(t, "justfile"), "build-client")
	recipe := strings.ToLower(header + "\n" + strings.Join(body, "\n"))
	for _, banned := range []string{"nix", "cachix", "flake"} {
		if strings.Contains(recipe, banned) {
			t.Errorf("build-client recipe contains %q:\n%s", banned, recipe)
		}
	}
}

// TestGoLambdaOpenAPIEnabledBuildClientGeneratesIntoPkgClient runs the
// rendered build-client recipe against a stub generator that writes the
// metadata files openapi-generator emits beside the Go client.
func TestGoLambdaOpenAPIEnabledBuildClientGeneratesIntoPkgClient(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skipf("build-client recipe needs bash on PATH: %v", err)
	}
	goLambdaRenderOpenAPI(t, true)
	header, body := goLambdaJustfileRecipe(t, goLambdaReadFile(t, "justfile"), "build-client")
	if header != "build-client:" {
		t.Errorf("build-client header = %q, want %q", header, "build-client:")
	}
	if len(body) == 0 || body[0] != "#!/usr/bin/env bash" {
		t.Fatalf("build-client recipe is not a bash script: %q", body)
	}

	stubBin := t.TempDir()
	argsFile := filepath.Join(t.TempDir(), "args")
	stub := "#!/bin/sh\n" +
		"echo \"$*\" > '" + argsFile + "'\n" +
		"out=\n" +
		"while [ $# -gt 0 ]; do [ \"$1\" = -o ] && out=$2; shift; done\n" +
		"mkdir -p \"$out/.openapi-generator\" \"$out/api\" \"$out/docs\"\n" +
		"for f in client.go model_pet.go .gitignore .openapi-generator/FILES .openapi-generator-ignore .travis.yml README.md api/openapi.yaml docs/Pet.md git_push.sh; do\n" +
		"  echo generated > \"$out/$f\"\n" +
		"done\n"
	if err := os.WriteFile(filepath.Join(stubBin, "openapi-generator-cli"), []byte(stub), 0o755); err != nil { //nolint:gosec // the stub must be executable
		t.Fatal(err)
	}

	// A file from an earlier generation must not survive regeneration.
	if err := os.MkdirAll("pkg/client", 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("pkg/client/stale.go", []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", "-c", strings.Join(body, "\n")) //nolint:gosec // runs the recipe the test itself rendered
	cmd.Env = append(os.Environ(), "PATH="+stubBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build-client recipe failed: %v\n%s", err, out)
	}

	args := strings.TrimSpace(goLambdaReadFile(t, argsFile))
	if !strings.HasPrefix(args, "generate ") {
		t.Errorf("generator args = %q, want the generate command", args)
	}
	for _, want := range []string{"-i api/spec.yaml", "-g go", "-o pkg/client", "-p packageName=client"} {
		if !strings.Contains(args, want) {
			t.Errorf("generator args = %q, missing %q", args, want)
		}
	}

	entries, err := os.ReadDir("pkg/client")
	if err != nil {
		t.Fatalf("reading pkg/client: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if want := []string{"client.go", "model_pet.go"}; !slices.Equal(names, want) {
		t.Errorf("pkg/client = %q, want %q", names, want)
	}
}

func TestGoLambdaOpenAPIDisabledOmitsGeneratorAndBuildClient(t *testing.T) {
	goLambdaRenderOpenAPI(t, false)

	if lits := goLambdaStringLits(t, goLambdaParseVorpalProgram(t)); slices.Contains(lits, goLambdaOpenAPIGeneratorAlias) {
		t.Errorf("rendered Vorpal program fetches %q with openapi disabled", goLambdaOpenAPIGeneratorAlias)
	}

	for line := range strings.SplitSeq(goLambdaReadFile(t, "justfile"), "\n") {
		if strings.HasPrefix(line, "build-client") {
			t.Errorf("justfile has a build-client recipe with openapi disabled: %q", line)
		}
	}
}

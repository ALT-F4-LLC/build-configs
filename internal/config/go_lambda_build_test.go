package config

import (
	"archive/zip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// goLambdaRenderJustfile renders go-lambda with the api and worker lambdas
// into a fresh working directory and returns the rendered justfile.
func goLambdaRenderJustfile(t *testing.T, privateModules string) string {
	t.Helper()
	t.Chdir(t.TempDir())

	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Lambdas = []string{"api", "worker"}
	cfg.PrivateModules = privateModules
	if err := cfg.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return goLambdaReadFile(t, "justfile")
}

// goLambdaJustfileRecipe returns the header line and the indentation-stripped
// body lines of the recipe called name.
func goLambdaJustfileRecipe(t *testing.T, justfile, name string) (string, []string) {
	t.Helper()
	var header string
	var body []string
	for line := range strings.SplitSeq(justfile, "\n") {
		switch {
		case strings.TrimSpace(line) == "":
		case line[0] != ' ' && line[0] != '\t':
			if header != "" {
				return header, body
			}
			if fields := strings.Fields(strings.TrimSuffix(line, ":")); len(fields) > 0 && fields[0] == name {
				header = line
			}
		case header != "":
			body = append(body, strings.TrimSpace(line))
		}
	}
	if header == "" {
		t.Fatalf("justfile has no %s recipe:\n%s", name, justfile)
	}
	return header, body
}

func TestGoLambdaJustfileOmitsNixCachixFlake(t *testing.T) {
	for _, privateModules := range []string{"", "github.com/ALT-F4-LLC/quirk-service-kit"} {
		t.Run("privateModules="+privateModules, func(t *testing.T) {
			got := strings.ToLower(goLambdaRenderJustfile(t, privateModules))
			for _, banned := range []string{"nix", "cachix", "flake"} {
				if strings.Contains(got, banned) {
					t.Errorf("justfile contains %q:\n%s", banned, got)
				}
			}
		})
	}
}

func TestGoLambdaJustfileBuildRunsGoBuildForProfile(t *testing.T) {
	justfile := goLambdaRenderJustfile(t, "github.com/ALT-F4-LLC/quirk-service-kit")

	header, body := goLambdaJustfileRecipe(t, justfile, "build")
	if header != "build profile:" {
		t.Errorf("build header = %q, want %q", header, "build profile:")
	}
	want := []string{
		"#!/usr/bin/env bash",
		"set -euxo pipefail",
		`tags=""`,
		"case '{{ profile }}' in",
		`api|worker) tags="lambda.norpc" ;;`,
		"esac",
		"mkdir -p 'dist/{{ profile }}'",
		`CGO_ENABLED=0 go build \`,
		`-ldflags='-s -w' \`,
		`-tags="$tags" \`,
		`-o 'dist/{{ profile }}/{{ profile }}' \`,
		"'./cmd/{{ profile }}'",
	}
	if !slices.Equal(body, want) {
		t.Errorf("build body = %q, want %q", body, want)
	}
	if strings.Contains(strings.Join(body, "\n"), "vorpal") {
		t.Errorf("build runs vorpal: %q", body)
	}
}

// Services build without the lambda tag, as the Vorpal program builds them.
func TestGoLambdaJustfileBuildWithoutLambdasHasNoTagCase(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})
	cfg.Lambdas = nil
	cfg.Deploy = nil
	cfg.Services = []string{"svc"}
	if err := cfg.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	_, body := goLambdaJustfileRecipe(t, goLambdaReadFile(t, "justfile"), "build")
	if slices.Contains(body, "case '{{ profile }}' in") {
		t.Errorf("build with no lambdas has a tag case: %q", body)
	}
	_, pkg := goLambdaJustfileRecipe(t, goLambdaReadFile(t, "justfile"), "package-artifact")
	if want := []string{"vorpal build --path '{{ name }}'"}; !slices.Equal(pkg, want) {
		t.Errorf("package-artifact body = %q, want %q", pkg, want)
	}
}

func TestGoLambdaJustfileCheckRunsGolangciLint(t *testing.T) {
	justfile := goLambdaRenderJustfile(t, "")

	if _, body := goLambdaJustfileRecipe(t, justfile, "check"); !slices.Equal(body, []string{"golangci-lint run ./..."}) {
		t.Errorf("check body = %q, want [\"golangci-lint run ./...\"]", body)
	}
}

// TestGoLambdaJustfilePackageZipsBinaryAsBootstrap runs the rendered package
// recipe against a stub vorpal whose artifact holds the binary at
// bin/<lambda>, the layout the Vorpal Go builder writes.
func TestGoLambdaJustfilePackageZipsBinaryAsBootstrap(t *testing.T) {
	for _, tool := range []string{"bash", "zip"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("package recipe needs %s on PATH: %v", tool, err)
		}
	}
	justfile := goLambdaRenderJustfile(t, "github.com/ALT-F4-LLC/quirk-service-kit")
	header, body := goLambdaJustfileRecipe(t, justfile, "package")
	if header != "package profile:" {
		t.Errorf("package header = %q, want %q", header, "package profile:")
	}
	if len(body) == 0 || body[0] != "#!/usr/bin/env bash" {
		t.Fatalf("package recipe is not a bash script: %q", body)
	}

	stubRoot := t.TempDir()
	artifact := filepath.Join(stubRoot, "artifact")
	if err := os.MkdirAll(filepath.Join(artifact, "bin"), 0o750); err != nil {
		t.Fatal(err)
	}
	const binary = "fake api binary"
	if err := os.WriteFile(filepath.Join(artifact, "bin", "api"), []byte(binary), 0o555); err != nil { //nolint:gosec // stands in for a read-only store binary
		t.Fatal(err)
	}
	stubBin := filepath.Join(stubRoot, "bin")
	if err := os.Mkdir(stubBin, 0o750); err != nil {
		t.Fatal(err)
	}
	stub := "#!/bin/sh\n" +
		"[ \"$*\" = \"build --path api\" ] || { echo \"unexpected vorpal $*\" >&2; exit 1; }\n" +
		"echo '" + artifact + "'\n"
	if err := os.WriteFile(filepath.Join(stubBin, "vorpal"), []byte(stub), 0o755); err != nil { //nolint:gosec // the stub must be executable
		t.Fatal(err)
	}

	// A stale archive from an earlier package run must not leave entries
	// behind.
	goLambdaWriteZip(t, "dist/api/lambda.zip", "api")

	script := strings.ReplaceAll(strings.Join(body, "\n"), "{{ profile }}", "api")
	cmd := exec.Command("bash", "-c", script) //nolint:gosec // runs the recipe the test itself rendered
	cmd.Env = append(os.Environ(), "PATH="+stubBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("package recipe failed: %v\n%s", err, out)
	}

	info, err := os.Stat("dist/api/bootstrap")
	if err != nil {
		t.Fatalf("dist/api/bootstrap was not written: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("dist/api/bootstrap mode = %#o, want it executable", info.Mode().Perm())
	}
	if got := goLambdaReadFile(t, "dist/api/bootstrap"); got != binary {
		t.Errorf("dist/api/bootstrap = %q, want the built binary %q", got, binary)
	}

	archive, err := zip.OpenReader("dist/api/lambda.zip")
	if err != nil {
		t.Fatalf("opening dist/api/lambda.zip: %v", err)
	}
	defer func() { _ = archive.Close() }()
	var names []string
	for _, f := range archive.File {
		names = append(names, f.Name)
	}
	if !slices.Equal(names, []string{"bootstrap"}) {
		t.Fatalf("dist/api/lambda.zip entries = %q, want [\"bootstrap\"]", names)
	}
	if mode := archive.File[0].Mode().Perm(); mode&0o111 == 0 {
		t.Errorf("zipped bootstrap mode = %#o, want it executable", mode)
	}
	rc, err := archive.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rc.Close() }()
	content, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != binary {
		t.Errorf("zipped bootstrap = %q, want the built binary %q", content, binary)
	}
}

func goLambdaWriteZip(t *testing.T, path, entry string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path) //nolint:gosec // writes inside the test's own temp dir
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	if _, err := w.Create(entry); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestGoLambdaVorpalProgramDefaultToolsIncludeDeployTools(t *testing.T) {
	goLambdaRenderTemp(t, []string{"api"})
	lits := goLambdaStringLits(t, goLambdaParseVorpalProgram(t))

	for _, want := range []string{"jq:1.8.1", "awscli2:2.33.1"} {
		if !slices.Contains(lits, want) {
			t.Errorf("rendered Vorpal program has no tool alias %q", want)
		}
	}
}

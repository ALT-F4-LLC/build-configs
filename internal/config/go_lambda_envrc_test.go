package config

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

func TestGoLambdaRenderEnvrcActivatesVorpalDevEnvironment(t *testing.T) {
	goLambdaRenderTemp(t, []string{"api"})

	b, err := os.ReadFile(".envrc")
	if err != nil {
		t.Fatalf("reading .envrc: %v", err)
	}
	got := string(b)

	for _, want := range []string{
		"use_vorpal() {",
		`source "$(vorpal build --path 'example-dev')/bin/activate"`,
		"\nuse vorpal\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf(".envrc missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "flake") {
		t.Errorf(".envrc mentions flake:\n%s", got)
	}
}

// The render map writes every entry to the same output path space, so a
// second ".envrc" entry would overwrite the first in map iteration order.
func TestGoLambdaRenderMapListsEnvrcOnceFromGoLambdaTemplates(t *testing.T) {
	cfg := NewGoLambdaConfig(Config{Name: "example", Template: GoLambdaName})

	var owners []string
	for tmpl, files := range cfg.renderMap() {
		if slices.Contains(files, ".envrc") {
			owners = append(owners, tmpl.Name())
		}
	}
	if len(owners) != 1 {
		t.Fatalf(".envrc is listed by %d template sets %q, want 1", len(owners), owners)
	}
	if files := cfg.renderMap()[templates.GoLambdaTemplates]; !slices.Contains(files, ".envrc") {
		t.Errorf("go-lambda templates %q do not list .envrc", files)
	}
}

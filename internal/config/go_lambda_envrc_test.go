package config

import (
	"os"
	"strings"
	"testing"
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

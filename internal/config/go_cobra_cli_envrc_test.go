package config

import (
	"strings"
	"testing"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

func TestGoCobraCliRendersVorpalEnvrc(t *testing.T) {
	goCobraCliRenderTemp(t, nil)

	want := `use_vorpal() {
    source "$(vorpal build --path 'demo-dev')/bin/activate"
}

use vorpal
`
	got := goCobraCliReadFile(t, ".envrc")
	if got != want {
		t.Errorf(".envrc =\n%s\nwant\n%s", got, want)
	}
	if strings.Contains(got, "flake") {
		t.Errorf(".envrc mentions flake:\n%s", got)
	}
}

func TestGoCobraCliRendersEnvrcOnlyFromItsOwnTemplate(t *testing.T) {
	own, shared := 0, 0
	for set, files := range NewGoCobraCliConfig(Config{Name: "demo"}).renderMap() {
		for _, file := range files {
			if file != ".envrc" {
				continue
			}
			if set == templates.GoCobraCliTemplates {
				own++
			} else {
				shared++
			}
		}
	}
	if own != 1 || shared != 0 {
		t.Errorf(".envrc render entries: go-cobra-cli set %d, shared sets %d; want 1 and 0", own, shared)
	}
}

func TestGoCobraCliEnvrcQuotesNameForTheShell(t *testing.T) {
	t.Chdir(t.TempDir())

	tpl, err := Config{Name: `a'$(id)'b`, Template: GoCobraCliName}.GetTemplater()
	if err != nil {
		t.Fatalf("GetTemplater: %v", err)
	}
	if err := tpl.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	want := `vorpal build --path 'a'\''$(id)'\''b-dev'`
	if got := goCobraCliReadFile(t, ".envrc"); !strings.Contains(got, want) {
		t.Errorf(".envrc does not single-quote the dev environment name, want %s in:\n%s", want, got)
	}
}

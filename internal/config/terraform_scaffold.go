package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

// terraformScaffoldFiles are the Bun project files the Vorpal config program
// builds from. Renovate and bun rewrite them after the first run, so the
// generator creates each one only when it is absent and never overwrites it.
// None can carry a generated notice: package.json is strict JSON and bun.lock
// is rewritten by bun.
var terraformScaffoldFiles = []string{
	"package.json",
	"bun.lock",
	"tsconfig.json",
}

// addTerraformScaffold renders each missing file in terraformScaffoldFiles into
// files. A file that exists, even empty, is left alone.
func addTerraformScaffold(files map[string]string, data any) error {
	for _, name := range terraformScaffoldFiles {
		if _, err := os.Stat(name); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("checking %s: %w", name, err)
		}

		out, err := templates.RenderTemplate(templates.TerraformCommonTemplates, name, data)
		if err != nil {
			return err
		}
		files["./"+name] = out
	}

	return nil
}

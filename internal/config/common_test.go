package config

import (
	"slices"
	"testing"
	"text/template"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

// The render map is a Go map, so a file listed under two template sets is
// written by whichever set iterates last. Every output path must be listed
// once across a type's render map, and each file shared across types must
// come from the common set that owns it.
func TestRenderMapsListEveryFileOnceFromItsOwningSet(t *testing.T) {
	name := Config{Name: "demo"}

	vorpal := []string{".envrc", vorpalShell}
	terraformShared := []string{".gitignore", "Vorpal.toml"}

	for _, tc := range []struct {
		template string
		rm       templates.RenderMap
		owners   map[*template.Template][]string
	}{
		{
			template: GoCobraCliName,
			rm:       NewGoCobraCliConfig(name).renderMap(),
			owners:   map[*template.Template][]string{templates.VorpalCommonTemplates: vorpal},
		},
		{
			template: GoLambdaName,
			rm:       NewGoLambdaConfig(name).renderMap(),
			owners:   map[*template.Template][]string{templates.VorpalCommonTemplates: vorpal},
		},
		{
			template: GoLibraryName,
			rm:       NewGoLibraryConfig(name).renderMap(),
			owners:   map[*template.Template][]string{templates.VorpalCommonTemplates: vorpal},
		},
		{
			template: TerraformName,
			rm:       NewTerraformConfig(name).renderMap(),
			owners: map[*template.Template][]string{
				templates.VorpalCommonTemplates:    vorpal,
				templates.TerraformCommonTemplates: terraformShared,
			},
		},
		{
			template: TerraformModuleName,
			rm:       NewTerraformModuleConfig(name).renderMap(),
			owners: map[*template.Template][]string{
				templates.VorpalCommonTemplates:    vorpal,
				templates.TerraformCommonTemplates: terraformShared,
			},
		},
	} {
		t.Run(tc.template, func(t *testing.T) {
			seen := map[string]int{}
			for _, files := range tc.rm {
				for _, file := range files {
					seen[file]++
				}
			}
			for file, count := range seen {
				if count != 1 {
					t.Errorf("%s is listed %d times across template sets, want 1", file, count)
				}
			}

			for owner, files := range tc.owners {
				for _, file := range files {
					if !slices.Contains(tc.rm[owner], file) {
						t.Errorf("%s is not listed by its common set %q", file, owner.Name())
					}
				}
			}
		})
	}
}

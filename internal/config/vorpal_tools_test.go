package config

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestResolveTools(t *testing.T) {
	defaults := []string{"a:1", "b:1", "c:1"}
	for _, tc := range []struct {
		name    string
		added   []string
		exclude []string
		want    []string
	}{
		{name: "defaults only", want: []string{"a:1", "b:1", "c:1"}},
		{name: "added appends", added: []string{"d:1"}, want: []string{"a:1", "b:1", "c:1", "d:1"}},
		{name: "added same name only appends", added: []string{"b:2"}, want: []string{"a:1", "b:1", "c:1", "b:2"}},
		{name: "added exact default is not duplicated", added: []string{"a:1"}, want: []string{"a:1", "b:1", "c:1"}},
		{name: "exclude by name", exclude: []string{"b"}, want: []string{"a:1", "c:1"}},
		{name: "exclude by alias", exclude: []string{"c:1"}, want: []string{"a:1", "b:1"}},
		{name: "exclude other version keeps tool", exclude: []string{"c:9"}, want: []string{"a:1", "b:1", "c:1"}},
		{name: "exclude everything", exclude: []string{"a", "b", "c"}, want: []string{}},
		{name: "excluded then re-added", added: []string{"b:2"}, exclude: []string{"b:1"}, want: []string{"a:1", "c:1", "b:2"}},
		{name: "excluded by name then re-added", added: []string{"b:2"}, exclude: []string{"b"}, want: []string{"a:1", "c:1", "b:2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveTools(defaults, tc.added, tc.exclude); !slices.Equal(got, tc.want) {
				t.Errorf("resolveTools = %q, want %q", got, tc.want)
			}
		})
	}
}

// Every template keeps its defaults when parameters set vorpal.tools, and
// drops a default named in vorpal.excludeTools.
func TestTemplatesVorpalToolsAddAndExcludeToolsOptOut(t *testing.T) {
	type devTooler interface{ DevTools() []string }

	for _, template := range []string{GoCobraCliName, GoLambdaName, GoLibraryName, TerraformName, TerraformModuleName} {
		t.Run(template, func(t *testing.T) {
			load := func(vorpal map[string]any) []string {
				t.Helper()
				cfg := Config{Name: "demo", Template: template, Parameters: map[string]any{}}
				if vorpal != nil {
					// Round-trip through JSON so the map matches what YAML loading yields.
					b, err := json.Marshal(vorpal)
					if err != nil {
						t.Fatal(err)
					}
					var v map[string]any
					if err := json.Unmarshal(b, &v); err != nil {
						t.Fatal(err)
					}
					cfg.Parameters["vorpal"] = v
				}
				tpl, err := cfg.GetTemplater()
				if err != nil {
					t.Fatalf("GetTemplater: %v", err)
				}
				return tpl.(devTooler).DevTools()
			}

			defaults := load(nil)
			if len(defaults) == 0 {
				t.Fatal("template has no default tools")
			}

			added := load(map[string]any{"tools": []string{"extra:1.0.0"}})
			if want := append(slices.Clone(defaults), "extra:1.0.0"); !slices.Equal(added, want) {
				t.Errorf("with vorpal.tools = %q, want %q", added, want)
			}

			first := toolName(defaults[0])
			excluded := load(map[string]any{"excludeTools": []string{first}})
			if want := defaults[1:]; !slices.Equal(excluded, want) {
				t.Errorf("with vorpal.excludeTools [%s] = %q, want %q", first, excluded, want)
			}
		})
	}
}

package config

import (
	"slices"
	"testing"
)

// Every template's development environment carries jq and just at the pinned
// versions, whatever else it adds.
func TestEveryTemplateCarriesJqAndJust(t *testing.T) {
	cfg := Config{Name: "demo"}
	for name, tools := range map[string][]string{
		GoCobraCliName:         NewGoCobraCliConfig(cfg).DevTools(),
		GoLambdaName:           NewGoLambdaConfig(cfg).DevTools(),
		GoServiceName:          NewGoServiceConfig(cfg).DevTools(),
		GoLibraryName:          NewGoLibraryConfig(cfg).DevTools(),
		TerraformName:          NewTerraformConfig(cfg).DevTools(),
		TerraformModuleName:    NewTerraformModuleConfig(cfg).DevTools(),
		TypeScriptFrontendName: NewTypeScriptFrontendConfig(cfg).DevTools(),
	} {
		for _, want := range []string{"jq:1.8.1", "just:1.45.0"} {
			if !slices.Contains(tools, want) {
				t.Errorf("%s dev tools = %q, missing %s", name, tools, want)
			}
		}
	}
}

// go-lambda's development environment carries awscli2 and openjdk by default,
// with or without openapi.enable.
func TestGoLambdaCarriesAwscliAndOpenJDK(t *testing.T) {
	for _, enable := range []bool{false, true} {
		cfg := NewGoLambdaConfig(Config{Name: "demo"})
		cfg.OpenAPI.Enable = enable
		tools := cfg.DevTools()
		for _, want := range []string{"awscli2:2.33.1", "openjdk:25.0.1"} {
			if !slices.Contains(tools, want) {
				t.Errorf("openapi.enable=%v: go-lambda dev tools = %q, missing %s", enable, tools, want)
			}
		}
	}
}

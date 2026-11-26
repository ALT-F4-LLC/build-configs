package config

import (
	"slices"
	"strings"
)

// Default Vorpal artifact aliases each template puts in its development
// environment. They are always included unless vorpal.excludeTools names
// them; vorpal.tools only adds to them.
var (
	goDefaultTools = []string{
		"golangci-lint:2.15.0",
		"govulncheck:1.9.0",
		"jq:1.8.1",
		"just:1.45.0",
		"gitleaks:8.30.1",
		"rumdl:0.3.2",
	}

	goLambdaDefaultTools = []string{
		"awscli2:2.33.1",
		"gitleaks:8.30.1",
		"golangci-lint:2.15.0",
		"govulncheck:1.9.0",
		"jq:1.8.1",
		"just:1.45.0",
		"openjdk:25.0.1",
		"rumdl:0.3.2",
	}

	goLambdaOpenAPITool = "openapi-generator-cli:7.18.0"

	terraformDefaultTools = []string{
		"awscli2:2.33.1",
		"gitleaks:8.30.1",
		"jq:1.8.1",
		"just:1.45.0",
		"rumdl:0.3.2",
	}

	terraformModuleDefaultTools = []string{
		"gitleaks:8.30.1",
		"jq:1.8.1",
		"just:1.45.0",
		"rumdl:0.3.2",
	}
)

// toolName returns the artifact name of a Vorpal alias, the part before the
// first `:`.
func toolName(alias string) string {
	name, _, _ := strings.Cut(alias, ":")
	return name
}

// resolveTools returns defaults, minus every default whose name or full alias
// appears in exclude, followed by added. Added aliases only append: they never
// replace or remove a default, and exclude never drops one. To change a
// default's version, exclude it and add the new alias. An alias already in the
// list is not repeated.
func resolveTools(defaults, added, exclude []string) []string {
	out := make([]string, 0, len(defaults)+len(added))
	for _, alias := range defaults {
		if slices.Contains(exclude, alias) || slices.Contains(exclude, toolName(alias)) {
			continue
		}
		if !slices.Contains(out, alias) {
			out = append(out, alias)
		}
	}
	for _, alias := range added {
		if alias != "" && !slices.Contains(out, alias) {
			out = append(out, alias)
		}
	}
	return out
}

// hasTool reports whether tools carries an alias for the artifact name, at
// any version. Generated CI uses it to skip a check step whose tool was excluded.
func hasTool(tools []string, name string) bool {
	return slices.ContainsFunc(tools, func(alias string) bool { return toolName(alias) == name })
}

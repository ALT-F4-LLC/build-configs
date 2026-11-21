package config

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
)

type OpenAPIConfig struct {
	Enable   bool   `json:"enable" yaml:"enable"`
	Filename string `json:"filename" yaml:"filename"`

	// Output is the directory `just build-client` generates the Go client
	// into, relative to the repository root. The recipe deletes it first.
	Output string `json:"output,omitempty" yaml:"output,omitempty"`

	// PackageName is the Go package name of the generated client.
	PackageName string `json:"packageName,omitempty" yaml:"packageName,omitempty"`

	// GenerateInCI runs `just build-client` before `just check` and before the
	// CI build job, for repositories that do not commit the generated client.
	GenerateInCI bool `json:"generateInCI,omitempty" yaml:"generateInCI,omitempty"`

	// Module, when set, publishes the client as its own Go module with this
	// path: build-client writes <output>/go.mod, the client is committed, and
	// CI fails when it is stale (`just check-client`). Empty keeps today's
	// behavior.
	Module string `json:"module,omitempty" yaml:"module,omitempty"`
}

var (
	openAPIOutputPattern  = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_./-]*$`)
	openAPIPackagePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// validate rejects an output or package name that would send the build-client
// recipe's `rm -rf` outside the repository or break its shell words.
func (c OpenAPIConfig) validate() error {
	if !openAPIOutputPattern.MatchString(c.Output) || slices.Contains(strings.Split(c.Output, "/"), "..") {
		return fmt.Errorf("openapi.output %q must be a relative path of letters, digits, _, ., / and -, with no parent segment", c.Output)
	}
	if !openAPIPackagePattern.MatchString(c.PackageName) {
		return fmt.Errorf("openapi.packageName %q must be a Go identifier", c.PackageName)
	}
	if c.Module != "" {
		out := path.Clean(c.Output)
		if c.GenerateInCI {
			return fmt.Errorf("openapi.module needs a committed client; unset openapi.generateInCI")
		}
		if slices.Contains(strings.Split(out, "/"), "internal") || slices.Contains(strings.Split(c.Module, "/"), "internal") {
			return fmt.Errorf("openapi.module %q: an internal path cannot be imported by other repositories", c.Module)
		}
		if !strings.HasSuffix(c.Module, "/"+out) || strings.ContainsAny(c.Module, " \t'\"$`\\") {
			return fmt.Errorf("openapi.module %q must end with /%s", c.Module, out)
		}
	}
	return nil
}

func NewOpenAPIConfig() OpenAPIConfig {
	return OpenAPIConfig{
		Enable:      false,
		Filename:    "openapi.json",
		Output:      "pkg/client",
		PackageName: "client",
	}
}

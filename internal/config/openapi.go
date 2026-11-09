package config

import (
	"fmt"
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
	// CI build jobs, for repositories that do not commit the generated client.
	GenerateInCI bool `json:"generateInCI,omitempty" yaml:"generateInCI,omitempty"`
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

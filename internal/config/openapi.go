package config

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

func NewOpenAPIConfig() OpenAPIConfig {
	return OpenAPIConfig{
		Enable:      false,
		Filename:    "openapi.json",
		Output:      "pkg/client",
		PackageName: "client",
	}
}

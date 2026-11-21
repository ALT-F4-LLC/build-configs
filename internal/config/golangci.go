package config

type GolangCILintConfig struct {
	// Disable lists linters to turn off, by golangci-lint name.
	Disable      []string `json:"disable,omitempty" yaml:"disable,omitempty"`
	Exclude      []string `json:"exclude,omitempty" yaml:"exclude,omitempty"`
	ExtraExclude []string `json:"extraExclude,omitempty" yaml:"extraExclude,omitempty"`
}

func NewGolangCiLintConfig() GolangCILintConfig {
	return GolangCILintConfig{
		Exclude: []string{
			"Error return value of `\\(github.com/go-kit/log.Logger\\).Log` is not checked",
		},
		ExtraExclude: []string{},
	}
}

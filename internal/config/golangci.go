package config

type GolangCILintConfig struct {
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

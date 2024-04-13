package config

type GolangCILintConfig struct {
	Exclude []string `json:"exclude,omitEmpty" yaml:"exclude,omitEmpty"`
	ExtraExclude []string `json:"extraExclude,omitEmpty" yaml:"extraExclude,omitEmpty"`
}

func NewGolangCiLintConfig() GolangCILintConfig {
	return GolangCILintConfig{
		Exclude: []string{
			"Error return value of `\\(github.com/go-kit/log.Logger\\).Log` is not checked",
		},
		ExtraExclude: []string{},
	}
}

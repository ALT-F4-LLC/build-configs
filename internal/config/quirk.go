package config

type QuirkConfig struct {
	Service string `json:"service,omitempty" yaml:"service,omitempty"`
}

func NewQuirkConfig(c Config) QuirkConfig {
	return QuirkConfig{
		Service: c.Name,
	}
}

package config

type OpenAPIConfig struct {
	Enable bool `json:"enable" yaml:"enable"`
}

func NewOpenAPIConfig() OpenAPIConfig {
	return OpenAPIConfig{
		Enable: false,
	}
}

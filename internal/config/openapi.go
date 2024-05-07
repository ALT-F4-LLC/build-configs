package config

type OpenAPIConfig struct {
	Enable   bool   `json:"enable" yaml:"enable"`
	Filename string `json:"filename" yaml:"filename"`
}

func NewOpenAPIConfig() OpenAPIConfig {
	return OpenAPIConfig{
		Enable:   false,
		Filename: "openapi.json",
	}
}

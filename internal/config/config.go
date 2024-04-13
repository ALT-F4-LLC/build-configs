package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Name       string                 `json:"name" yaml:"name"`
	Template   string                 `json:"template" yaml:"template"`
	Parameters map[string]interface{} `json:"parameters" yaml:"parameters"`
}

func New(configPath string) (Config, error) {
	b, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, err
	}

	ext := path.Ext(configPath)
	switch ext {
	case "json":
		return loadConfigJson(b)
	case "yaml", "yml":
		return loadConfigYaml(b)
	}

	return Config{}, errors.New("unsupported file type; supported types are 'json', 'yaml' and 'yml'")
}

func (c Config) GetTemplateParams() (Templater, error) {
	switch c.Template {
	case "go-cobra-cli":
		var params GoCobraCliConfig
		b, err := json.Marshal(c.Parameters)
		if err != nil {
			return params, err
		}
		if err := json.Unmarshal(b, &params); err != nil {
			return params, err
		}
		return params, nil
	}
	return nil, fmt.Errorf("unsupported template: %v", c.Template)
}

func loadConfigJson(b []byte) (Config, error) {
	var cfg Config

	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func loadConfigYaml(b []byte) (Config, error) {
	var cfg Config

	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

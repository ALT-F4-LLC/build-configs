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

var (
	ErrUnsupportedFileType = errors.New("unsupported config file type")
	ErrUnsupportedTemplate = errors.New("unsupported template selected")
)

func New(configPath string) (Config, error) {
	b, err := os.ReadFile("./" + configPath)
	if err != nil {
		return Config{}, err
	}

	// Detect config file type and load it depending on extension
	ext := path.Ext(configPath)
	switch ext {
	case ".json":
		return loadConfigJson(b)
	case ".yaml", ".yml":
		return loadConfigYaml(b)
	}

	return Config{}, ErrUnsupportedFileType
}

func (c Config) GetTemplater() (Templater, error) {
	switch c.Template {
	case "go-cobra-cli":
		if Debug {
			fmt.Println("loading go-cobra-cli templater")
		}
		tpl := NewGoCobraCliConfig(c)

		// Convert the parameters (map type) to JSON
		b, err := json.Marshal(c.Parameters)
		if err != nil {
			return tpl, err
		}

		// Then convert them back into the type for the templater selected
		if err := json.Unmarshal(b, &tpl); err != nil {
			return tpl, err
		}
		return tpl, nil

	case "go-lambda":
		if Debug {
			fmt.Println("loading go-lambda templater")
		}
		tpl := NewGoLambdaConfig(c)

		// Convert the parameters (map type) to JSON
		b, err := json.Marshal(c.Parameters)
		if err != nil {
			return tpl, err
		}

		// Then convert them back into the type for the templater selected
		if err := json.Unmarshal(b, &tpl); err != nil {
			return tpl, err
		}
		return tpl, nil

	case "terraform":
		if Debug {
			fmt.Println("loading terraform templater")
		}
		tpl := NewTerraformConfig(c)

		// Convert the parameters (map type) to JSON
		b, err := json.Marshal(c.Parameters)
		if err != nil {
			return tpl, err
		}

		// Then convert them back into the type for the templater selected
		if err := json.Unmarshal(b, &tpl); err != nil {
			return tpl, err
		}
		return tpl, nil
	}

	return nil, ErrUnsupportedTemplate
}

func loadConfigJson(b []byte) (Config, error) {
	var cfg Config

	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}

	Cfg = cfg

	return cfg, nil
}

func loadConfigYaml(b []byte) (Config, error) {
	var cfg Config

	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}

	Cfg = cfg

	return cfg, nil
}

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Name       string                 `json:"name" yaml:"name"`
	Template   string                 `json:"template" yaml:"template"`
	Parameters map[string]interface{} `json:"parameters" yaml:"parameters"`
}

// DevEnvironment returns the Vorpal development environment name, <name>-dev.
func (c Config) DevEnvironment() string {
	return c.Name + "-dev"
}

// ShellQuotedDevEnvironment returns DevEnvironment as one POSIX single-quoted
// shell word, so generated scripts pass it to vorpal verbatim whatever
// characters the name holds.
func (c Config) ShellQuotedDevEnvironment() string {
	return "'" + strings.ReplaceAll(c.DevEnvironment(), "'", `'\''`) + "'"
}

var (
	ErrUnsupportedFileType = errors.New("unsupported config file type")
	ErrUnsupportedTemplate = errors.New("unsupported template selected")
)

func New(configPath string) (Config, error) {
	b, err := os.ReadFile("./" + configPath) //nolint:gosec // config path is chosen by the operator on the command line
	if err != nil {
		return Config{}, err
	}

	// Detect config file type and load it depending on extension
	ext := path.Ext(configPath)
	switch ext {
	case ".json":
		return loadConfigJSON(b)
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

	case "terraform-module":
		if Debug {
			fmt.Println("loading terraform module templater")
		}
		tpl := NewTerraformModuleConfig(c)

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

func loadConfigJSON(b []byte) (Config, error) {
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

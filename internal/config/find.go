package config

import "os"

var searchPaths = []string{
	"build-configs.json",
	"build-configs.yaml",
	"build-configs.yml",
}

func FindConfigPath() string {
	var configPath string

	for _, p := range searchPaths {
		if _, err := os.Stat("./" + p); err != nil {
			continue
		}
		configPath = p
	}

	return configPath
}

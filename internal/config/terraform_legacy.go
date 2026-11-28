package config

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

var terraformLegacyKeys = map[string]struct{}{
	"nix":        {},
	"setupnix":   {},
	"installnix": {},
	"cachix":     {},
}

func validateTerraformLegacyOptions(parameters map[string]interface{}) error {
	key, path, ok := findTerraformLegacyKey(parameters, "parameters")
	if !ok {
		return nil
	}
	return terraformLegacyOptionError(key, path)
}

func validateTerraformLegacyRootKeys(keys []string) error {
	slices.Sort(keys)
	for _, key := range keys {
		if _, ok := terraformLegacyKeys[strings.ToLower(key)]; ok {
			return terraformLegacyOptionError(key, key)
		}
	}
	return nil
}

func terraformLegacyOptionError(key, path string) error {
	return fmt.Errorf("legacy Terraform option %q is no longer supported; delete %s from build-configs.yaml", key, path)
}

func findTerraformLegacyKey(value interface{}, path string) (string, string, bool) {
	switch value := value.(type) {
	case map[string]interface{}:
		for _, key := range slices.Sorted(maps.Keys(value)) {
			keyPath := path + "." + key
			if _, ok := terraformLegacyKeys[strings.ToLower(key)]; ok {
				return key, keyPath, true
			}
			if legacyKey, legacyPath, ok := findTerraformLegacyKey(value[key], keyPath); ok {
				return legacyKey, legacyPath, true
			}
		}
	case []interface{}:
		for index, item := range value {
			if legacyKey, legacyPath, ok := findTerraformLegacyKey(item, fmt.Sprintf("%s[%d]", path, index)); ok {
				return legacyKey, legacyPath, true
			}
		}
	}
	return "", "", false
}

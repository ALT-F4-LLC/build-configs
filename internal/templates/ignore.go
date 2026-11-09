package templates

import (
	"os"
	"path"
	"strings"
)

func HasIgnoreFile(configDir string) bool {
	_, err := os.Stat(configDir + "/.bcignore")
	return err == nil
}

func GetIgnoredFiles(configDir string) (files []string, err error) {
	if !HasIgnoreFile(configDir) {
		return
	}

	b, err := os.ReadFile(configDir + "/.bcignore") //nolint:gosec // directory is chosen by the operator on the command line
	if err != nil {
		return
	}

	for _, line := range strings.Split(string(b), "\n") {
		entry := strings.TrimSpace(line)
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		files = append(files, path.Clean(entry))
	}

	return
}

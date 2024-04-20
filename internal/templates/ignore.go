package templates

import (
	"os"
	"strings"
)

func HasIgnoreFile(configDir string) bool {
	_, err := os.Stat(configDir+"/.bcignore")
	return err == nil
}

func GetIgnoredFiles(configDir string) (files []string, err error) {
	if !HasIgnoreFile(configDir) {
		return
	}

	b, err := os.ReadFile(configDir+"/.bcignore")
	if err != nil {
		return
	}

	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		files = append(files, line)
	}

	return
}

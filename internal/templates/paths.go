package templates

import (
	"os"
	"strings"
)

const (
	pathDivider = "/"
	templateDivider = "__"
)

func PathToTemplate(p string) string {
	return strings.ReplaceAll(p, pathDivider, templateDivider)
}

func TemplateToPath(p string) string {
	return strings.ReplaceAll(p, templateDivider, pathDivider)
}

func EnsureDirExists(p string) error {
	parts := strings.Split(p, pathDivider)
	dir := strings.Join(parts[:len(parts)-1], pathDivider)
	if dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}

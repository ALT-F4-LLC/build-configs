package templates

import "strings"

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

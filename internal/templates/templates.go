package templates

import "embed"

//go:embed templates
var content embed.FS

func RenderTemplate(name string, context interface{}) (string, error) {
	return "", nil
}

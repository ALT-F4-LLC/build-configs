package templates

import (
	"embed"
	"text/template"
)

var (
	//go:embed all:templates/go-cobra-cli/*
	content embed.FS

	GoCobraCliTemplates *template.Template
)

func init() {
	GoCobraCliTemplates = template.Must(template.ParseFS(content, "templates/go-cobra-cli/*"))
}

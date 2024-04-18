package templates

import (
	"embed"
	"os"
	"strings"
	"text/template"
)

var (
	//go:embed all:templates/common/all/*
	allCommonFS embed.FS

	//go:embed all:templates/common/go/*
	goCommonFS embed.FS

	//go:embed all:templates/go-cobra-cli/*
	goCobraCliFS embed.FS

	//go:embed all:templates/go-lambda/*
	goLambdaFS embed.FS

	AllCommonTemplates  *template.Template
	GoCommonTemplates   *template.Template
	GoCobraCliTemplates *template.Template
	GoLambdaTemplates *template.Template
)

// RenderMap maps a template set to the filenames* that should be written.
// * - filenames are the end result filenames, not the template names with
// substitution placeholders.
type RenderMap = map[*template.Template][]string

func init() {
	AllCommonTemplates = template.Must(template.ParseFS(allCommonFS, "templates/common/all/*"))
	GoCommonTemplates = template.Must(template.ParseFS(goCommonFS, "templates/common/go/*"))
	GoCobraCliTemplates = template.Must(template.ParseFS(goCobraCliFS, "templates/go-cobra-cli/*"))
	GoLambdaTemplates = template.Must(template.ParseFS(goLambdaFS, "templates/go-lambda/*"))
}

func RenderTemplates(in RenderMap, context any) (map[string]string, error) {
	files := map[string]string{}

	for tmpl, set := range in {
		for _, tmplPath := range set {
			out, err := RenderTemplate(tmpl, tmplPath, context)
			if err != nil {
				return files, err
			}

			files["./"+tmplPath] = out
		}
	}

	return files, nil
}

func RenderTemplate(t *template.Template, path string, context any) (string, error) {
	sb := strings.Builder{}
	if err := t.ExecuteTemplate(&sb, PathToTemplate(path), context); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func WriteFiles(in map[string]string) error {
	for k, v := range in {
		if err := EnsureDirExists(k); err != nil {
			return err
		}

		if err := os.WriteFile(k, []byte(v), 0644); err != nil {
			return err
		}
	}

	return nil
}

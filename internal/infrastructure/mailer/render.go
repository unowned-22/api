package mailer

import (
	"bytes"
	"embed"
	"fmt"
	"path/filepath"
	"text/template"
)

//go:embed templates/*
var templatesFS embed.FS

func RenderTemplate(name string, data any) (html, text string, err error) {
	htmlName := filepath.Join("templates", name+".html")
	textName := filepath.Join("templates", name+".txt")

	htmlTmpl, err := template.ParseFS(templatesFS, htmlName)
	if err != nil {
		return "", "", fmt.Errorf("parse html template: %w", err)
	}

	textTmpl, err := template.ParseFS(templatesFS, textName)
	if err != nil {
		return "", "", fmt.Errorf("parse text template: %w", err)
	}

	var htmlBuf bytes.Buffer
	if err := htmlTmpl.Execute(&htmlBuf, data); err != nil {
		return "", "", fmt.Errorf("execute html template: %w", err)
	}

	var textBuf bytes.Buffer
	if err := textTmpl.Execute(&textBuf, data); err != nil {
		return "", "", fmt.Errorf("execute text template: %w", err)
	}

	return htmlBuf.String(), textBuf.String(), nil
}

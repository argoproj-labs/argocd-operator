package util

import (
	"bytes"
	"fmt"
	"text/template"
)

// LoadTemplateFile will parse a template with the given path and execute it with the given params.
func LoadTemplateFile(path string, params map[string]string) (string, error) {
	tmpl, err := template.ParseFiles(path)
	if err != nil {
		return "", fmt.Errorf("LoadTemplateFile: unable to parse template: %w", err)
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, params)
	if err != nil {
		return "", fmt.Errorf("LoadTemplateFile: unable to execute template: %w", err)
	}
	return buf.String(), nil
}

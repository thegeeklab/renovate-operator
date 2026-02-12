package template

import (
	"embed"
	"html/template"
	"path"
	"strings"
)

//go:embed partial/*.html
var templateFS embed.FS

// Parse parses all templates from the embedded filesystem
// and names them based on the filename without the .html extension.
func Parse() (*template.Template, error) {
	tmpl := template.New("")

	entries, err := templateFS.ReadDir("partial")
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		fullPath := path.Join("partial", fileName)

		content, err := templateFS.ReadFile(fullPath)
		if err != nil {
			return nil, err
		}

		tplName := strings.TrimSuffix(fileName, ".html")

		_, err = tmpl.New(tplName).Parse(string(content))
		if err != nil {
			return nil, err
		}
	}

	return tmpl, nil
}

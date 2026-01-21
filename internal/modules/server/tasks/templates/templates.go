package templates

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"text/template"

	pkgtemplates "github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

//go:embed provision/*.sh
//go:embed software/*.sh
var templateFS embed.FS

// templates holds all parsed templates
var templates *template.Template

func init() {
	// Use shared CommonFuncMap from pkg/taskrunner/templates
	templates = template.New("").Funcs(pkgtemplates.CommonFuncMap)

	// Walk through embedded files and add each template with its full path
	err := fs.WalkDir(templateFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".sh" {
			return nil
		}

		content, err := templateFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		_, err = templates.New(path).Parse(string(content))
		if err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
		return nil
	})

	if err != nil {
		panic("failed to parse script templates: " + err.Error())
	}
}

// Render executes a named template with the given data
func Render(name string, data any) (string, error) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("template %s: %w", name, err)
	}
	return buf.String(), nil
}

// MustRender executes a template and panics on error
func MustRender(name string, data any) string {
	result, err := Render(name, data)
	if err != nil {
		panic(err)
	}
	return result
}

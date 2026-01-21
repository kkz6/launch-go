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

//go:embed *.sh
//go:embed deployment/*.sh
//go:embed deployment/prepare_fresh_installation/*.sh
var templateFS embed.FS

// templates holds all parsed templates
var templates *template.Template

func init() {
	// Create a copy of CommonFuncMap and override shellDefaults with lenient version
	// Site module uses set -eu (not pipefail) to match Laravel behavior
	funcMap := make(template.FuncMap)
	for k, v := range pkgtemplates.CommonFuncMap {
		funcMap[k] = v
	}
	funcMap["shellDefaults"] = pkgtemplates.ShellDefaultsLenient

	templates = template.New("").Funcs(funcMap)

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
		panic("failed to parse site script templates: " + err.Error())
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

package taskrunner

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates
var templateFS embed.FS

// TemplateEngine handles script template rendering
type TemplateEngine struct {
	templates *template.Template
	funcMap   template.FuncMap
}

// NewTemplateEngine creates a new template engine with common templates
func NewTemplateEngine() (*TemplateEngine, error) {
	funcMap := template.FuncMap{
		"join":     strings.Join,
		"contains": strings.Contains,
		"lower":    strings.ToLower,
		"upper":    strings.ToUpper,
		"trim":     strings.TrimSpace,
		"replace":  strings.ReplaceAll,
		"dirName":  filepath.Dir,
		"default": func(defaultVal, val interface{}) interface{} {
			if val == nil || val == "" {
				return defaultVal
			}
			return val
		},
		"quote": func(s string) string {
			return fmt.Sprintf("%q", s)
		},
		"escape": func(s string) string {
			return strings.ReplaceAll(s, "'", "'\"'\"'")
		},
	}

	tmpl := template.New("").Funcs(funcMap)

	// Load common templates
	err := fs.WalkDir(templateFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".sh") {
			return nil
		}

		content, err := templateFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", path, err)
		}

		// Create template name: templates/common/helpers.sh -> common/helpers
		name := strings.TrimPrefix(path, "templates/")
		name = strings.TrimSuffix(name, ".sh")

		_, err = tmpl.New(name).Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %w", path, err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	return &TemplateEngine{
		templates: tmpl,
		funcMap:   funcMap,
	}, nil
}

// RegisterModuleTemplates registers templates from a module's embedded filesystem
func (e *TemplateEngine) RegisterModuleTemplates(moduleFS embed.FS, prefix string) error {
	err := fs.WalkDir(moduleFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".sh") {
			return nil
		}

		content, err := moduleFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", path, err)
		}

		// Create template name with module prefix: templates/provision.sh -> server/provision
		name := strings.TrimPrefix(path, "templates/")
		name = strings.TrimSuffix(name, ".sh")
		name = prefix + "/" + name

		_, err = e.templates.New(name).Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %w", path, err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to register %s templates: %w", prefix, err)
	}

	return nil
}

// Render renders a template with the given data
func (e *TemplateEngine) Render(name string, data interface{}) (string, error) {
	var buf bytes.Buffer

	err := e.templates.ExecuteTemplate(&buf, name, data)
	if err != nil {
		return "", fmt.Errorf("failed to render template %s: %w", name, err)
	}

	return buf.String(), nil
}

// RenderString renders a template string (not from embedded files)
func (e *TemplateEngine) RenderString(templateStr string, data interface{}) (string, error) {
	tmpl, err := e.templates.Clone()
	if err != nil {
		return "", err
	}

	tmpl, err = tmpl.Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template string: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	return buf.String(), nil
}

// GetFuncMap returns the template function map for use by module engines
func (e *TemplateEngine) GetFuncMap() template.FuncMap {
	return e.funcMap
}

// Available template names
const (
	// Common templates
	TemplateHelpers         = "common/helpers"
	TemplateCallbackWrapper = "common/callback_wrapper"
	TemplateFunctions       = "common/functions"
)

// Data structures for templates

// CallbackWrapperData contains data for the callback wrapper
type CallbackWrapperData struct {
	Script      string
	Timeout     int
	FinishedURL string
	FailedURL   string
	TimeoutURL  string
}

package templates

import (
	"bytes"
	"fmt"
	"io/fs"
	"path/filepath"
	"sync"
	"text/template"
)

// RegisterOptions configures module template registration
type RegisterOptions struct {
	// UseLenientShellMode uses set -eu instead of set -euo pipefail
	UseLenientShellMode bool
	// CustomFuncs adds additional template functions for this module
	CustomFuncs template.FuncMap
}

type moduleTemplates struct {
	templates *template.Template
}

// Registry holds all module templates
type Registry struct {
	modules map[string]*moduleTemplates
	mu      sync.RWMutex
}

var globalRegistry = &Registry{
	modules: make(map[string]*moduleTemplates),
}

// Register adds a module's templates to the registry
func Register(module string, fsys fs.FS, opts *RegisterOptions) error {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	if _, exists := globalRegistry.modules[module]; exists {
		return fmt.Errorf("module %q already registered", module)
	}

	// Build function map starting with common functions
	funcMap := make(template.FuncMap)
	for k, v := range CommonFuncMap {
		funcMap[k] = v
	}

	// Apply options
	if opts != nil {
		if opts.UseLenientShellMode {
			funcMap["shellDefaults"] = ShellDefaultsLenient
		}
		for k, v := range opts.CustomFuncs {
			funcMap[k] = v
		}
	}

	tmpl := template.New("").Funcs(funcMap)

	// Walk and parse all .sh files
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".sh" {
			return nil
		}

		content, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		_, err = tmpl.New(path).Parse(string(content))
		if err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("loading templates for %s: %w", module, err)
	}

	globalRegistry.modules[module] = &moduleTemplates{templates: tmpl}
	return nil
}

// Has reports whether a module has a template registered under name.
// Callers use this to fail cleanly on a missing script instead of letting
// MustRender panic deep inside a worker job.
func Has(module, name string) bool {
	globalRegistry.mu.RLock()
	mod, exists := globalRegistry.modules[module]
	globalRegistry.mu.RUnlock()

	if !exists {
		return false
	}
	return mod.templates.Lookup(name) != nil
}

// Render renders a template from a registered module
func Render(module, name string, data any) (string, error) {
	globalRegistry.mu.RLock()
	mod, exists := globalRegistry.modules[module]
	globalRegistry.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("module %q not registered", module)
	}

	var buf bytes.Buffer
	if err := mod.templates.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("template %s/%s: %w", module, name, err)
	}
	return buf.String(), nil
}

// MustRender renders a template or panics on error
func MustRender(module, name string, data any) string {
	result, err := Render(module, name, data)
	if err != nil {
		panic(err)
	}
	return result
}

// Reset clears the registry (for testing)
func Reset() {
	globalRegistry.mu.Lock()
	globalRegistry.modules = make(map[string]*moduleTemplates)
	globalRegistry.mu.Unlock()
}

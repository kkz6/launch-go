# Template Registry Refactoring Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace duplicated module template engines with a single centralized registry.

**Architecture:** Single `Registry` in `pkg/taskrunner/templates/` that modules register with at app startup. Clean API: `templates.Render("server", "provision/apt.sh", data)`.

**Tech Stack:** Go text/template, embed.FS

---

## Task 1: Create Template Registry

**Files:**
- Create: `internal/pkg/taskrunner/templates/registry.go`

**Implementation:**
```go
package templates

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sync"
	"text/template"
)

// RegisterOptions configures module template registration
type RegisterOptions struct {
	UseLenientShellMode bool
	CustomFuncs         template.FuncMap
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
func Register(module string, fsys embed.FS, opts *RegisterOptions) error {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	if _, exists := globalRegistry.modules[module]; exists {
		return fmt.Errorf("module %q already registered", module)
	}

	funcMap := make(template.FuncMap)
	for k, v := range CommonFuncMap {
		funcMap[k] = v
	}

	if opts != nil {
		if opts.UseLenientShellMode {
			funcMap["shellDefaults"] = ShellDefaultsLenient
		}
		for k, v := range opts.CustomFuncs {
			funcMap[k] = v
		}
	}

	tmpl := template.New("").Funcs(funcMap)

	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".sh" {
			return nil
		}

		content, err := fsys.ReadFile(path)
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
```

---

## Task 2: Simplify Server Module Templates

**Files:**
- Rewrite: `internal/modules/server/tasks/templates/templates.go`

**New content:**
```go
package templates

import "embed"

//go:embed provision/*.sh
//go:embed software/*.sh
var FS embed.FS
```

---

## Task 3: Simplify Site Module Templates

**Files:**
- Rewrite: `internal/modules/site/tasks/templates/templates.go`

**New content:**
```go
package templates

import "embed"

//go:embed *.sh
//go:embed deployment/*.sh
//go:embed deployment/prepare_fresh_installation/*.sh
var FS embed.FS
```

---

## Task 4: Simplify Database Module Templates

**Files:**
- Rewrite: `internal/modules/database/tasks/templates/templates.go`

**New content:**
```go
package templates

import "embed"

//go:embed mysql/*.sh
//go:embed postgresql/*.sh
var FS embed.FS
```

---

## Task 5: Create Template Registration Function

**Files:**
- Create: `internal/pkg/taskrunner/templates/init.go`

**Implementation:**
```go
package templates

import (
	dbtemplates "github.com/kkz6/launch-go/internal/modules/database/tasks/templates"
	servertemplates "github.com/kkz6/launch-go/internal/modules/server/tasks/templates"
	sitetemplates "github.com/kkz6/launch-go/internal/modules/site/tasks/templates"
)

// RegisterAll registers all module templates
func RegisterAll() error {
	if err := Register("server", servertemplates.FS, nil); err != nil {
		return err
	}
	if err := Register("site", sitetemplates.FS, &RegisterOptions{UseLenientShellMode: true}); err != nil {
		return err
	}
	if err := Register("database", dbtemplates.FS, nil); err != nil {
		return err
	}
	return nil
}
```

---

## Task 6: Update All Template Callers

**Pattern:**
- Find: `templates.MustRender("xxx.sh", data)` or `templates.Render("xxx.sh", data)`
- Replace with: `templates.Render("module", "xxx.sh", data)` using centralized package

**Files to update:**
- `internal/modules/server/tasks/*.go`
- `internal/modules/site/tasks/*.go`
- `internal/modules/database/tasks/*.go`
- Any tests using templates

---

## Task 7: Cleanup Unused Code

**Files to delete:**
- `internal/pkg/taskrunner/template_engine.go`

**Code to remove from `internal/pkg/taskrunner/templates/functions.go`:**
- `Engine` struct and all its methods (LoadFromFS, AddFunc, Render, MustRender, RenderString)

---

## Task 8: Call RegisterAll at App Startup

**Files:**
- Modify: `cmd/api/main.go` or app initialization

Add call to `templates.RegisterAll()` early in startup.

---

## Task 9: Verify and Commit

- Run `go build ./...`
- Run `go test ./...`
- Commit: "Refactor templates to use centralized registry"

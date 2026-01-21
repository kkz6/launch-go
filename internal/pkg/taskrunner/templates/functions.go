// Package templates provides common bash template functions and utilities
// for script generation across all modules.
package templates

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"text/template"
)

// CommonFuncMap provides template functions available to all modules
var CommonFuncMap = template.FuncMap{
	"shellDefaults":   ShellDefaults,
	"aptFunctions":    AptFunctions,
	"commonFuncs":     CommonFunctions,
	"phpPpaFunctions": PhpPpaFunctions,
	"dirName":         filepath.Dir,
	"join":            strings.Join,
	"contains":        strings.Contains,
	"lower":           strings.ToLower,
	"upper":           strings.ToUpper,
	"trim":            strings.TrimSpace,
	"replace":         strings.ReplaceAll,
	"quote": func(s string) string {
		return fmt.Sprintf("%q", s)
	},
	"escape": func(s string) string {
		return strings.ReplaceAll(s, "'", "'\"'\"'")
	},
	"default": func(defaultVal, val interface{}) interface{} {
		if val == nil || val == "" {
			return defaultVal
		}
		return val
	},
}

// ShellDefaults returns the standard shell script header with strict error handling.
// Uses set -euo pipefail for:
//   - -e: Exit on first error
//   - -u: Exit on undefined variable
//   - -o pipefail: Exit if any command in a pipeline fails
func ShellDefaults() string {
	return `set -euo pipefail
export DEBIAN_FRONTEND=noninteractive`
}

// ShellDefaultsLenient returns a shell script header that's more lenient.
// Uses set -eu (no pipefail) for compatibility with Laravel's behavior.
func ShellDefaultsLenient() string {
	return `set -eu
export DEBIAN_FRONTEND=noninteractive`
}

// CommonFunctions returns common bash helper functions.
func CommonFunctions() string {
	return `# Send a POST request to the given URL, ignoring the response and errors
function httpPostSilently() {
    if [ -z "${2:-}" ]; then
        (curl -X POST --silent --max-time 15 --output /dev/null $1 || true)
    else
        (curl -X POST --silent --max-time 15 --output /dev/null $1 -H 'Content-Type: application/json' --data "$2" || true)
    fi
}

function httpPostRawSilently() {
    (curl -X POST --silent --max-time 15 --output /dev/null $1 --data "$2" || true)
}`
}

// AptFunctions returns apt-related bash functions for Ubuntu/Debian.
func AptFunctions() string {
	return `# Wait for apt to be unlocked
function waitForAptUnlock() {
    while ps -C apt,apt-get,dpkg >/dev/null 2>&1; do
        echo "apt, apt-get or dpkg is running..."
        sleep 5
    done

    while fuser /var/{lib/{dpkg,apt/lists},cache/apt/archives}/{lock,lock-frontend} >/dev/null 2>&1; do
        echo "Waiting: apt is locked..."
        sleep 5
    done

    if [ -f /var/log/unattended-upgrades/unattended-upgrades.log ]; then
        while fuser /var/log/unattended-upgrades/unattended-upgrades.log >/dev/null 2>&1; do
            echo "Waiting: unattended-upgrades is locked..."
            sleep 5
        done
    fi
}`
}

// PhpPpaFunctions returns the PHP PPA installation function for Ubuntu.
func PhpPpaFunctions() string {
	return `function ensurePhpPpaInstalled() {
    if ! grep -q "ondrej/php" /etc/apt/sources.list.d/*.list 2>/dev/null && ! grep -q "ondrej/php" /etc/apt/sources.list.d/*.sources 2>/dev/null; then
        echo "Adding ondrej/php PPA..."
        waitForAptUnlock
        sudo DEBIAN_FRONTEND=noninteractive apt-get install -y software-properties-common
        waitForAptUnlock
        sudo DEBIAN_FRONTEND=noninteractive add-apt-repository ppa:ondrej/php -y
        waitForAptUnlock
        sudo DEBIAN_FRONTEND=noninteractive apt-get update -y
        echo "ondrej/php PPA installed successfully"
    else
        echo "ondrej/php PPA already installed, refreshing package lists..."
        waitForAptUnlock
        sudo DEBIAN_FRONTEND=noninteractive apt-get update -y
    fi
}`
}

// TaskMarkerFunctions returns functions for task status markers.
// These are used by the StreamMonitor to detect task status in SSH output.
func TaskMarkerFunctions() string {
	return `# Task output markers for SSH streaming
function taskStarted() {
    echo "::LAUNCH_TASK_STARTED::"
}

function taskFinished() {
    local exit_code=${1:-0}
    echo "::LAUNCH_EXIT_CODE::${exit_code}"
    echo "::LAUNCH_TASK_FINISHED::"
}

function taskFailed() {
    local exit_code=${1:-1}
    echo "::LAUNCH_EXIT_CODE::${exit_code}"
    echo "::LAUNCH_TASK_FAILED::"
}

function taskProgress() {
    local progress=${1:-0}
    echo "::LAUNCH_TASK_PROGRESS::${progress}"
}

function taskStatus() {
    local message="$1"
    echo "::LAUNCH_TASK_STATUS::${message}"
}`
}

// Engine provides template parsing and rendering with common functions.
type Engine struct {
	templates *template.Template
	funcMap   template.FuncMap
}

// NewEngine creates a new template engine with common functions.
func NewEngine() *Engine {
	funcMap := make(template.FuncMap)
	for k, v := range CommonFuncMap {
		funcMap[k] = v
	}

	return &Engine{
		templates: template.New("").Funcs(funcMap),
		funcMap:   funcMap,
	}
}

// LoadFromFS loads templates from an embedded filesystem.
// Templates are named by their path relative to the root.
func (e *Engine) LoadFromFS(fsys embed.FS, root string) error {
	return fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
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

		// Strip the root prefix from the template name
		name := strings.TrimPrefix(path, root+"/")
		if name == path {
			name = strings.TrimPrefix(path, root)
		}

		_, err = e.templates.New(name).Parse(string(content))
		if err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}

		return nil
	})
}

// AddFunc adds a custom template function.
func (e *Engine) AddFunc(name string, fn interface{}) {
	e.funcMap[name] = fn
	e.templates = e.templates.Funcs(template.FuncMap{name: fn})
}

// Render executes a named template with the given data.
func (e *Engine) Render(name string, data interface{}) (string, error) {
	var buf bytes.Buffer
	if err := e.templates.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("template %s: %w", name, err)
	}
	return buf.String(), nil
}

// MustRender executes a template and panics on error.
func (e *Engine) MustRender(name string, data interface{}) string {
	result, err := e.Render(name, data)
	if err != nil {
		panic(err)
	}
	return result
}

// RenderString renders an inline template string with the given data.
func (e *Engine) RenderString(templateStr string, data interface{}) (string, error) {
	tmpl, err := e.templates.Clone()
	if err != nil {
		return "", err
	}

	tmpl, err = tmpl.Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("parsing template string: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("rendering template: %w", err)
	}

	return buf.String(), nil
}

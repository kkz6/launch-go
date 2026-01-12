package templates

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"text/template"
)

//go:embed provision/*.sh
//go:embed software/*.sh
var templateFS embed.FS

// templates holds all parsed templates
var templates *template.Template

func init() {
	templates = template.New("").Funcs(templateFuncs)

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

// templateFuncs provides custom template functions
var templateFuncs = template.FuncMap{
	"shellDefaults":   ShellDefaults,
	"aptFunctions":    AptFunctions,
	"commonFuncs":     CommonFunctions,
	"phpPpaFunctions": PhpPpaFunctions,
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

// ShellDefaults returns the standard shell script header
func ShellDefaults() string {
	return `set -euo pipefail
export DEBIAN_FRONTEND=noninteractive`
}

// CommonFunctions returns common bash helper functions
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

// AptFunctions returns apt-related bash functions
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

// PhpPpaFunctions returns the PHP PPA installation function
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

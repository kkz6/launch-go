package tasks

import (
	"encoding/base64"
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

const (
	PHPConfigValidationFailedMarker = "::PHP_CONFIG_VALIDATION_FAILED::"
	PHPConfigReloadFailedMarker     = "::PHP_CONFIG_RELOAD_FAILED::"
)

// ReadPHPConfigurationConfig holds trusted server-side configuration for a
// PHP configuration read. Path must be derived by the API, never the client.
type ReadPHPConfigurationConfig struct {
	Path     string
	MaxBytes int
}

// ReadPHPConfiguration reads an exact PHP configuration file while refusing
// missing or unexpectedly large files.
func ReadPHPConfiguration(config ReadPHPConfigurationConfig) *taskrunner.BaseTask {
	maxBytes := config.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 256 * 1024
	}

	quotedPath := taskrunner.ShellQuote(config.Path)
	script := fmt.Sprintf(`#!/bin/bash
set -euo pipefail

target=%s
if [ ! -f "$target" ]; then
    echo "PHP configuration file not found" >&2
    exit 44
fi

size="$(wc -c < "$target")"
if [ "$size" -gt %d ]; then
    echo "PHP configuration file exceeds the supported size" >&2
    exit 45
fi

cat -- "$target"
`, quotedPath, maxBytes)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Read PHP Configuration"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// UpdatePHPConfigurationConfig holds trusted server-side configuration for a
// PHP configuration update.
type UpdatePHPConfigurationConfig struct {
	Kind     string
	Version  string
	Path     string
	Contents string
}

// UpdatePHPConfiguration validates a candidate file before installation,
// retains the previous version as a recovery copy, and restores it when the
// PHP-FPM reload fails.
func UpdatePHPConfiguration(config UpdatePHPConfigurationConfig) *taskrunner.BaseTask {
	quotedPath := taskrunner.ShellQuote(config.Path)
	quotedVersion := taskrunner.ShellQuote(config.Version)
	quotedKind := taskrunner.ShellQuote(config.Kind)
	encodedContents := taskrunner.ShellQuote(base64.StdEncoding.EncodeToString([]byte(config.Contents)))

	script := fmt.Sprintf(`#!/bin/bash
set -euo pipefail

target=%s
version=%s
kind=%s
candidate="$(mktemp)"
validation_output="$(mktemp)"
backup="${target}.launchctl.bak"
cleanup() {
    rm -f -- "$candidate" "$validation_output"
}
trap cleanup EXIT

printf '%%s' %s | base64 --decode > "$candidate"
chmod 0644 "$candidate"

php_fpm="$(command -v "php-fpm${version}" || true)"
if [ -z "$php_fpm" ]; then
    echo "PHP-FPM ${version} is not installed" >&2
    exit 46
fi

if [ "$kind" = "php_ini" ]; then
    if ! "$php_fpm" -t -c "$candidate" -y "/etc/php/${version}/fpm/php-fpm.conf" >"$validation_output" 2>&1; then
        echo "%s" >&2
        cat "$validation_output" >&2
        exit 42
    fi
elif [ "$kind" = "php_fpm" ]; then
    if ! "$php_fpm" -t -c "/etc/php/${version}/fpm/php.ini" -y "$candidate" >"$validation_output" 2>&1; then
        echo "%s" >&2
        cat "$validation_output" >&2
        exit 42
    fi
else
    echo "Unsupported PHP configuration kind" >&2
    exit 43
fi

cp --preserve=mode,ownership,timestamps -- "$target" "$backup"
install -o root -g root -m 0644 "$candidate" "$target"

if ! systemctl reload "php${version}-fpm"; then
    cp --preserve=mode,ownership,timestamps -- "$backup" "$target"
    systemctl reload "php${version}-fpm" || true
    echo "%s" >&2
    exit 47
fi

echo "PHP configuration updated successfully"
`, quotedPath, quotedVersion, quotedKind, encodedContents, PHPConfigValidationFailedMarker, PHPConfigValidationFailedMarker, PHPConfigReloadFailedMarker)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Update PHP Configuration"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(60),
	)
}

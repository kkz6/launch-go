package tasks

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/server/types"
)

func TestPatchPHPTask(t *testing.T) {
	task := PatchPHP(types.SoftwarePhp83)
	script := task.Script()

	assert.Equal(t, "Patch PHP 8.3", task.Name())
	assert.Equal(t, 15*time.Minute, task.Timeout())
	assert.Contains(t, script, `PHP_SERIES="8.3"`)
	assert.Contains(t, script, `ubuntu|debian`)
	assert.Contains(t, script, `dpkg-query`)
	assert.Contains(t, script, `"php${PHP_SERIES}*"`)
	assert.Contains(t, script, `--only-upgrade`)
	assert.Contains(t, script, `--force-confold`)
	assert.Contains(t, script, `systemctl restart "${FPM_SERVICE}"`)
	assert.Contains(t, script, `systemctl is-active --quiet "${FPM_SERVICE}"`)
	assert.Contains(t, script, `LAUNCH_PHP_PATCH_VERSION=${installedVersion}`)
	assert.NotContains(t, script, "apt-get dist-upgrade")
	assert.NotContains(t, script, "\napt-get upgrade")
	assert.NotContains(t, script, "\nsudo apt-get upgrade")
}

func TestParsePatchedPHPVersion(t *testing.T) {
	t.Parallel()

	output := strings.Join([]string{
		"Updating package lists",
		"LAUNCH_PHP_PATCH_VERSION=8.3.12",
		"PHP 8.3.12 patched successfully",
	}, "\n")

	assert.Equal(t, "8.3.12", ParsePatchedPHPVersion(output))
	assert.Empty(t, ParsePatchedPHPVersion("PHP patch finished without marker"))
}

func TestParsePreviousDefaultPHPVersion(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t,
		"8.2",
		ParsePreviousDefaultPHPVersion(
			"LAUNCH_PREVIOUS_DEFAULT_PHP_VERSION=8.2\nPHP switched",
		),
	)
	assert.Empty(t, ParsePreviousDefaultPHPVersion("marker unavailable"))
}

func TestUpdateAlternativesSwitchesEveryTarget(t *testing.T) {
	output, callLines, err := runUpdateAlternativesScript(t)

	require.NoError(t, err, output)
	assert.Contains(t, output, "LAUNCH_PREVIOUS_DEFAULT_PHP_VERSION=8.2")
	assert.Equal(t, []string{
		"update-alternatives --set php /usr/bin/php8.3",
		"update-alternatives --set php-config /usr/bin/php-config8.3",
		"update-alternatives --set phpize /usr/bin/phpize8.3",
	}, callLines[len(callLines)-3:])
	assert.NotContains(
		t,
		strings.Join(callLines, "\n"),
		"update-alternatives --set php /usr/bin/php8.2",
	)
}

func TestUpdateAlternativesRollsBackAfterPartialFailure(t *testing.T) {
	task := UpdateAlternatives("8.3")
	script := task.Script()

	assert.Contains(t, script, "set -euo pipefail")

	output, callLines, err := runUpdateAlternativesScript(t, "FAIL_FORWARD=php-config")

	require.Error(t, err, output)
	var exitError *exec.ExitError
	require.True(t, errors.As(err, &exitError))
	assert.Equal(t, 17, exitError.ExitCode())
	assert.Contains(t, output, "LAUNCH_PREVIOUS_DEFAULT_PHP_VERSION=8.2")
	assert.Equal(t, []string{
		"update-alternatives --query php",
		"update-alternatives --query php-config",
		"update-alternatives --query phpize",
		"update-alternatives --list php",
		"update-alternatives --list php-config",
		"update-alternatives --list phpize",
		"update-alternatives --set php /usr/bin/php8.3",
		"update-alternatives --set php-config /usr/bin/php-config8.3",
		"update-alternatives --set php /usr/bin/php8.2",
		"update-alternatives --set php-config /usr/bin/php-config8.2",
		"update-alternatives --set phpize /usr/bin/phpize8.2",
	}, callLines)
	assert.NotContains(t, callLines, "update-alternatives --set phpize /usr/bin/phpize8.3")
}

func TestUpdateAlternativesPreflightsEveryTarget(t *testing.T) {
	output, callLines, err := runUpdateAlternativesScript(t, "MISSING_TARGET=phpize")

	require.Error(t, err, output)
	var exitError *exec.ExitError
	require.True(t, errors.As(err, &exitError))
	assert.Equal(t, 1, exitError.ExitCode())
	assert.Contains(t, output, "LAUNCH_PREVIOUS_DEFAULT_PHP_VERSION=8.2")
	assert.Contains(
		t,
		output,
		"PHP alternative phpize does not include target /usr/bin/phpize8.3",
	)
	assert.NotContains(t, strings.Join(callLines, "\n"), "update-alternatives --set")
}

func TestUpdateAlternativesPreservesOriginalFailureWhenRollbackFails(t *testing.T) {
	output, callLines, err := runUpdateAlternativesScript(
		t,
		"FAIL_FORWARD=php-config",
		"FAIL_ROLLBACK=phpize",
	)

	require.Error(t, err, output)
	var exitError *exec.ExitError
	require.True(t, errors.As(err, &exitError))
	assert.Equal(t, 17, exitError.ExitCode())
	assert.Contains(t, output, "Failed to completely roll back PHP alternatives to 8.2")
	assert.Contains(
		t,
		callLines,
		"update-alternatives --set phpize /usr/bin/phpize8.2",
	)
}

func runUpdateAlternativesScript(t *testing.T, extraEnv ...string) (string, []string, error) {
	t.Helper()

	script := UpdateAlternatives("8.3").Script()
	binDir := t.TempDir()
	callLog := filepath.Join(binDir, "calls.log")
	fakeSudo := filepath.Join(binDir, "sudo")
	fakePHP := filepath.Join(binDir, "php")
	require.NoError(t, os.WriteFile(fakePHP, []byte(`#!/bin/bash
printf '8.2'
`), 0o700))
	require.NoError(t, os.WriteFile(fakeSudo, []byte(`#!/bin/bash
printf '%s\n' "$*" >> "$CALL_LOG"

case "${2:-}" in
    --query)
        case "${3:-}" in
            php) value="/usr/bin/php8.2" ;;
            php-config) value="/usr/bin/php-config8.2" ;;
            phpize) value="/usr/bin/phpize8.2" ;;
            *) exit 2 ;;
        esac
        printf 'Value: %s\n' "${value}"
        ;;
    --list)
        case "${3:-}" in
            php)
                previous="/usr/bin/php8.2"
                target="/usr/bin/php8.3"
                ;;
            php-config)
                previous="/usr/bin/php-config8.2"
                target="/usr/bin/php-config8.3"
                ;;
            phpize)
                previous="/usr/bin/phpize8.2"
                target="/usr/bin/phpize8.3"
                ;;
            *) exit 2 ;;
        esac
        if [[ "${MISSING_TARGET:-}" != "${3}" ]]; then
            printf '%s\n' "${target}"
        fi
        printf '%s\n' "${previous}"
        ;;
    --set)
        if [[ "${FAIL_FORWARD:-}" == "${3:-}" && "${4:-}" == *"8.3" ]]; then
            exit 17
        fi
        if [[ "${FAIL_ROLLBACK:-}" == "${3:-}" && "${4:-}" == *"8.2" ]]; then
            exit 23
        fi
        ;;
    *)
        exit 2
        ;;
esac
`), 0o700))

	command := exec.Command("/bin/bash", "-c", script)
	command.Env = append(
		os.Environ(),
		"PATH="+binDir+":"+os.Getenv("PATH"),
		"CALL_LOG="+callLog,
	)
	command.Env = append(command.Env, extraEnv...)
	output, err := command.CombinedOutput()

	calls, readErr := os.ReadFile(callLog)
	require.NoError(t, readErr)
	callLines := strings.Split(strings.TrimSpace(string(calls)), "\n")

	return string(output), callLines, err
}

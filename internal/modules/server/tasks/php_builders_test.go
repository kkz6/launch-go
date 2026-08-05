package tasks

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// aptScript
// =============================================================================

// Scripts built inline here don't go through the .sh templates, so they only
// reach apt safely if aptScript supplies the same preamble {{ shellDefaults }}
// and {{ aptFunctions }} give the templates.
func TestAptScriptPreamble(t *testing.T) {
	script := aptScript("echo body")

	assert.True(t, strings.HasPrefix(script, "#!/bin/bash\n"), "script must start with a shebang")
	assert.Contains(t, script, "set -euo pipefail")
	assert.Contains(t, script, "function waitForAptUnlock()")
	assert.Contains(t, script, "function aptGet()")
	assert.Contains(t, script, "echo body")

	assert.Less(t, strings.Index(script, "function aptGet()"), strings.Index(script, "echo body"),
		"helpers must be defined before the body that calls them")
}

// =============================================================================
// AddPhpVersion
// =============================================================================

func TestAddPhpVersion(t *testing.T) {
	task := AddPhpVersion("8.2")
	require.NotNil(t, task)

	assert.Equal(t, "Install PHP 8.2", task.Name())
	assert.Equal(t, 600*time.Second, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, "aptGet install -y php8.2 php8.2-fpm")
	assert.Contains(t, script, "waitForAptUnlock")
	assert.Contains(t, script, "function aptGet()")
	assert.NotContains(t, script, "sudo apt-get", "apt must go through the wrapper for the lock timeout")
}

// Ubuntu gets the ondrej PPA, Debian the sury.org repo; anything else has to
// fail with a clear message rather than a confusing apt error.
func TestAddPhpVersionBranchesPerDistro(t *testing.T) {
	script := AddPhpVersion("8.3").Script()

	assert.Contains(t, script, ". /etc/os-release")
	assert.Contains(t, script, "add-apt-repository ppa:ondrej/php")
	assert.Contains(t, script, "packages.sury.org")
	assert.Contains(t, script, "ERROR: PHP install supports only Ubuntu or Debian")
}

// =============================================================================
// RemovePhpVersion
// =============================================================================

func TestRemovePhpVersion(t *testing.T) {
	task := RemovePhpVersion("8.1")
	require.NotNil(t, task)

	assert.Equal(t, "Remove PHP 8.1", task.Name())
	assert.Equal(t, 300*time.Second, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, "aptGet purge -y 'php8.1-*'")
	assert.Contains(t, script, "aptGet autoremove -y")
	assert.Contains(t, script, "waitForAptUnlock")
	assert.NotContains(t, script, "sudo apt-get")
}

// =============================================================================
// PHP extensions
// =============================================================================

func TestInstallPhpExtension(t *testing.T) {
	task := InstallPhpExtension("8.2", "redis")
	require.NotNil(t, task)

	assert.Equal(t, "Install PHP Extension redis", task.Name())
	assert.Equal(t, 300*time.Second, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, "aptGet install -y php8.2-redis")
	assert.Contains(t, script, "sudo service php8.2-fpm restart")
	assert.Contains(t, script, "waitForAptUnlock")
	assert.NotContains(t, script, "sudo apt-get")
}

func TestUninstallPhpExtension(t *testing.T) {
	task := UninstallPhpExtension("8.2", "imagick")
	require.NotNil(t, task)

	assert.Equal(t, "Uninstall PHP Extension imagick", task.Name())
	assert.Equal(t, 120*time.Second, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, "aptGet purge -y php8.2-imagick")
	assert.Contains(t, script, "sudo service php8.2-fpm restart")
	assert.Contains(t, script, "waitForAptUnlock")
	assert.NotContains(t, script, "sudo apt-get")
}

// =============================================================================
// OPcache
// =============================================================================

func TestGetOpcacheStatus(t *testing.T) {
	task := GetOpcacheStatus("8.3")
	require.NotNil(t, task)

	assert.Equal(t, 30*time.Second, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, "php8.3")
	// The probe runs under a PHP binary that may not have OPcache loaded at
	// all; it has to answer with JSON either way so the caller can parse it.
	assert.Contains(t, script, "extension_loaded('Zend OPcache')")
	assert.Contains(t, script, "opcache_get_status")
	assert.Contains(t, script, "json_encode")
}

func TestResetOpcache(t *testing.T) {
	task := ResetOpcache("8.3", "/home/launch/site")
	require.NotNil(t, task)

	assert.Equal(t, "Reset OPcache", task.Name())
	assert.Equal(t, 30*time.Second, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, "sudo service php8.3-fpm reload")
	// FPM keeps its own OPcache instance, so a failed reload has to escalate
	// to a restart or the cache survives the "reset".
	assert.Contains(t, script, "sudo service php8.3-fpm restart")
}

func TestClearOpcache(t *testing.T) {
	task := ClearOpcache()
	require.NotNil(t, task)

	assert.Equal(t, "Clear OPcache", task.Name())
	assert.Equal(t, 60*time.Second, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, "systemctl list-units")
	assert.Contains(t, script, "php.*-fpm")
	assert.Contains(t, script, "systemctl reload")
}

func TestConfigureOpcache(t *testing.T) {
	task := ConfigureOpcache("8.3", map[string]string{
		"memory_consumption":      "256",
		"max_accelerated_files":   "20000",
		"validate_timestamps":     "0",
		"interned_strings_buffer": "16",
	})
	require.NotNil(t, task)

	assert.Equal(t, "Configure OPcache", task.Name())
	assert.Equal(t, 60*time.Second, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, "/etc/php/8.3/mods-available/opcache-custom.ini")
	assert.Contains(t, script, "opcache.memory_consumption=256")
	assert.Contains(t, script, "opcache.max_accelerated_files=20000")
	assert.Contains(t, script, "opcache.validate_timestamps=0")
	assert.Contains(t, script, "sudo phpenmod -v 8.3 opcache-custom")
	assert.Contains(t, script, "sudo service php8.3-fpm restart")
}

// Map iteration order is random, so the keys are sorted before rendering —
// otherwise the same settings would produce a different script every call and
// churn the ini file on every save.
func TestConfigureOpcacheIsDeterministic(t *testing.T) {
	settings := map[string]string{
		"memory_consumption":      "256",
		"max_accelerated_files":   "20000",
		"validate_timestamps":     "0",
		"interned_strings_buffer": "16",
		"revalidate_freq":         "2",
	}

	first := ConfigureOpcache("8.3", settings).Script()
	for i := 0; i < 20; i++ {
		assert.Equal(t, first, ConfigureOpcache("8.3", settings).Script())
	}

	assert.Less(t,
		strings.Index(first, "opcache.interned_strings_buffer"),
		strings.Index(first, "opcache.max_accelerated_files"),
		"settings should render in sorted key order")
}

func TestConfigureOpcacheWithNoSettings(t *testing.T) {
	task := ConfigureOpcache("8.3", nil)
	require.NotNil(t, task)

	script := task.Script()
	assert.Contains(t, script, "[opcache]")
	assert.Contains(t, script, "OPCACHE_EOF")
}

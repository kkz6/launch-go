package tasks_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/tests/testutil"
)

func TestAddPhpVersion(t *testing.T) {
	task := tasks.AddPhpVersion("8.2")

	testutil.AssertTask(t, task).
		HasName("Install PHP 8.2").
		ScriptContains("add-apt-repository ppa:ondrej/php").
		ScriptContains("aptGet install").
		ScriptContains("php8.2").
		ScriptContains("php8.2-fpm").
		ScriptContains("php8.2-cli").
		ScriptContains("php8.2-mysql").
		ScriptMatches("tasks/php_add_version")
}

func TestRemovePhpVersion(t *testing.T) {
	task := tasks.RemovePhpVersion("8.1")

	testutil.AssertTask(t, task).
		HasName("Remove PHP 8.1").
		ScriptContains("aptGet purge").
		ScriptContains("php8.1").
		ScriptContains("autoremove").
		ScriptMatches("tasks/php_remove_version")
}

func TestInstallPhpExtension(t *testing.T) {
	task := tasks.InstallPhpExtension("8.2", "redis")

	testutil.AssertTask(t, task).
		HasName("Install PHP Extension redis").
		ScriptContains("aptGet install").
		ScriptContains("php8.2-redis").
		ScriptContains("php8.2-fpm restart").
		ScriptMatches("tasks/php_install_extension")
}

func TestUninstallPhpExtension(t *testing.T) {
	task := tasks.UninstallPhpExtension("8.2", "imagick")

	testutil.AssertTask(t, task).
		HasName("Uninstall PHP Extension imagick").
		ScriptContains("aptGet purge").
		ScriptContains("php8.2-imagick").
		ScriptContains("php8.2-fpm restart").
		ScriptMatches("tasks/php_uninstall_extension")
}

func TestUpdateAlternatives(t *testing.T) {
	task := tasks.UpdateAlternatives("8.2")

	testutil.AssertTask(t, task).
		HasName("Update PHP Alternatives").
		ScriptContains("update-alternatives --set php").
		ScriptContains("/usr/bin/php8.2").
		ScriptContains("php-config8.2").
		ScriptContains("phpize8.2").
		ScriptMatches("tasks/php_update_alternatives")
}

func TestGetOpcacheStatus(t *testing.T) {
	task := tasks.GetOpcacheStatus("8.2")

	testutil.AssertTask(t, task).
		HasName("Get OPcache Status").
		ScriptContains("php8.2").
		ScriptContains("opcache_get_status").
		ScriptContains("json_encode").
		ScriptMatches("tasks/php_opcache_status")
}

func TestResetOpcache(t *testing.T) {
	task := tasks.ResetOpcache("8.2", "/home/launcher/mysite")

	testutil.AssertTask(t, task).
		HasName("Reset OPcache").
		ScriptContains("php8.2-fpm reload").
		ScriptContains("php8.2-fpm restart").
		ScriptMatches("tasks/php_reset_opcache")
}

func TestClearOpcache(t *testing.T) {
	task := tasks.ClearOpcache()

	testutil.AssertTask(t, task).
		HasName("Clear OPcache").
		ScriptContains("php.*-fpm").
		ScriptContains("systemctl reload").
		ScriptMatches("tasks/php_clear_opcache")
}

func TestConfigureOpcache(t *testing.T) {
	settings := map[string]string{
		"memory_consumption":      "128",
		"interned_strings_buffer": "8",
		"max_accelerated_files":   "10000",
		"revalidate_freq":         "2",
		"enable_cli":              "1",
	}

	task := tasks.ConfigureOpcache("8.2", settings)

	testutil.AssertTask(t, task).
		HasName("Configure OPcache").
		ScriptContains("/etc/php/8.2/mods-available/opcache-custom.ini").
		ScriptContains("phpenmod").
		ScriptContains("php8.2-fpm restart").
		ScriptMatches("tasks/php_configure_opcache")
}

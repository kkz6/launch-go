package tasks_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/tests/testutil"
)

func TestRestartService_Generic(t *testing.T) {
	task := tasks.RestartService("nginx")

	testutil.AssertTask(t, task).
		HasName("Restart nginx").
		ScriptContains("sudo service nginx restart").
		ScriptMatches("tasks/service_restart_generic")
}

func TestRestartMySql(t *testing.T) {
	task := tasks.RestartMySql()

	testutil.AssertTask(t, task).
		HasName("Restart mysql").
		ScriptContains("sudo service mysql restart").
		ScriptMatches("tasks/service_restart_mysql")
}

func TestRestartPostgreSql(t *testing.T) {
	task := tasks.RestartPostgreSql()

	testutil.AssertTask(t, task).
		HasName("Restart postgresql").
		ScriptContains("sudo service postgresql restart").
		ScriptMatches("tasks/service_restart_postgresql")
}

func TestRestartRedis(t *testing.T) {
	task := tasks.RestartRedis()

	testutil.AssertTask(t, task).
		HasName("Restart redis-server").
		ScriptContains("sudo service redis-server restart").
		ScriptMatches("tasks/service_restart_redis")
}

func TestRestartPhp(t *testing.T) {
	task := tasks.RestartPhp("8.3")

	testutil.AssertTask(t, task).
		HasName("Restart php8.3-fpm").
		ScriptContains("sudo service php8.3-fpm restart").
		ScriptMatches("tasks/service_restart_php")
}

func TestReloadCaddy(t *testing.T) {
	task := tasks.ReloadCaddy()

	testutil.AssertTask(t, task).
		HasName("Reload Caddy").
		ScriptContains("sudo systemctl reload caddy").
		ScriptMatches("tasks/service_reload_caddy")
}

func TestCheckServiceStatus(t *testing.T) {
	task := tasks.CheckServiceStatus("nginx")

	testutil.AssertTask(t, task).
		HasName("Check nginx Status").
		ScriptContains("systemctl is-active nginx").
		ScriptMatches("tasks/service_check_status")
}

func TestStopService(t *testing.T) {
	task := tasks.StopService("nginx")

	testutil.AssertTask(t, task).
		HasName("Stop nginx").
		ScriptContains("sudo service nginx stop").
		ScriptMatches("tasks/service_stop")
}

func TestStartService(t *testing.T) {
	task := tasks.StartService("nginx")

	testutil.AssertTask(t, task).
		HasName("Start nginx").
		ScriptContains("sudo service nginx start").
		ScriptMatches("tasks/service_start")
}

func TestReloadService(t *testing.T) {
	task := tasks.ReloadService("nginx")

	testutil.AssertTask(t, task).
		HasName("Reload nginx").
		ScriptContains("sudo systemctl reload nginx").
		ScriptMatches("tasks/service_reload")
}

func TestRebootServer(t *testing.T) {
	task := tasks.RebootServer()

	testutil.AssertTask(t, task).
		HasName("Reboot Server").
		ScriptContains("sudo reboot").
		ScriptMatches("tasks/service_reboot")
}

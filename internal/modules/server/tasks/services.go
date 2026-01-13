package tasks

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// RestartServiceTask restarts a service
type RestartServiceTask struct {
	*ServerTask
	Service string
}

// NewRestartServiceTask creates a new RestartServiceTask
func NewRestartServiceTask(service string) *RestartServiceTask {
	script := fmt.Sprintf("sudo service %s restart", service)
	return &RestartServiceTask{
		ServerTask: NewServerTaskWithName(fmt.Sprintf("Restart %s", service), script, 30),
		Service:    service,
	}
}

// RestartService creates a task to restart a service
func RestartService(service string) *taskrunner.BaseTask {
	return taskrunner.NewBaseTask(
		taskrunner.WithName(fmt.Sprintf("Restart %s", service)),
		taskrunner.WithScript(fmt.Sprintf("sudo service %s restart", service)),
		taskrunner.WithTimeout(30),
	)
}

// RestartMySql creates a task to restart MySQL
func RestartMySql() *taskrunner.BaseTask {
	return RestartService("mysql")
}

// RestartPostgreSql creates a task to restart PostgreSQL
func RestartPostgreSql() *taskrunner.BaseTask {
	return RestartService("postgresql")
}

// RestartRedis creates a task to restart Redis
func RestartRedis() *taskrunner.BaseTask {
	return RestartService("redis-server")
}

// RestartNginx creates a task to restart Nginx
func RestartNginx() *taskrunner.BaseTask {
	return RestartService("nginx")
}

// RestartPhp creates a task to restart PHP-FPM
func RestartPhp(version string) *taskrunner.BaseTask {
	service := fmt.Sprintf("php%s-fpm", version)
	return RestartService(service)
}

// RestartSupervisor creates a task to restart Supervisor
func RestartSupervisor() *taskrunner.BaseTask {
	return RestartService("supervisor")
}

// ReloadCaddy creates a task to reload Caddy
func ReloadCaddy() *taskrunner.BaseTask {
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Reload Caddy"),
		taskrunner.WithScript("sudo systemctl reload caddy"),
		taskrunner.WithTimeout(30),
	)
}

// CheckServiceStatus creates a task to check service status
func CheckServiceStatus(service string) *taskrunner.BaseTask {
	return taskrunner.NewBaseTask(
		taskrunner.WithName(fmt.Sprintf("Check %s Status", service)),
		taskrunner.WithScript(fmt.Sprintf("systemctl is-active %s", service)),
		taskrunner.WithTimeout(15),
	)
}

// StopService creates a task to stop a service
func StopService(service string) *taskrunner.BaseTask {
	return taskrunner.NewBaseTask(
		taskrunner.WithName(fmt.Sprintf("Stop %s", service)),
		taskrunner.WithScript(fmt.Sprintf("sudo service %s stop", service)),
		taskrunner.WithTimeout(30),
	)
}

// StartService creates a task to start a service
func StartService(service string) *taskrunner.BaseTask {
	return taskrunner.NewBaseTask(
		taskrunner.WithName(fmt.Sprintf("Start %s", service)),
		taskrunner.WithScript(fmt.Sprintf("sudo service %s start", service)),
		taskrunner.WithTimeout(30),
	)
}

// ReloadService creates a task to reload a service
func ReloadService(service string) *taskrunner.BaseTask {
	return taskrunner.NewBaseTask(
		taskrunner.WithName(fmt.Sprintf("Reload %s", service)),
		taskrunner.WithScript(fmt.Sprintf("sudo systemctl reload %s", service)),
		taskrunner.WithTimeout(30),
	)
}

// RebootServer creates a task to reboot the server
func RebootServer() *taskrunner.BaseTask {
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Reboot Server"),
		taskrunner.WithScript("sudo reboot"),
		taskrunner.WithTimeout(15),
	)
}

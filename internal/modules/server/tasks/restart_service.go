package tasks

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// RestartService is a base task for restarting system services
type RestartService struct {
	taskrunner.BaseTask
	serviceName string
}

// NewRestartService creates a new RestartService task for any service
func NewRestartService(serviceName string) *RestartService {
	task := &RestartService{
		BaseTask: taskrunner.BaseTask{
			TaskName:     fmt.Sprintf("restart-%s", serviceName),
			TemplateName: "restart-service",
			TaskTimeout:  30 * time.Second,
		},
		serviceName: serviceName,
	}

	return task
}

// Data returns the template data
func (t *RestartService) Data() map[string]interface{} {
	return map[string]interface{}{
		"ServiceName": t.serviceName,
	}
}

// Script returns the command to run directly
func (t *RestartService) Script() (string, error) {
	return fmt.Sprintf("sudo service %s restart", t.serviceName), nil
}

// ServiceName returns the name of the service being restarted
func (t *RestartService) ServiceName() string {
	return t.serviceName
}

// RestartMySql restarts the MySQL service
type RestartMySql struct {
	RestartService
}

// NewRestartMySql creates a new RestartMySql task
func NewRestartMySql() *RestartMySql {
	base := NewRestartService("mysql")

	return &RestartMySql{
		RestartService: *base,
	}
}

// RestartPostgreSql restarts the PostgreSQL service
type RestartPostgreSql struct {
	RestartService
}

// NewRestartPostgreSql creates a new RestartPostgreSql task
func NewRestartPostgreSql() *RestartPostgreSql {
	base := NewRestartService("postgresql")

	return &RestartPostgreSql{
		RestartService: *base,
	}
}

// RestartRedis restarts the Redis service
type RestartRedis struct {
	RestartService
}

// NewRestartRedis creates a new RestartRedis task
func NewRestartRedis() *RestartRedis {
	base := NewRestartService("redis-server")

	return &RestartRedis{
		RestartService: *base,
	}
}

// RestartSupervisor restarts the Supervisor service
type RestartSupervisor struct {
	RestartService
}

// NewRestartSupervisor creates a new RestartSupervisor task
func NewRestartSupervisor() *RestartSupervisor {
	base := NewRestartService("supervisor")

	return &RestartSupervisor{
		RestartService: *base,
	}
}

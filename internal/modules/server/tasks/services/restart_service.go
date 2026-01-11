package services

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// RestartService restarts a systemd service
type RestartService struct {
	tasks.BaseServerTask
	serviceName string
}

// NewRestartService creates a new RestartService task
func NewRestartService(server *models.Server, serviceName string) *RestartService {
	task := &RestartService{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/services/restart-service",
				TaskTimeout:  30 * time.Second,
			},
		},
		serviceName: serviceName,
	}
	task.SetServer(server)

	return task
}

// Data returns the template data
func (t *RestartService) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":      t.Server(),
		"ServiceName": t.serviceName,
	}
}

// ServiceName returns the service name
func (t *RestartService) ServiceName() string {
	return t.serviceName
}

// BuildScript generates the shell script for restarting the service
func (t *RestartService) BuildScript() string {
	return fmt.Sprintf("sudo service %s restart", t.serviceName)
}

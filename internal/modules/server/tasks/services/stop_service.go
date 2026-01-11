package services

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// StopService stops a systemd service
type StopService struct {
	tasks.BaseServerTask
	serviceName string
}

// NewStopService creates a new StopService task
func NewStopService(server *models.Server, serviceName string) *StopService {
	task := &StopService{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/services/stop-service",
				TaskTimeout:  30 * time.Second,
			},
		},
		serviceName: serviceName,
	}
	task.SetServer(server)

	return task
}

// Data returns the template data
func (t *StopService) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":      t.Server(),
		"ServiceName": t.serviceName,
	}
}

// ServiceName returns the service name
func (t *StopService) ServiceName() string {
	return t.serviceName
}

// BuildScript generates the shell script for stopping the service
func (t *StopService) BuildScript() string {
	return fmt.Sprintf("sudo service %s stop", t.serviceName)
}

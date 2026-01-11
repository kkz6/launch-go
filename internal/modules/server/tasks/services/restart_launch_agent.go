package services

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// RestartLaunchAgent restarts the launch-agent service
type RestartLaunchAgent struct {
	tasks.BaseServerTask
	restartService *RestartService
}

// NewRestartLaunchAgent creates a new RestartLaunchAgent task
func NewRestartLaunchAgent(server *models.Server) *RestartLaunchAgent {
	restartService := NewRestartService(server, "launch-agent")

	task := &RestartLaunchAgent{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/services/restart-launch-agent",
				TaskTimeout:  30 * time.Second,
			},
		},
		restartService: restartService,
	}
	task.SetServer(server)

	return task
}

// Data returns the template data
func (t *RestartLaunchAgent) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":      t.Server(),
		"ServiceName": "launch-agent",
	}
}

// ServiceName returns the service name
func (t *RestartLaunchAgent) ServiceName() string {
	return "launch-agent"
}

// BuildScript generates the shell script for restarting the launch-agent
func (t *RestartLaunchAgent) BuildScript() string {
	return t.restartService.BuildScript()
}

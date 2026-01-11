package services

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// StopLaunchAgent stops the launch-agent service
type StopLaunchAgent struct {
	tasks.BaseServerTask
	stopService *StopService
}

// NewStopLaunchAgent creates a new StopLaunchAgent task
func NewStopLaunchAgent(server *models.Server) *StopLaunchAgent {
	stopService := NewStopService(server, "launch-agent")

	task := &StopLaunchAgent{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/services/stop-launch-agent",
				TaskTimeout:  30 * time.Second,
			},
		},
		stopService: stopService,
	}
	task.SetServer(server)

	return task
}

// Data returns the template data
func (t *StopLaunchAgent) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":      t.Server(),
		"ServiceName": "launch-agent",
	}
}

// ServiceName returns the service name
func (t *StopLaunchAgent) ServiceName() string {
	return "launch-agent"
}

// BuildScript generates the shell script for stopping the launch-agent
func (t *StopLaunchAgent) BuildScript() string {
	return t.stopService.BuildScript()
}

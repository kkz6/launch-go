package services

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// RemoveLaunchAgent removes the launch-agent from the server
type RemoveLaunchAgent struct {
	tasks.BaseServerTask
}

// NewRemoveLaunchAgent creates a new RemoveLaunchAgent task
func NewRemoveLaunchAgent(server *models.Server) *RemoveLaunchAgent {
	task := &RemoveLaunchAgent{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/software/remove-launch-agent",
				TaskTimeout:  60 * time.Second,
			},
		},
	}
	task.SetServer(server)

	return task
}

// Data returns the template data
func (t *RemoveLaunchAgent) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server": t.Server(),
	}
}

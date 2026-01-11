package services

import (
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// RemoveBun removes Bun runtime from the server
type RemoveBun struct {
	tasks.BaseServerTask
}

// NewRemoveBun creates a new RemoveBun task
func NewRemoveBun(server *models.Server) *RemoveBun {
	task := &RemoveBun{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/services/remove-bun",
				TaskTimeout:  60 * time.Second,
			},
		},
	}
	task.SetServer(server)

	return task
}

// Data returns the template data
func (t *RemoveBun) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server": t.Server(),
	}
}

// BuildScript generates the shell script for removing Bun
func (t *RemoveBun) BuildScript() string {
	commands := []string{
		`echo "Removing Bun..."`,
		"rm -rf ~/.bun",
		"sudo rm -f /usr/local/bin/bun",
		"sudo rm -f /usr/bin/bun",
		`echo "Bun removed successfully"`,
	}

	return strings.Join(commands, " && ")
}

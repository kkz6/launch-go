package services

import (
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// CheckBunStatus checks the status of Bun runtime
type CheckBunStatus struct {
	tasks.BaseServerTask
}

// NewCheckBunStatus creates a new CheckBunStatus task
func NewCheckBunStatus(server *models.Server) *CheckBunStatus {
	task := &CheckBunStatus{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/services/check-bun-status",
				TaskTimeout:  30 * time.Second,
			},
		},
	}
	task.SetServer(server)

	return task
}

// Data returns the template data
func (t *CheckBunStatus) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server": t.Server(),
	}
}

// BuildScript generates the shell script for checking Bun status
func (t *CheckBunStatus) BuildScript() string {
	commands := []string{
		`echo "===BUN_VERSION==="`,
		`bun --version 2>/dev/null || echo "Bun not found"`,
		`echo "===BUN_LOCATION==="`,
		`which bun 2>/dev/null || echo "Bun not in PATH"`,
		`echo "===BUN_INFO==="`,
		`bun --help 2>/dev/null | head -5 || echo "Bun help not available"`,
	}

	return strings.Join(commands, " && ")
}

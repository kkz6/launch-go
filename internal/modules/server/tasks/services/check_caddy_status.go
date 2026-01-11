package services

import (
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// CheckCaddyStatus checks the status of Caddy web server
type CheckCaddyStatus struct {
	tasks.BaseServerTask
}

// NewCheckCaddyStatus creates a new CheckCaddyStatus task
func NewCheckCaddyStatus(server *models.Server) *CheckCaddyStatus {
	task := &CheckCaddyStatus{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/services/check-caddy-status",
				TaskTimeout:  30 * time.Second,
			},
		},
	}
	task.SetServer(server)

	return task
}

// Data returns the template data
func (t *CheckCaddyStatus) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":      t.Server(),
		"ServiceName": "caddy",
	}
}

// ServiceName returns the systemd service name
func (t *CheckCaddyStatus) ServiceName() string {
	return "caddy"
}

// BuildScript generates the shell script for checking Caddy status
func (t *CheckCaddyStatus) BuildScript() string {
	commands := []string{
		"sudo systemctl status caddy --no-pager --full",
		`echo "===PROCESSES==="`,
		"ps aux | grep [c]addy | head -5",
		`echo "===CONNECTIONS==="`,
		"sudo ss -tulpn | grep caddy",
		`echo "===MEMORY==="`,
		"sudo systemctl show caddy --property=MemoryCurrent,MemoryPeak",
		`echo "===CONFIG_TEST==="`,
		`timeout 5 sudo caddy validate --config /etc/caddy/Caddyfile 2>/dev/null || echo "Config validation not available"`,
	}

	return strings.Join(commands, " && ")
}

package services

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// CheckSupervisorStatus checks the status of Supervisor process manager
type CheckSupervisorStatus struct {
	tasks.BaseServerTask
	checkServiceStatus *CheckServiceStatus
}

// NewCheckSupervisorStatus creates a new CheckSupervisorStatus task
func NewCheckSupervisorStatus(server *models.Server) *CheckSupervisorStatus {
	checkServiceStatus := NewCheckServiceStatus(server, CheckServiceStatusConfig{
		ServiceName: "supervisor",
		AdditionalCmd: []string{
			`echo "===SUPERVISOR_STATUS==="`,
			`timeout 5 sudo supervisorctl status 2>/dev/null || echo "Supervisorctl not accessible"`,
			`echo "===PROGRAMS==="`,
			`sudo ls -la /etc/supervisor/conf.d/ 2>/dev/null || echo "Config directory not accessible"`,
		},
	})

	task := &CheckSupervisorStatus{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/services/check-supervisor-status",
				TaskTimeout:  30 * time.Second,
			},
		},
		checkServiceStatus: checkServiceStatus,
	}
	task.SetServer(server)

	return task
}

// Data returns the template data
func (t *CheckSupervisorStatus) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":      t.Server(),
		"ServiceName": "supervisor",
	}
}

// ServiceName returns the systemd service name
func (t *CheckSupervisorStatus) ServiceName() string {
	return "supervisor"
}

// BuildScript generates the shell script for checking Supervisor status
func (t *CheckSupervisorStatus) BuildScript() string {
	return t.checkServiceStatus.BuildScript()
}

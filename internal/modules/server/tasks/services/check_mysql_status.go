package services

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// CheckMySqlStatus checks the status of MySQL database server
type CheckMySqlStatus struct {
	tasks.BaseServerTask
	checkServiceStatus *CheckServiceStatus
}

// NewCheckMySqlStatus creates a new CheckMySqlStatus task
func NewCheckMySqlStatus(server *models.Server) *CheckMySqlStatus {
	port := 3306
	checkServiceStatus := NewCheckServiceStatus(server, CheckServiceStatusConfig{
		ServiceName: "mysql",
		Port:        &port,
	})

	task := &CheckMySqlStatus{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/services/check-mysql-status",
				TaskTimeout:  30 * time.Second,
			},
		},
		checkServiceStatus: checkServiceStatus,
	}
	task.SetServer(server)

	return task
}

// Data returns the template data
func (t *CheckMySqlStatus) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":      t.Server(),
		"ServiceName": "mysql",
		"Port":        3306,
	}
}

// ServiceName returns the systemd service name
func (t *CheckMySqlStatus) ServiceName() string {
	return "mysql"
}

// Port returns the MySQL port
func (t *CheckMySqlStatus) Port() int {
	return 3306
}

// BuildScript generates the shell script for checking MySQL status
func (t *CheckMySqlStatus) BuildScript() string {
	return t.checkServiceStatus.BuildScript()
}

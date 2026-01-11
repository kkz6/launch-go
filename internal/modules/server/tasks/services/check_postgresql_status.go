package services

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// CheckPostgreSqlStatus checks the status of PostgreSQL database server
type CheckPostgreSqlStatus struct {
	tasks.BaseServerTask
	checkServiceStatus *CheckServiceStatus
}

// NewCheckPostgreSqlStatus creates a new CheckPostgreSqlStatus task
func NewCheckPostgreSqlStatus(server *models.Server) *CheckPostgreSqlStatus {
	port := 5432
	checkServiceStatus := NewCheckServiceStatus(server, CheckServiceStatusConfig{
		ServiceName: "postgresql",
		GrepPattern: "[p]ostgres",
		Port:        &port,
	})

	task := &CheckPostgreSqlStatus{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/services/check-postgresql-status",
				TaskTimeout:  30 * time.Second,
			},
		},
		checkServiceStatus: checkServiceStatus,
	}
	task.SetServer(server)

	return task
}

// Data returns the template data
func (t *CheckPostgreSqlStatus) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":      t.Server(),
		"ServiceName": "postgresql",
		"GrepPattern": "[p]ostgres",
		"Port":        5432,
	}
}

// ServiceName returns the systemd service name
func (t *CheckPostgreSqlStatus) ServiceName() string {
	return "postgresql"
}

// GrepPattern returns the pattern for grep
func (t *CheckPostgreSqlStatus) GrepPattern() string {
	return "[p]ostgres"
}

// Port returns the PostgreSQL port
func (t *CheckPostgreSqlStatus) Port() int {
	return 5432
}

// BuildScript generates the shell script for checking PostgreSQL status
func (t *CheckPostgreSqlStatus) BuildScript() string {
	return t.checkServiceStatus.BuildScript()
}

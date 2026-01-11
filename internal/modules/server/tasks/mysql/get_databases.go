package mysql

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// GetDatabases retrieves a list of all MySQL databases
type GetDatabases struct {
	tasks.BaseServerTask
	hosts         []string
	mySQLUser     string
	mySQLPassword string
}

// NewGetDatabases creates a new GetDatabases task
func NewGetDatabases(server *models.Server) *GetDatabases {
	task := &GetDatabases{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/mysql/get-databases",
				TaskTimeout:  2 * time.Minute,
			},
		},
		hosts: []string{"%"},
	}
	task.SetServer(server)

	task.FinishedCallback = task.onFinished
	task.FailedCallback = task.onFailed

	return task
}

// WithCredentials sets the MySQL connection credentials
func (t *GetDatabases) WithCredentials(user, password string) *GetDatabases {
	t.mySQLUser = user
	t.mySQLPassword = password

	return t
}

// OnServer sets the credentials using the server's database credentials
func (t *GetDatabases) OnServer(server *models.Server) *GetDatabases {
	t.hosts = GetHostsForServer(server)
	t.mySQLUser = "root"
	t.mySQLPassword = GetServerDatabasePassword(server)

	return t
}

// Data returns the template data
func (t *GetDatabases) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":        t.Server(),
		"MySQLUser":     t.mySQLUser,
		"MySQLPassword": t.mySQLPassword,
		"SQL":           t.SQL(),
	}
}

// SQL returns the SQL query to list databases
func (t *GetDatabases) SQL() string {
	return "SHOW DATABASES;"
}

// onFinished handles successful completion
func (t *GetDatabases) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// Databases retrieved successfully
}

// onFailed handles task failure
func (t *GetDatabases) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	// Handle get databases failure
}

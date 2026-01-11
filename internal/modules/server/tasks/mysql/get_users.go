package mysql

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// GetUsers retrieves a list of all MySQL users
type GetUsers struct {
	tasks.BaseServerTask
	hosts         []string
	mySQLUser     string
	mySQLPassword string
}

// NewGetUsers creates a new GetUsers task
func NewGetUsers(server *models.Server) *GetUsers {
	task := &GetUsers{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/mysql/get-users",
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
func (t *GetUsers) WithCredentials(user, password string) *GetUsers {
	t.mySQLUser = user
	t.mySQLPassword = password

	return t
}

// OnServer sets the credentials using the server's database credentials
func (t *GetUsers) OnServer(server *models.Server) *GetUsers {
	t.hosts = GetHostsForServer(server)
	t.mySQLUser = "root"
	t.mySQLPassword = GetServerDatabasePassword(server)

	return t
}

// Data returns the template data
func (t *GetUsers) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":        t.Server(),
		"MySQLUser":     t.mySQLUser,
		"MySQLPassword": t.mySQLPassword,
		"SQL":           t.SQL(),
	}
}

// SQL returns the SQL query to list users
func (t *GetUsers) SQL() string {
	return "SELECT host, user FROM mysql.user;"
}

// onFinished handles successful completion
func (t *GetUsers) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// Users retrieved successfully
}

// onFailed handles task failure
func (t *GetUsers) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	// Handle get users failure
}

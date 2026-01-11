package mysql

import (
	"context"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// GetTables retrieves a list of tables from a MySQL database
type GetTables struct {
	tasks.BaseServerTask
	hosts         []string
	mySQLUser     string
	mySQLPassword string
	database      string
}

// NewGetTables creates a new GetTables task
func NewGetTables(server *models.Server, database string) *GetTables {
	task := &GetTables{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/mysql/get-tables",
				TaskTimeout:  2 * time.Minute,
			},
		},
		hosts:    []string{"%"},
		database: database,
	}
	task.SetServer(server)

	task.FinishedCallback = task.onFinished
	task.FailedCallback = task.onFailed

	return task
}

// WithCredentials sets the MySQL connection credentials
func (t *GetTables) WithCredentials(user, password string) *GetTables {
	t.mySQLUser = user
	t.mySQLPassword = password

	return t
}

// OnServer sets the credentials using the server's database credentials
func (t *GetTables) OnServer(server *models.Server) *GetTables {
	t.hosts = GetHostsForServer(server)
	t.mySQLUser = "root"
	t.mySQLPassword = GetServerDatabasePassword(server)

	return t
}

// Data returns the template data
func (t *GetTables) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":        t.Server(),
		"MySQLUser":     t.mySQLUser,
		"MySQLPassword": t.mySQLPassword,
		"SQL":           t.SQL(),
	}
}

// SQL returns the SQL query to list tables
func (t *GetTables) SQL() string {
	return fmt.Sprintf("USE %s; SHOW TABLES;", t.database)
}

// Database returns the database name
func (t *GetTables) Database() string {
	return t.database
}

// onFinished handles successful completion
func (t *GetTables) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// Tables retrieved successfully
}

// onFailed handles task failure
func (t *GetTables) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	// Handle get tables failure
}

package mysql

import (
	"context"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// DropDatabase drops a MySQL database
type DropDatabase struct {
	tasks.BaseServerTask
	hosts         []string
	mySQLUser     string
	mySQLPassword string
	name          string
}

// NewDropDatabase creates a new DropDatabase task
func NewDropDatabase(server *models.Server, name string) *DropDatabase {
	task := &DropDatabase{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/mysql/drop-database",
				TaskTimeout:  2 * time.Minute,
			},
		},
		hosts: []string{"%"},
		name:  name,
	}
	task.SetServer(server)

	task.FinishedCallback = task.onFinished
	task.FailedCallback = task.onFailed

	return task
}

// WithCredentials sets the MySQL connection credentials
func (t *DropDatabase) WithCredentials(user, password string) *DropDatabase {
	t.mySQLUser = user
	t.mySQLPassword = password

	return t
}

// OnServer sets the credentials using the server's database credentials
func (t *DropDatabase) OnServer(server *models.Server) *DropDatabase {
	t.hosts = GetHostsForServer(server)
	t.mySQLUser = "root"
	t.mySQLPassword = GetServerDatabasePassword(server)

	return t
}

// Data returns the template data
func (t *DropDatabase) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":        t.Server(),
		"MySQLUser":     t.mySQLUser,
		"MySQLPassword": t.mySQLPassword,
		"SQL":           t.SQL(),
	}
}

// SQL returns the SQL query to drop the database
func (t *DropDatabase) SQL() string {
	return fmt.Sprintf(
		"DROP DATABASE IF EXISTS %s;",
		WrapValue(t.name),
	)
}

// GetName returns the database name being dropped
func (t *DropDatabase) GetName() string {
	return t.name
}

// onFinished handles successful completion
func (t *DropDatabase) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// Database dropped successfully
}

// onFailed handles task failure
func (t *DropDatabase) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	// Handle database drop failure
}

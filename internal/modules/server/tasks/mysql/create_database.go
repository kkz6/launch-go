package mysql

import (
	"context"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// CreateDatabase creates a new MySQL database
type CreateDatabase struct {
	tasks.BaseServerTask
	hosts         []string
	mySQLUser     string
	mySQLPassword string
	name          string
	charset       string
	collation     string
}

// NewCreateDatabase creates a new CreateDatabase task
func NewCreateDatabase(server *models.Server, name, charset, collation string) *CreateDatabase {
	task := &CreateDatabase{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/mysql/create-database",
				TaskTimeout:  2 * time.Minute,
			},
		},
		hosts:     []string{"%"},
		name:      name,
		charset:   charset,
		collation: collation,
	}
	task.SetServer(server)

	task.FinishedCallback = task.onFinished
	task.FailedCallback = task.onFailed

	return task
}

// WithCredentials sets the MySQL connection credentials
func (t *CreateDatabase) WithCredentials(user, password string) *CreateDatabase {
	t.mySQLUser = user
	t.mySQLPassword = password

	return t
}

// OnServer sets the credentials using the server's database credentials
func (t *CreateDatabase) OnServer(server *models.Server) *CreateDatabase {
	t.hosts = GetHostsForServer(server)
	t.mySQLUser = "root"
	t.mySQLPassword = GetServerDatabasePassword(server)

	return t
}

// Data returns the template data
func (t *CreateDatabase) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":        t.Server(),
		"MySQLUser":     t.mySQLUser,
		"MySQLPassword": t.mySQLPassword,
		"SQL":           t.SQL(),
	}
}

// SQL returns the SQL query to create the database
func (t *CreateDatabase) SQL() string {
	return fmt.Sprintf(
		"CREATE DATABASE %s CHARACTER SET %s COLLATE %s;",
		WrapValue(t.name),
		WrapValue(t.charset),
		WrapValue(t.collation),
	)
}

// GetName returns the database name
func (t *CreateDatabase) GetName() string {
	return t.name
}

// Charset returns the database charset
func (t *CreateDatabase) Charset() string {
	return t.charset
}

// Collation returns the database collation
func (t *CreateDatabase) Collation() string {
	return t.collation
}

// onFinished handles successful completion
func (t *CreateDatabase) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// Database created successfully
}

// onFailed handles task failure
func (t *CreateDatabase) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	// Handle database creation failure
}

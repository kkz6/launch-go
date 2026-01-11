package mysql

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// DropUser drops a MySQL user
type DropUser struct {
	tasks.BaseServerTask
	hosts         []string
	mySQLUser     string
	mySQLPassword string
	userName      string
}

// NewDropUser creates a new DropUser task
func NewDropUser(server *models.Server, userName string) *DropUser {
	task := &DropUser{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/mysql/drop-user",
				TaskTimeout:  2 * time.Minute,
			},
		},
		hosts:    []string{"%"},
		userName: userName,
	}
	task.SetServer(server)

	task.FinishedCallback = task.onFinished
	task.FailedCallback = task.onFailed

	return task
}

// WithCredentials sets the MySQL connection credentials
func (t *DropUser) WithCredentials(user, password string) *DropUser {
	t.mySQLUser = user
	t.mySQLPassword = password

	return t
}

// OnServer sets the credentials using the server's database credentials
func (t *DropUser) OnServer(server *models.Server) *DropUser {
	t.hosts = GetHostsForServer(server)
	t.mySQLUser = "root"
	t.mySQLPassword = GetServerDatabasePassword(server)

	return t
}

// Data returns the template data
func (t *DropUser) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":        t.Server(),
		"MySQLUser":     t.mySQLUser,
		"MySQLPassword": t.mySQLPassword,
		"SQL":           t.SQL(),
	}
}

// SQL returns the SQL query to drop the user
func (t *DropUser) SQL() string {
	results := make([]string, len(t.hosts))
	for i, host := range t.hosts {
		results[i] = fmt.Sprintf(
			"DROP USER IF EXISTS %s@%s;",
			WrapValue(t.userName),
			WrapValue(host),
		)
	}

	return strings.Join(results, " ")
}

// UserName returns the username being dropped
func (t *DropUser) UserName() string {
	return t.userName
}

// onFinished handles successful completion
func (t *DropUser) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// User dropped successfully
}

// onFailed handles task failure
func (t *DropUser) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	// Handle user drop failure
}

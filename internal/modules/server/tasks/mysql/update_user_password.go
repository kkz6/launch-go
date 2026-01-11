package mysql

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// UpdateUserPassword updates a MySQL user's password
type UpdateUserPassword struct {
	tasks.BaseServerTask
	hosts         []string
	mySQLUser     string
	mySQLPassword string
	userName      string
	newPassword   string
}

// NewUpdateUserPassword creates a new UpdateUserPassword task
func NewUpdateUserPassword(server *models.Server, userName, newPassword string) *UpdateUserPassword {
	task := &UpdateUserPassword{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/mysql/update-user-password",
				TaskTimeout:  2 * time.Minute,
			},
		},
		hosts:       []string{"%"},
		userName:    userName,
		newPassword: newPassword,
	}
	task.SetServer(server)

	task.FinishedCallback = task.onFinished
	task.FailedCallback = task.onFailed

	return task
}

// WithCredentials sets the MySQL connection credentials
func (t *UpdateUserPassword) WithCredentials(user, password string) *UpdateUserPassword {
	t.mySQLUser = user
	t.mySQLPassword = password

	return t
}

// OnServer sets the credentials using the server's database credentials
func (t *UpdateUserPassword) OnServer(server *models.Server) *UpdateUserPassword {
	t.hosts = GetHostsForServer(server)
	t.mySQLUser = "root"
	t.mySQLPassword = GetServerDatabasePassword(server)

	return t
}

// Data returns the template data
func (t *UpdateUserPassword) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":        t.Server(),
		"MySQLUser":     t.mySQLUser,
		"MySQLPassword": t.mySQLPassword,
		"SQL":           t.SQL(),
	}
}

// SQL returns the SQL query to update the user password
func (t *UpdateUserPassword) SQL() string {
	results := make([]string, len(t.hosts))
	for i, host := range t.hosts {
		results[i] = fmt.Sprintf(
			"ALTER USER %s@%s IDENTIFIED BY \"%s\";",
			WrapValue(t.userName),
			WrapValue(host),
			t.newPassword,
		)
	}

	return strings.Join(results, " ") + " FLUSH PRIVILEGES;"
}

// UserName returns the username being updated
func (t *UpdateUserPassword) UserName() string {
	return t.userName
}

// NewPassword returns the new password
func (t *UpdateUserPassword) NewPassword() string {
	return t.newPassword
}

// onFinished handles successful completion
func (t *UpdateUserPassword) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// Password updated successfully
}

// onFailed handles task failure
func (t *UpdateUserPassword) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	// Handle password update failure
}

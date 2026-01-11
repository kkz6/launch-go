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

// RevokeAllPrivileges revokes all privileges on a database from a MySQL user
type RevokeAllPrivileges struct {
	tasks.BaseServerTask
	hosts         []string
	mySQLUser     string
	mySQLPassword string
	userName      string
	database      string
}

// NewRevokeAllPrivileges creates a new RevokeAllPrivileges task
func NewRevokeAllPrivileges(server *models.Server, userName, database string) *RevokeAllPrivileges {
	task := &RevokeAllPrivileges{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/mysql/revoke-all-privileges",
				TaskTimeout:  2 * time.Minute,
			},
		},
		hosts:    []string{"%"},
		userName: userName,
		database: database,
	}
	task.SetServer(server)

	task.FinishedCallback = task.onFinished
	task.FailedCallback = task.onFailed

	return task
}

// WithCredentials sets the MySQL connection credentials
func (t *RevokeAllPrivileges) WithCredentials(user, password string) *RevokeAllPrivileges {
	t.mySQLUser = user
	t.mySQLPassword = password

	return t
}

// OnServer sets the credentials using the server's database credentials
func (t *RevokeAllPrivileges) OnServer(server *models.Server) *RevokeAllPrivileges {
	t.hosts = GetHostsForServer(server)
	t.mySQLUser = "root"
	t.mySQLPassword = GetServerDatabasePassword(server)

	return t
}

// Data returns the template data
func (t *RevokeAllPrivileges) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":        t.Server(),
		"MySQLUser":     t.mySQLUser,
		"MySQLPassword": t.mySQLPassword,
		"SQL":           t.SQL(),
	}
}

// SQL returns the SQL query to revoke privileges
func (t *RevokeAllPrivileges) SQL() string {
	results := make([]string, len(t.hosts))
	for i, host := range t.hosts {
		results[i] = fmt.Sprintf(
			"REVOKE ALL ON %s.* FROM %s@%s;",
			WrapValue(t.database),
			WrapValue(t.userName),
			WrapValue(host),
		)
	}

	return strings.Join(results, " ") + " FLUSH PRIVILEGES;"
}

// UserName returns the username losing privileges
func (t *RevokeAllPrivileges) UserName() string {
	return t.userName
}

// Database returns the database name
func (t *RevokeAllPrivileges) Database() string {
	return t.database
}

// onFinished handles successful completion
func (t *RevokeAllPrivileges) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// Privileges revoked successfully
}

// onFailed handles task failure
func (t *RevokeAllPrivileges) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	// Handle revoke privileges failure
}

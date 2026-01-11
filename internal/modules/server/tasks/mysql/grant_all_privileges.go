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

// GrantAllPrivileges grants all privileges on a database to a MySQL user
type GrantAllPrivileges struct {
	tasks.BaseServerTask
	hosts         []string
	mySQLUser     string
	mySQLPassword string
	userName      string
	database      string
}

// NewGrantAllPrivileges creates a new GrantAllPrivileges task
func NewGrantAllPrivileges(server *models.Server, userName, database string) *GrantAllPrivileges {
	task := &GrantAllPrivileges{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/mysql/grant-all-privileges",
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
func (t *GrantAllPrivileges) WithCredentials(user, password string) *GrantAllPrivileges {
	t.mySQLUser = user
	t.mySQLPassword = password

	return t
}

// OnServer sets the credentials using the server's database credentials
func (t *GrantAllPrivileges) OnServer(server *models.Server) *GrantAllPrivileges {
	t.hosts = GetHostsForServer(server)
	t.mySQLUser = "root"
	t.mySQLPassword = GetServerDatabasePassword(server)

	return t
}

// Data returns the template data
func (t *GrantAllPrivileges) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":        t.Server(),
		"MySQLUser":     t.mySQLUser,
		"MySQLPassword": t.mySQLPassword,
		"SQL":           t.SQL(),
	}
}

// SQL returns the SQL query to grant privileges
func (t *GrantAllPrivileges) SQL() string {
	results := make([]string, len(t.hosts))
	for i, host := range t.hosts {
		results[i] = fmt.Sprintf(
			"GRANT ALL ON %s.* TO %s@%s WITH GRANT OPTION;",
			WrapValue(t.database),
			WrapValue(t.userName),
			WrapValue(host),
		)
	}

	return strings.Join(results, " ") + " FLUSH PRIVILEGES;"
}

// UserName returns the username receiving privileges
func (t *GrantAllPrivileges) UserName() string {
	return t.userName
}

// Database returns the database name
func (t *GrantAllPrivileges) Database() string {
	return t.database
}

// onFinished handles successful completion
func (t *GrantAllPrivileges) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// Privileges granted successfully
}

// onFailed handles task failure
func (t *GrantAllPrivileges) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	// Handle grant privileges failure
}

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

// CreateUser creates a new MySQL user
type CreateUser struct {
	tasks.BaseServerTask
	hosts         []string
	mySQLUser     string
	mySQLPassword string
	userName      string
	userPassword  string
}

// NewCreateUser creates a new CreateUser task
func NewCreateUser(server *models.Server, userName, userPassword string) *CreateUser {
	task := &CreateUser{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/mysql/create-user",
				TaskTimeout:  2 * time.Minute,
			},
		},
		hosts:        []string{"%"},
		userName:     userName,
		userPassword: userPassword,
	}
	task.SetServer(server)

	task.FinishedCallback = task.onFinished
	task.FailedCallback = task.onFailed

	return task
}

// WithCredentials sets the MySQL connection credentials
func (t *CreateUser) WithCredentials(user, password string) *CreateUser {
	t.mySQLUser = user
	t.mySQLPassword = password

	return t
}

// OnServer sets the credentials using the server's database credentials
func (t *CreateUser) OnServer(server *models.Server) *CreateUser {
	t.hosts = GetHostsForServer(server)
	t.mySQLUser = "root"
	t.mySQLPassword = GetServerDatabasePassword(server)

	return t
}

// Data returns the template data
func (t *CreateUser) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":        t.Server(),
		"MySQLUser":     t.mySQLUser,
		"MySQLPassword": t.mySQLPassword,
		"SQL":           t.SQL(),
	}
}

// SQL returns the SQL query to create the user
func (t *CreateUser) SQL() string {
	results := make([]string, len(t.hosts))
	for i, host := range t.hosts {
		results[i] = fmt.Sprintf(
			"CREATE USER IF NOT EXISTS %s@%s IDENTIFIED BY \"%s\";",
			WrapValue(t.userName),
			WrapValue(host),
			t.userPassword,
		)
	}

	return strings.Join(results, " ")
}

// UserName returns the username being created
func (t *CreateUser) UserName() string {
	return t.userName
}

// UserPassword returns the password for the new user
func (t *CreateUser) UserPassword() string {
	return t.userPassword
}

// onFinished handles successful completion
func (t *CreateUser) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// User created successfully
}

// onFailed handles task failure
func (t *CreateUser) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	// Handle user creation failure
}

package postgresql

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// CreateUser creates a new PostgreSQL user
type CreateUser struct {
	tasks.BaseServerTask
	pgUser     string
	pgPassword string
	name       string
	password   string
}

// NewCreateUser creates a new CreateUser task
func NewCreateUser(server *models.Server, name, password string) *CreateUser {
	task := &CreateUser{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/postgresql/create-user",
				TaskTimeout:  2 * time.Minute,
			},
		},
		name:     name,
		password: password,
	}

	task.SetServer(server)

	return task
}

// WithCredentials sets the PostgreSQL admin credentials
func (t *CreateUser) WithCredentials(user, password string) *CreateUser {
	t.pgUser = user
	t.pgPassword = password

	return t
}

// OnServer sets credentials from server's database password
func (t *CreateUser) OnServer(server *models.Server) *CreateUser {
	password := ""
	if server.DatabasePassword != nil {
		password = *server.DatabasePassword
	}

	return t.WithCredentials("launch", password)
}

// SQL returns the SQL query to create the user
func (t *CreateUser) SQL() string {
	return fmt.Sprintf(
		"CREATE USER %s WITH PASSWORD %s",
		QuoteIdentifier(t.name),
		QuoteValue(t.password),
	)
}

// Data returns the template data
func (t *CreateUser) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":     t.Server(),
		"PgUser":     t.pgUser,
		"PgPassword": t.pgPassword,
		"PgDatabase": "postgres",
		"Name":       t.name,
		"Password":   t.password,
		"SQL":        t.SQL(),
	}
}

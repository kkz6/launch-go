package postgresql

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// UpdateUserPassword updates the password for a PostgreSQL user
type UpdateUserPassword struct {
	tasks.BaseServerTask
	pgUser     string
	pgPassword string
	name       string
	password   string
}

// NewUpdateUserPassword creates a new UpdateUserPassword task
func NewUpdateUserPassword(server *models.Server, name, password string) *UpdateUserPassword {
	task := &UpdateUserPassword{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/postgresql/update-user-password",
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
func (t *UpdateUserPassword) WithCredentials(user, password string) *UpdateUserPassword {
	t.pgUser = user
	t.pgPassword = password

	return t
}

// OnServer sets credentials from server's database password
func (t *UpdateUserPassword) OnServer(server *models.Server) *UpdateUserPassword {
	password := ""
	if server.DatabasePassword != nil {
		password = *server.DatabasePassword
	}

	return t.WithCredentials("launch", password)
}

// SQL returns the SQL query to update the user password
func (t *UpdateUserPassword) SQL() string {
	return fmt.Sprintf(
		"ALTER USER %s WITH PASSWORD %s",
		QuoteIdentifier(t.name),
		QuoteValue(t.password),
	)
}

// Data returns the template data
func (t *UpdateUserPassword) Data() map[string]interface{} {
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

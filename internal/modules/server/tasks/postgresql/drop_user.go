package postgresql

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// DropUser drops a PostgreSQL user
type DropUser struct {
	tasks.BaseServerTask
	pgUser     string
	pgPassword string
	user       string
}

// NewDropUser creates a new DropUser task
func NewDropUser(server *models.Server, user string) *DropUser {
	task := &DropUser{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/postgresql/drop-user",
				TaskTimeout:  2 * time.Minute,
			},
		},
		user: user,
	}

	task.SetServer(server)

	return task
}

// WithCredentials sets the PostgreSQL admin credentials
func (t *DropUser) WithCredentials(user, password string) *DropUser {
	t.pgUser = user
	t.pgPassword = password

	return t
}

// OnServer sets credentials from server's database password
func (t *DropUser) OnServer(server *models.Server) *DropUser {
	password := ""
	if server.DatabasePassword != nil {
		password = *server.DatabasePassword
	}

	return t.WithCredentials("launch", password)
}

// SQL returns the SQL query to drop the user
func (t *DropUser) SQL() string {
	return fmt.Sprintf(
		"DROP USER IF EXISTS %s",
		QuoteIdentifier(t.user),
	)
}

// Data returns the template data
func (t *DropUser) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":     t.Server(),
		"PgUser":     t.pgUser,
		"PgPassword": t.pgPassword,
		"PgDatabase": "postgres",
		"User":       t.user,
		"SQL":        t.SQL(),
	}
}

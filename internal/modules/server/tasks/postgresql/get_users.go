package postgresql

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// GetUsers retrieves a list of all PostgreSQL users
type GetUsers struct {
	tasks.BaseServerTask
	pgUser     string
	pgPassword string
}

// NewGetUsers creates a new GetUsers task
func NewGetUsers(server *models.Server) *GetUsers {
	task := &GetUsers{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/postgresql/get-users",
				TaskTimeout:  1 * time.Minute,
			},
		},
	}

	task.SetServer(server)

	return task
}

// WithCredentials sets the PostgreSQL admin credentials
func (t *GetUsers) WithCredentials(user, password string) *GetUsers {
	t.pgUser = user
	t.pgPassword = password

	return t
}

// OnServer sets credentials from server's database password
func (t *GetUsers) OnServer(server *models.Server) *GetUsers {
	password := ""
	if server.DatabasePassword != nil {
		password = *server.DatabasePassword
	}

	return t.WithCredentials("launch", password)
}

// SQL returns the SQL query to get all users
func (t *GetUsers) SQL() string {
	return "SELECT usename FROM pg_user"
}

// Data returns the template data
func (t *GetUsers) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":     t.Server(),
		"PgUser":     t.pgUser,
		"PgPassword": t.pgPassword,
		"PgDatabase": "postgres",
		"SQL":        t.SQL(),
	}
}

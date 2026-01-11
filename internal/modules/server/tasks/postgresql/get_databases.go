package postgresql

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// GetDatabases retrieves a list of all PostgreSQL databases
type GetDatabases struct {
	tasks.BaseServerTask
	pgUser     string
	pgPassword string
}

// NewGetDatabases creates a new GetDatabases task
func NewGetDatabases(server *models.Server) *GetDatabases {
	task := &GetDatabases{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/postgresql/get-databases",
				TaskTimeout:  1 * time.Minute,
			},
		},
	}

	task.SetServer(server)

	return task
}

// WithCredentials sets the PostgreSQL admin credentials
func (t *GetDatabases) WithCredentials(user, password string) *GetDatabases {
	t.pgUser = user
	t.pgPassword = password

	return t
}

// OnServer sets credentials from server's database password
func (t *GetDatabases) OnServer(server *models.Server) *GetDatabases {
	password := ""
	if server.DatabasePassword != nil {
		password = *server.DatabasePassword
	}

	return t.WithCredentials("launch", password)
}

// SQL returns the SQL query to get all databases
func (t *GetDatabases) SQL() string {
	return "SELECT datname FROM pg_database WHERE datistemplate = false"
}

// Data returns the template data
func (t *GetDatabases) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":     t.Server(),
		"PgUser":     t.pgUser,
		"PgPassword": t.pgPassword,
		"PgDatabase": "postgres",
		"SQL":        t.SQL(),
	}
}

package postgresql

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// GetTables retrieves a list of all tables in a PostgreSQL database
type GetTables struct {
	tasks.BaseServerTask
	pgUser     string
	pgPassword string
	database   string
}

// NewGetTables creates a new GetTables task
func NewGetTables(server *models.Server, database string) *GetTables {
	task := &GetTables{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/postgresql/get-tables",
				TaskTimeout:  1 * time.Minute,
			},
		},
		database: database,
	}

	task.SetServer(server)

	return task
}

// WithCredentials sets the PostgreSQL admin credentials
func (t *GetTables) WithCredentials(user, password string) *GetTables {
	t.pgUser = user
	t.pgPassword = password

	return t
}

// OnServer sets credentials from server's database password
func (t *GetTables) OnServer(server *models.Server) *GetTables {
	password := ""
	if server.DatabasePassword != nil {
		password = *server.DatabasePassword
	}

	return t.WithCredentials("launch", password)
}

// SQL returns the SQL query to get all tables in public schema
func (t *GetTables) SQL() string {
	return "SELECT tablename FROM pg_tables WHERE schemaname = 'public'"
}

// Data returns the template data
func (t *GetTables) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":     t.Server(),
		"PgUser":     t.pgUser,
		"PgPassword": t.pgPassword,
		"PgDatabase": t.database,
		"Database":   t.database,
		"SQL":        t.SQL(),
	}
}

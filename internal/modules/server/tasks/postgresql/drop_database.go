package postgresql

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// DropDatabase drops a PostgreSQL database
type DropDatabase struct {
	tasks.BaseServerTask
	pgUser     string
	pgPassword string
	name       string
}

// NewDropDatabase creates a new DropDatabase task
func NewDropDatabase(server *models.Server, name string) *DropDatabase {
	task := &DropDatabase{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/postgresql/drop-database",
				TaskTimeout:  2 * time.Minute,
			},
		},
		name: name,
	}

	task.SetServer(server)

	return task
}

// WithCredentials sets the PostgreSQL admin credentials
func (t *DropDatabase) WithCredentials(user, password string) *DropDatabase {
	t.pgUser = user
	t.pgPassword = password

	return t
}

// OnServer sets credentials from server's database password
func (t *DropDatabase) OnServer(server *models.Server) *DropDatabase {
	password := ""
	if server.DatabasePassword != nil {
		password = *server.DatabasePassword
	}

	return t.WithCredentials("launch", password)
}

// SQL returns the SQL query to drop the database
func (t *DropDatabase) SQL() string {
	return fmt.Sprintf(
		"DROP DATABASE IF EXISTS %s",
		QuoteIdentifier(t.name),
	)
}

// Data returns the template data
func (t *DropDatabase) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":     t.Server(),
		"PgUser":     t.pgUser,
		"PgPassword": t.pgPassword,
		"PgDatabase": "postgres",
		"Name":       t.name,
		"SQL":        t.SQL(),
	}
}

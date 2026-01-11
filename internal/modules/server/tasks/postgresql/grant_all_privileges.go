package postgresql

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// GrantAllPrivileges grants all privileges on a database to a user
type GrantAllPrivileges struct {
	tasks.BaseServerTask
	pgUser     string
	pgPassword string
	name       string
	database   string
}

// NewGrantAllPrivileges creates a new GrantAllPrivileges task
func NewGrantAllPrivileges(server *models.Server, name, database string) *GrantAllPrivileges {
	task := &GrantAllPrivileges{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/postgresql/grant-all-privileges",
				TaskTimeout:  2 * time.Minute,
			},
		},
		name:     name,
		database: database,
	}

	task.SetServer(server)

	return task
}

// WithCredentials sets the PostgreSQL admin credentials
func (t *GrantAllPrivileges) WithCredentials(user, password string) *GrantAllPrivileges {
	t.pgUser = user
	t.pgPassword = password

	return t
}

// OnServer sets credentials from server's database password
func (t *GrantAllPrivileges) OnServer(server *models.Server) *GrantAllPrivileges {
	password := ""
	if server.DatabasePassword != nil {
		password = *server.DatabasePassword
	}

	return t.WithCredentials("launch", password)
}

// SQL returns the SQL query to grant database privileges
func (t *GrantAllPrivileges) SQL() string {
	return fmt.Sprintf(
		"GRANT ALL PRIVILEGES ON DATABASE %s TO %s",
		QuoteIdentifier(t.database),
		QuoteIdentifier(t.name),
	)
}

// Commands returns all the commands needed to grant full privileges
func (t *GrantAllPrivileges) Commands() []string {
	dbIdentifier := QuoteIdentifier(t.database)
	userIdentifier := QuoteIdentifier(t.name)

	return []string{
		fmt.Sprintf(
			`PGPASSWORD=%s psql -U %s -d postgres -c "GRANT ALL PRIVILEGES ON DATABASE %s TO %s"`,
			t.pgPassword, t.pgUser, dbIdentifier, userIdentifier,
		),
		fmt.Sprintf(
			`PGPASSWORD=%s psql -U %s -d %s -c "GRANT ALL PRIVILEGES ON SCHEMA public TO %s"`,
			t.pgPassword, t.pgUser, t.database, userIdentifier,
		),
		fmt.Sprintf(
			`PGPASSWORD=%s psql -U %s -d %s -c "GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO %s"`,
			t.pgPassword, t.pgUser, t.database, userIdentifier,
		),
		fmt.Sprintf(
			`PGPASSWORD=%s psql -U %s -d %s -c "GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO %s"`,
			t.pgPassword, t.pgUser, t.database, userIdentifier,
		),
		fmt.Sprintf(
			`PGPASSWORD=%s psql -U %s -d %s -c "ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL PRIVILEGES ON TABLES TO %s"`,
			t.pgPassword, t.pgUser, t.database, userIdentifier,
		),
		fmt.Sprintf(
			`PGPASSWORD=%s psql -U %s -d %s -c "ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL PRIVILEGES ON SEQUENCES TO %s"`,
			t.pgPassword, t.pgUser, t.database, userIdentifier,
		),
	}
}

// Data returns the template data
func (t *GrantAllPrivileges) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":         t.Server(),
		"PgUser":         t.pgUser,
		"PgPassword":     t.pgPassword,
		"Name":           t.name,
		"Database":       t.database,
		"UserIdentifier": QuoteIdentifier(t.name),
		"DbIdentifier":   QuoteIdentifier(t.database),
		"Commands":       t.Commands(),
		"SQL":            t.SQL(),
	}
}

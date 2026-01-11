package postgresql

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// RevokeAllPrivileges revokes all privileges on a database from a user
type RevokeAllPrivileges struct {
	tasks.BaseServerTask
	pgUser     string
	pgPassword string
	name       string
	database   string
}

// NewRevokeAllPrivileges creates a new RevokeAllPrivileges task
func NewRevokeAllPrivileges(server *models.Server, name, database string) *RevokeAllPrivileges {
	task := &RevokeAllPrivileges{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/postgresql/revoke-all-privileges",
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
func (t *RevokeAllPrivileges) WithCredentials(user, password string) *RevokeAllPrivileges {
	t.pgUser = user
	t.pgPassword = password

	return t
}

// OnServer sets credentials from server's database password
func (t *RevokeAllPrivileges) OnServer(server *models.Server) *RevokeAllPrivileges {
	password := ""
	if server.DatabasePassword != nil {
		password = *server.DatabasePassword
	}

	return t.WithCredentials("launch", password)
}

// SQL returns the SQL query to revoke database privileges
func (t *RevokeAllPrivileges) SQL() string {
	return fmt.Sprintf(
		"REVOKE ALL PRIVILEGES ON DATABASE %s FROM %s",
		QuoteIdentifier(t.database),
		QuoteIdentifier(t.name),
	)
}

// Commands returns all the commands needed to revoke full privileges
func (t *RevokeAllPrivileges) Commands() []string {
	dbIdentifier := QuoteIdentifier(t.database)
	userIdentifier := QuoteIdentifier(t.name)

	return []string{
		fmt.Sprintf(
			`PGPASSWORD=%s psql -U %s -d %s -c "REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM %s"`,
			t.pgPassword, t.pgUser, t.database, userIdentifier,
		),
		fmt.Sprintf(
			`PGPASSWORD=%s psql -U %s -d %s -c "REVOKE ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public FROM %s"`,
			t.pgPassword, t.pgUser, t.database, userIdentifier,
		),
		fmt.Sprintf(
			`PGPASSWORD=%s psql -U %s -d %s -c "REVOKE ALL PRIVILEGES ON SCHEMA public FROM %s"`,
			t.pgPassword, t.pgUser, t.database, userIdentifier,
		),
		fmt.Sprintf(
			`PGPASSWORD=%s psql -U %s -d postgres -c "REVOKE ALL PRIVILEGES ON DATABASE %s FROM %s"`,
			t.pgPassword, t.pgUser, dbIdentifier, userIdentifier,
		),
	}
}

// Data returns the template data
func (t *RevokeAllPrivileges) Data() map[string]interface{} {
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

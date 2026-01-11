package postgresql

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// PostgreSQLTask is the base task for PostgreSQL operations
type PostgreSQLTask struct {
	tasks.BaseServerTask
	pgUser     string
	pgPassword string
	pgDatabase string
}

// NewPostgreSQLTask creates a new PostgreSQL base task
func NewPostgreSQLTask(server *models.Server) *PostgreSQLTask {
	return &PostgreSQLTask{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TaskTimeout: 2 * time.Minute,
			},
		},
	}
}

// WithCredentials sets the credentials to use for the PostgreSQL connection
func (t *PostgreSQLTask) WithCredentials(user, password string) *PostgreSQLTask {
	t.pgUser = user
	t.pgPassword = password

	return t
}

// WithDatabase sets the database to connect to
func (t *PostgreSQLTask) WithDatabase(database string) *PostgreSQLTask {
	t.pgDatabase = database

	return t
}

// OnServer sets the credentials to the server's database credentials
func (t *PostgreSQLTask) OnServer(server *models.Server) *PostgreSQLTask {
	password := ""
	if server.DatabasePassword != nil {
		password = *server.DatabasePassword
	}

	return t.WithCredentials("launch", password)
}

// GetPgUser returns the PostgreSQL user
func (t *PostgreSQLTask) GetPgUser() string {
	return t.pgUser
}

// GetPgPassword returns the PostgreSQL password
func (t *PostgreSQLTask) GetPgPassword() string {
	return t.pgPassword
}

// GetPgDatabase returns the PostgreSQL database (defaults to "postgres")
func (t *PostgreSQLTask) GetPgDatabase() string {
	if t.pgDatabase == "" {
		return "postgres"
	}

	return t.pgDatabase
}

// QuoteIdentifier quotes an identifier (database, table, user name) for PostgreSQL
func QuoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

// QuoteValue quotes a string value for PostgreSQL
func QuoteValue(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

// BuildPsqlCommand builds a psql command with credentials
func (t *PostgreSQLTask) BuildPsqlCommand(sql string) (string, error) {
	if t.pgUser == "" || t.pgPassword == "" {
		return "", errors.New("PostgreSQL user or password not set")
	}

	database := t.GetPgDatabase()

	return fmt.Sprintf(
		`PGPASSWORD=%s psql -U %s -d %s -t -A -c "%s"`,
		t.pgPassword,
		t.pgUser,
		database,
		sql,
	), nil
}

// BuildMultiPsqlCommands builds multiple psql commands joined with &&
func (t *PostgreSQLTask) BuildMultiPsqlCommands(commands []string) string {
	return strings.Join(commands, " && ")
}

package postgresql

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// CreateDatabase creates a new PostgreSQL database
type CreateDatabase struct {
	tasks.BaseServerTask
	pgUser     string
	pgPassword string
	name       string
	encoding   string
	locale     string
}

// NewCreateDatabase creates a new CreateDatabase task
func NewCreateDatabase(server *models.Server, name string) *CreateDatabase {
	task := &CreateDatabase{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/postgresql/create-database",
				TaskTimeout:  2 * time.Minute,
			},
		},
		name:     name,
		encoding: "UTF8",
		locale:   "en_US.UTF-8",
	}

	task.SetServer(server)

	return task
}

// WithEncoding sets the database encoding
func (t *CreateDatabase) WithEncoding(encoding string) *CreateDatabase {
	t.encoding = encoding

	return t
}

// WithLocale sets the database locale
func (t *CreateDatabase) WithLocale(locale string) *CreateDatabase {
	t.locale = locale

	return t
}

// WithCredentials sets the PostgreSQL credentials
func (t *CreateDatabase) WithCredentials(user, password string) *CreateDatabase {
	t.pgUser = user
	t.pgPassword = password

	return t
}

// OnServer sets credentials from server's database password
func (t *CreateDatabase) OnServer(server *models.Server) *CreateDatabase {
	password := ""
	if server.DatabasePassword != nil {
		password = *server.DatabasePassword
	}

	return t.WithCredentials("launch", password)
}

// SQL returns the SQL query to create the database
func (t *CreateDatabase) SQL() string {
	return fmt.Sprintf(
		"CREATE DATABASE %s ENCODING %s LC_COLLATE %s LC_CTYPE %s TEMPLATE template0",
		QuoteIdentifier(t.name),
		QuoteValue(t.encoding),
		QuoteValue(t.locale),
		QuoteValue(t.locale),
	)
}

// Data returns the template data
func (t *CreateDatabase) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":     t.Server(),
		"PgUser":     t.pgUser,
		"PgPassword": t.pgPassword,
		"PgDatabase": "postgres",
		"Name":       t.name,
		"Encoding":   t.encoding,
		"Locale":     t.locale,
		"SQL":        t.SQL(),
	}
}

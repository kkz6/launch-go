package services

import (
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// RemovePostgreSql removes PostgreSQL from the server
type RemovePostgreSql struct {
	tasks.BaseServerTask
}

// NewRemovePostgreSql creates a new RemovePostgreSql task
func NewRemovePostgreSql(server *models.Server) *RemovePostgreSql {
	task := &RemovePostgreSql{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/services/remove-postgresql",
				TaskTimeout:  2 * time.Minute,
			},
		},
	}
	task.SetServer(server)

	return task
}

// Data returns the template data
func (t *RemovePostgreSql) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server": t.Server(),
	}
}

// BuildScript generates the shell script for removing PostgreSQL
func (t *RemovePostgreSql) BuildScript() string {
	commands := []string{
		`echo "Stopping PostgreSQL service..."`,
		"sudo systemctl stop postgresql 2>/dev/null || sudo service postgresql stop 2>/dev/null || true",
		`echo "Removing PostgreSQL packages..."`,
		"sudo DEBIAN_FRONTEND=noninteractive apt-get purge -y postgresql postgresql-* libpq-dev",
		"sudo DEBIAN_FRONTEND=noninteractive apt-get autoremove -y",
		"sudo DEBIAN_FRONTEND=noninteractive apt-get autoclean",
		`echo "Cleaning up PostgreSQL data directories..."`,
		"sudo rm -rf /var/lib/postgresql",
		"sudo rm -rf /var/log/postgresql",
		"sudo rm -rf /etc/postgresql",
		`echo "PostgreSQL removed successfully"`,
	}

	return strings.Join(commands, " && ")
}

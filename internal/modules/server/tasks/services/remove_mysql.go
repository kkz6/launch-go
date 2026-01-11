package services

import (
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// RemoveMySql removes MySQL from the server
type RemoveMySql struct {
	tasks.BaseServerTask
}

// NewRemoveMySql creates a new RemoveMySql task
func NewRemoveMySql(server *models.Server) *RemoveMySql {
	task := &RemoveMySql{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/services/remove-mysql",
				TaskTimeout:  2 * time.Minute,
			},
		},
	}
	task.SetServer(server)

	return task
}

// Data returns the template data
func (t *RemoveMySql) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server": t.Server(),
	}
}

// BuildScript generates the shell script for removing MySQL
func (t *RemoveMySql) BuildScript() string {
	commands := []string{
		`echo "Stopping MySQL service..."`,
		"sudo systemctl stop mysql 2>/dev/null || sudo service mysql stop 2>/dev/null || true",
		`echo "Removing MySQL packages..."`,
		"sudo DEBIAN_FRONTEND=noninteractive apt-get purge -y mysql-server mysql-client mysql-common mysql-server-core-* mysql-client-core-*",
		"sudo DEBIAN_FRONTEND=noninteractive apt-get autoremove -y",
		"sudo DEBIAN_FRONTEND=noninteractive apt-get autoclean",
		`echo "Cleaning up MySQL data directories..."`,
		"sudo rm -rf /var/lib/mysql",
		"sudo rm -rf /var/log/mysql",
		"sudo rm -rf /etc/mysql",
		"sudo rm -f /var/log/mysql.*",
		`echo "MySQL removed successfully"`,
	}

	return strings.Join(commands, " && ")
}

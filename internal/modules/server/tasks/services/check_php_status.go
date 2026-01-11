package services

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// CheckPhpStatus checks the status of PHP-FPM
type CheckPhpStatus struct {
	tasks.BaseServerTask
	version string
}

// NewCheckPhpStatus creates a new CheckPhpStatus task
func NewCheckPhpStatus(server *models.Server, version string) *CheckPhpStatus {
	if version == "" {
		version = "8.0"
	}

	task := &CheckPhpStatus{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/services/check-php-status",
				TaskTimeout:  30 * time.Second,
			},
		},
		version: version,
	}
	task.SetServer(server)

	return task
}

// Data returns the template data
func (t *CheckPhpStatus) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":     t.Server(),
		"PhpVersion": t.version,
	}
}

// Version returns the PHP version
func (t *CheckPhpStatus) Version() string {
	return t.version
}

// BuildScript generates the shell script for checking PHP status
func (t *CheckPhpStatus) BuildScript() string {
	return fmt.Sprintf(
		`sudo systemctl status php%s-fpm --no-pager --full && echo "===PROCESSES===" && ps aux | grep [p]hp%s-fpm | head -10 && echo "===CONNECTIONS===" && sudo ss -tulpn | grep php-fpm && echo "===MEMORY===" && sudo systemctl show php%s-fpm --property=MemoryCurrent,MemoryPeak && echo "===FPM_STATUS===" && sudo php%s -v && echo "===POOL_CONFIG===" && sudo ls -la /etc/php/%s/fpm/pool.d/ 2>/dev/null || echo "Pool config not accessible"`,
		t.version, t.version, t.version, t.version, t.version,
	)
}

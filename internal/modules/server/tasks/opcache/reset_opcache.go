package opcache

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// ResetOpcache resets the OPcache on a server
type ResetOpcache struct {
	tasks.BaseServerTask
	service *models.InstalledService
}

// NewResetOpcache creates a new ResetOpcache task
func NewResetOpcache(server *models.Server, service *models.InstalledService) *ResetOpcache {
	task := &ResetOpcache{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/opcache/reset",
				TaskTimeout:  30 * time.Second,
			},
		},
		service: service,
	}

	task.SetServer(server)

	return task
}

// Data returns the template data
func (t *ResetOpcache) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":  t.Server(),
		"Service": t.service,
	}
}

// Service returns the installed service
func (t *ResetOpcache) Service() *models.InstalledService {
	return t.service
}

package opcache

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// GetOpcacheStatus retrieves OPcache status from a server
type GetOpcacheStatus struct {
	tasks.BaseServerTask
	service *models.InstalledService
}

// NewGetOpcacheStatus creates a new GetOpcacheStatus task
func NewGetOpcacheStatus(server *models.Server, service *models.InstalledService) *GetOpcacheStatus {
	task := &GetOpcacheStatus{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/opcache/status",
				TaskTimeout:  30 * time.Second,
			},
		},
		service: service,
	}

	task.SetServer(server)

	return task
}

// Data returns the template data
func (t *GetOpcacheStatus) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":  t.Server(),
		"Service": t.service,
	}
}

// Service returns the installed service
func (t *GetOpcacheStatus) Service() *models.InstalledService {
	return t.service
}

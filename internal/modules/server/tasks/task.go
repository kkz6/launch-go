package tasks

import (
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// ServerTask extends the base Task interface with server-specific methods
type ServerTask interface {
	taskrunner.Task
	Server() *models.Server
}

// BaseServerTask embeds taskrunner.BaseTask and adds server reference
type BaseServerTask struct {
	taskrunner.BaseTask
	server *models.Server
}

func (t *BaseServerTask) Server() *models.Server {
	return t.server
}

// SetServer sets the server reference
func (t *BaseServerTask) SetServer(server *models.Server) {
	t.server = server
}

// GetMemoryMB safely gets server memory with default fallback
func (t *BaseServerTask) GetMemoryMB() int {
	if t.server != nil && t.server.MemoryInMB != nil {
		return *t.server.MemoryInMB
	}
	return 1024 // Default to 1GB
}

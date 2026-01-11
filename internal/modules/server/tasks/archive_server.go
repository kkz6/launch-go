package tasks

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// ArchiveServer revokes Launch public key access from a server
type ArchiveServer struct {
	BaseServerTask
}

// NewArchiveServer creates a new ArchiveServer task
func NewArchiveServer(server *models.Server) *ArchiveServer {
	task := &ArchiveServer{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TaskName:     "archive-server",
				TemplateName: "server/archive-server",
				TaskTimeout:  2 * time.Minute,
			},
			server: server,
		},
	}

	return task
}

// Data returns the template data
func (t *ArchiveServer) Data() map[string]interface{} {
	publicKey := ""
	if t.server.PublicKey != nil {
		publicKey = *t.server.PublicKey
	}

	return map[string]interface{}{
		"Server":    t.server,
		"Username":  t.server.GetUsername(),
		"PublicKey": publicKey,
	}
}

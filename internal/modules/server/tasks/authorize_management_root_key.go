package tasks

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// AuthorizeManagementRootKey authorizes the management root key on a server
type AuthorizeManagementRootKey struct {
	BaseServerTask
}

// NewAuthorizeManagementRootKey creates a new AuthorizeManagementRootKey task
func NewAuthorizeManagementRootKey(server *models.Server) *AuthorizeManagementRootKey {
	task := &AuthorizeManagementRootKey{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/ssh/authorize-management-root-key",
				TaskTimeout:  15 * time.Second,
			},
			server: server,
		},
	}

	return task
}

// Data returns the template data
func (t *AuthorizeManagementRootKey) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server": t.server,
	}
}

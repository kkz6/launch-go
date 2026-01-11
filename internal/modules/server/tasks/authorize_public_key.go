package tasks

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// AuthorizePublicKey authorizes a public key on a server
type AuthorizePublicKey struct {
	BaseServerTask
	publicKey string
	root      bool
}

// NewAuthorizePublicKey creates a new AuthorizePublicKey task
func NewAuthorizePublicKey(server *models.Server, publicKey string, root bool) *AuthorizePublicKey {
	task := &AuthorizePublicKey{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/ssh/authorize-public-key",
				TaskTimeout:  15 * time.Second,
			},
			server: server,
		},
		publicKey: publicKey,
		root:      root,
	}

	return task
}

// Data returns the template data
func (t *AuthorizePublicKey) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":    t.server,
		"PublicKey": t.publicKey,
		"Root":      t.root,
		"Username":  t.getUsername(),
	}
}

// PublicKey returns the public key being authorized
func (t *AuthorizePublicKey) PublicKey() string {
	return t.publicKey
}

// IsRoot returns whether this is for root user
func (t *AuthorizePublicKey) IsRoot() bool {
	return t.root
}

// getUsername returns the target username based on root flag
func (t *AuthorizePublicKey) getUsername() string {
	if t.root {
		return "root"
	}

	return t.server.GetUsername()
}

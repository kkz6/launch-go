package tasks

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// DeauthorizePublicKey removes a public key from a server
type DeauthorizePublicKey struct {
	BaseServerTask
	publicKey string
	root      bool
}

// NewDeauthorizePublicKey creates a new DeauthorizePublicKey task
func NewDeauthorizePublicKey(server *models.Server, publicKey string, root bool) *DeauthorizePublicKey {
	task := &DeauthorizePublicKey{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/ssh/deauthorize-public-key",
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
func (t *DeauthorizePublicKey) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":    t.server,
		"PublicKey": t.publicKey,
		"Root":      t.root,
		"Username":  t.getUsername(),
	}
}

// PublicKey returns the public key being deauthorized
func (t *DeauthorizePublicKey) PublicKey() string {
	return t.publicKey
}

// IsRoot returns whether this is for root user
func (t *DeauthorizePublicKey) IsRoot() bool {
	return t.root
}

// getUsername returns the target username based on root flag
func (t *DeauthorizePublicKey) getUsername() string {
	if t.root {
		return "root"
	}

	return t.server.GetUsername()
}

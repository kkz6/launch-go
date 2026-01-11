package tasks

import (
	"time"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// GeneratePublicKey generates a public key from a private key
type GeneratePublicKey struct {
	taskrunner.BaseTask
	privatePath string
	publicPath  string
}

// NewGeneratePublicKey creates a new GeneratePublicKey task
func NewGeneratePublicKey(privatePath string, publicPath string) *GeneratePublicKey {
	task := &GeneratePublicKey{
		BaseTask: taskrunner.BaseTask{
			TemplateName: "server/ssh/generate-public-key",
			TaskTimeout:  10 * time.Minute,
		},
		privatePath: privatePath,
		publicPath:  publicPath,
	}

	return task
}

// Data returns the template data
func (t *GeneratePublicKey) Data() map[string]interface{} {
	return map[string]interface{}{
		"PrivatePath": t.privatePath,
		"PublicPath":  t.publicPath,
	}
}

// PrivatePath returns the private key path
func (t *GeneratePublicKey) PrivatePath() string {
	return t.privatePath
}

// PublicPath returns the public key path
func (t *GeneratePublicKey) PublicPath() string {
	return t.publicPath
}

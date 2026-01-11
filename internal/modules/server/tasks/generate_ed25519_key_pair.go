package tasks

import (
	"time"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// DefaultSSHComment is the default comment for generated SSH keys
const DefaultSSHComment = "launch@gigcodes.com"

// GenerateEd25519KeyPair generates an Ed25519 SSH key pair
type GenerateEd25519KeyPair struct {
	taskrunner.BaseTask
	privatePath string
	comment     string
}

// NewGenerateEd25519KeyPair creates a new GenerateEd25519KeyPair task
func NewGenerateEd25519KeyPair(privatePath string) *GenerateEd25519KeyPair {
	task := &GenerateEd25519KeyPair{
		BaseTask: taskrunner.BaseTask{
			TemplateName: "server/ssh/generate-ed25519-key-pair",
			TaskTimeout:  10 * time.Minute,
		},
		privatePath: privatePath,
		comment:     DefaultSSHComment,
	}

	return task
}

// NewGenerateEd25519KeyPairWithComment creates a new GenerateEd25519KeyPair task with a custom comment
func NewGenerateEd25519KeyPairWithComment(privatePath string, comment string) *GenerateEd25519KeyPair {
	task := NewGenerateEd25519KeyPair(privatePath)
	task.comment = comment

	return task
}

// Data returns the template data
func (t *GenerateEd25519KeyPair) Data() map[string]interface{} {
	return map[string]interface{}{
		"PrivatePath": t.privatePath,
		"Comment":     t.comment,
	}
}

// PrivatePath returns the private key path
func (t *GenerateEd25519KeyPair) PrivatePath() string {
	return t.privatePath
}

// Comment returns the SSH key comment
func (t *GenerateEd25519KeyPair) Comment() string {
	return t.comment
}

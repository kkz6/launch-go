package tasks

import (
	"time"

	"github.com/kkz6/launch-go/internal/taskrunner"
)

// GenerateRsaKeyPair generates an RSA SSH key pair
type GenerateRsaKeyPair struct {
	taskrunner.BaseTask
	privatePath string
	comment     string
}

// NewGenerateRsaKeyPair creates a new GenerateRsaKeyPair task
func NewGenerateRsaKeyPair(privatePath string) *GenerateRsaKeyPair {
	task := &GenerateRsaKeyPair{
		BaseTask: taskrunner.BaseTask{
			TemplateName: "server/ssh/generate-rsa-key-pair",
			TaskTimeout:  10 * time.Minute,
		},
		privatePath: privatePath,
		comment:     DefaultSSHComment,
	}

	return task
}

// NewGenerateRsaKeyPairWithComment creates a new GenerateRsaKeyPair task with a custom comment
func NewGenerateRsaKeyPairWithComment(privatePath string, comment string) *GenerateRsaKeyPair {
	task := NewGenerateRsaKeyPair(privatePath)
	task.comment = comment

	return task
}

// Data returns the template data
func (t *GenerateRsaKeyPair) Data() map[string]interface{} {
	return map[string]interface{}{
		"PrivatePath": t.privatePath,
		"Comment":     t.comment,
	}
}

// PrivatePath returns the private key path
func (t *GenerateRsaKeyPair) PrivatePath() string {
	return t.privatePath
}

// Comment returns the SSH key comment
func (t *GenerateRsaKeyPair) Comment() string {
	return t.comment
}

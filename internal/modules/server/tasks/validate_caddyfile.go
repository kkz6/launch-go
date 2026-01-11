package tasks

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/taskrunner"
)

// ValidateCaddyfile validates a Caddyfile configuration
type ValidateCaddyfile struct {
	taskrunner.BaseTask
	caddyfile string
	path      string
}

// NewValidateCaddyfile creates a new ValidateCaddyfile task
func NewValidateCaddyfile(caddyfile string) *ValidateCaddyfile {
	task := &ValidateCaddyfile{
		BaseTask: taskrunner.BaseTask{
			TaskName:     "validate-caddyfile",
			TemplateName: "server/validate-caddyfile",
			TaskTimeout:  2 * time.Minute,
		},
		caddyfile: caddyfile,
		path:      generateTempCaddyfilePath(),
	}

	return task
}

// Data returns the template data
func (t *ValidateCaddyfile) Data() map[string]interface{} {
	return map[string]interface{}{
		"Caddyfile": t.caddyfile,
		"Path":      t.path,
	}
}

// Caddyfile returns the Caddyfile content being validated
func (t *ValidateCaddyfile) Caddyfile() string {
	return t.caddyfile
}

// Path returns the temporary file path for validation
func (t *ValidateCaddyfile) Path() string {
	return t.path
}

// generateTempCaddyfilePath generates a unique temporary file path
func generateTempCaddyfilePath() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "/tmp/caddyfile-default.caddyfile"
	}

	return fmt.Sprintf("/tmp/caddyfile-%s.caddyfile", hex.EncodeToString(bytes))
}

package tasks

import (
	"time"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// PrettifyCaddyfile formats a Caddyfile using caddy fmt
type PrettifyCaddyfile struct {
	taskrunner.BaseTask
	path string
}

// NewPrettifyCaddyfile creates a new PrettifyCaddyfile task
func NewPrettifyCaddyfile(path string) *PrettifyCaddyfile {
	return &PrettifyCaddyfile{
		BaseTask: taskrunner.BaseTask{
			TemplateName: "site/prettify-caddyfile",
			TaskTimeout:  15 * time.Second,
		},
		path: path,
	}
}

// Data returns the template data
func (t *PrettifyCaddyfile) Data() map[string]interface{} {
	return map[string]interface{}{
		"Path": t.path,
	}
}

// Render returns the command to run (simple command, no template needed)
func (t *PrettifyCaddyfile) Render() string {
	return "caddy fmt " + t.path + " --overwrite"
}

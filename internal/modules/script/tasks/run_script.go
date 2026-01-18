package tasks

import (
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// RunScriptConfig holds configuration for running a script on a server
type RunScriptConfig struct {
	Content string // The interpolated script content
}

// RunScript creates a task to run a script on a server
func RunScript(cfg RunScriptConfig) taskrunner.Task {
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Run Script"),
		taskrunner.WithScript(cfg.Content),
		taskrunner.WithTimeoutSeconds(3600), // 1 hour max
	)
}

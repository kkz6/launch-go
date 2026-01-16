package tasks

import (
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// Task type constants for whoami
const (
	WhoamiTaskType = "server:whoami"
)

// Whoami creates a task to return the current user.
func Whoami() *taskrunner.BaseTask {
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Whoami"),
		taskrunner.WithScript("whoami"),
		taskrunner.WithTimeoutSeconds(15),
	)
}

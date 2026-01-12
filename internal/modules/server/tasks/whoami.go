package tasks

import (
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// WhoamiTask returns the current user
type WhoamiTask struct {
	*ServerTask
}

// NewWhoamiTask creates a new WhoamiTask
func NewWhoamiTask() *WhoamiTask {
	return &WhoamiTask{
		ServerTask: NewServerTaskWithName("Whoami", "whoami", 15),
	}
}

// Whoami creates a simple whoami task
func Whoami() *taskrunner.BaseTask {
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Whoami"),
		taskrunner.WithScript("whoami"),
		taskrunner.WithTimeout(15),
	)
}

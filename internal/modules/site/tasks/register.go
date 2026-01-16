package tasks

import (
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// RegisterTaskCallbacks registers all site task callback handlers.
// This should be called during application startup.
//
// Similar to job registration, this provides explicit registration
// of task callbacks that can handle completion events.
func RegisterTaskCallbacks() {
	// Deployment tasks - state struct implements NewTask()
	taskrunner.RegisterCallbackState[callbackData](DeploySiteTaskType)

	// Add more task callbacks here as needed:
	// taskrunner.RegisterCallbackState[rollbackState](RollbackTaskType)
}

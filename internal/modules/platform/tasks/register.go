package tasks

import (
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// RegisterTaskCallbacks registers all platform task callback handlers.
func RegisterTaskCallbacks() {
	// Register callback state for rename username task
	taskrunner.RegisterCallbackState[renameCallbackData](RenameUsernameTaskType)

	// Register the rename username task factory in the update task registry
	RegisterUpdateTask(RenameUsernameKey, RenameUsernameTask)
}

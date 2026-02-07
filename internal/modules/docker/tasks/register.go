package tasks

import "github.com/kkz6/launch-go/internal/pkg/taskrunner"

// RegisterTaskCallbacks registers all docker task callback handlers.
// This should be called during application startup.
func RegisterTaskCallbacks() {
	taskrunner.RegisterCallbackState[deployCallbackData](DeployDockerServiceTaskType)
}
